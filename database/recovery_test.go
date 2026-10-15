package database

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yongwei9527-art/s-ui-go/database/model"
)

func waitForRecovery(t *testing.T, gate *accessGate) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		gate.mu.Lock()
		busy := gate.busy
		gate.mu.Unlock()
		if busy {
			return
		}
		select {
		case <-deadline:
			t.Fatal("recovery did not enter drain")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestAccessGateDrainsAndRequiresSuccessfulRestart(t *testing.T) {
	var gate accessGate
	release, ok := gate.tryAcquire()
	if !ok {
		t.Fatal("initial lease rejected")
	}
	ready := make(chan error, 1)
	go func() { ready <- gate.beginRecovery(context.Background()) }()
	waitForRecovery(t, &gate)
	if _, ok := gate.tryAcquire(); ok {
		t.Fatal("new work admitted during drain")
	}
	select {
	case <-ready:
		t.Fatal("recovery passed an active operation")
	default:
	}
	release()
	release() // Entry releases are idempotent.
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	gate.finishRecovery(true)
	if _, ok := gate.tryAcquire(); ok {
		t.Fatal("post-import requests admitted before startup")
	}
	finish, err := gate.beginStart()
	if err != nil {
		t.Fatal(err)
	}
	finish(false)
	if _, ok := gate.tryAcquire(); ok {
		t.Fatal("failed startup reopened access")
	}
	finish, err = gate.beginStart()
	if err != nil {
		t.Fatal(err)
	}
	finish(true)
	release, ok = gate.tryAcquire()
	if !ok {
		t.Fatal("successful startup did not reopen access")
	}
	release()
}

func TestAccessGateCanceledDrainLeavesExistingWorkUsable(t *testing.T) {
	var gate accessGate
	release, _ := gate.tryAcquire()
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan error, 1)
	go func() { ready <- gate.beginRecovery(ctx) }()
	waitForRecovery(t, &gate)
	cancel()
	if err := <-ready; !errors.Is(err, context.Canceled) {
		t.Fatalf("drain error: %v", err)
	}
	next, ok := gate.tryAcquire()
	if !ok {
		t.Fatal("canceled recovery kept requests blocked")
	}
	next()
}

func TestApplicationStartCompletionIsIdempotent(t *testing.T) {
	var gate accessGate
	finish, err := gate.beginStart()
	if err != nil {
		t.Fatal(err)
	}
	finish(true)
	finish(false)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := gate.beginRecovery(ctx); err != nil {
		t.Fatalf("startup completion leaked an invalid active count: %v", err)
	}
	gate.finishRecovery(false)
}

func TestCoordinatedImportPanicNeverDispatchesRestart(t *testing.T) {
	t.Cleanup(func() { applicationDBAccess.finishRecovery(false) })
	_, data := importFixture(t)
	var decision <-chan bool
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		_ = coordinatedImport(context.Background(), backupUpload{bytes.NewReader(data)}, RecoveryHooks{
			Quiesce: func() error { return nil }, ResumeAfterAbort: func() error { return nil },
		}, func(done <-chan bool) error { decision = done; panic("injected scheduler panic") })
	}()
	if !panicked || decision == nil {
		t.Fatal("did not reach injected panic")
	}
	if <-decision {
		t.Fatal("panic was mistaken for a successful import and dispatched restart")
	}
	if !RecoveryBlocked() {
		t.Fatal("panic reopened unvalidated runtime access")
	}
}

func TestRecoveryResumePanicLeavesExplicitStartupPossible(t *testing.T) {
	t.Cleanup(func() { applicationDBAccess.finishRecovery(false) })
	_, data := importFixture(t)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("expected resume panic")
			}
		}()
		_ = coordinatedImport(context.Background(), backupUpload{bytes.NewReader(data)}, RecoveryHooks{
			Quiesce:          func() error { return errors.New("partial stop failed") },
			ResumeAfterAbort: func() error { panic("injected resume panic") },
		}, func(<-chan bool) error { t.Fatal("unexpected schedule"); return nil })
	}()
	if !RecoveryBlocked() {
		t.Fatal("partial runtime resume panic reopened requests")
	}
	finish, err := BeginApplicationStart()
	if err != nil {
		t.Fatalf("recovery gate remained permanently busy after panic: %v", err)
	}
	finish(false)
}

func TestCoordinatedImportDrainsFinalTransactionAndWaitsForRestart(t *testing.T) {
	t.Cleanup(func() { applicationDBAccess.finishRecovery(false) })
	source, data := importFixture(t)
	release, ok := TryAcquireAccess()
	if !ok {
		t.Fatal("lease rejected")
	}
	defer release()
	tx := source.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.Create(&model.Changes{Id: 70, Key: "last-active-request", Obj: []byte(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	quiesced := make(chan struct{})
	var decision <-chan bool
	result := make(chan error, 1)
	go func() {
		result <- coordinatedImport(context.Background(), backupUpload{bytes.NewReader(data)}, RecoveryHooks{
			Quiesce: func() error { close(quiesced); return nil }, ResumeAfterAbort: func() error { return nil },
		}, func(done <-chan bool) error { decision = done; return nil })
	}()
	waitForRecovery(t, &applicationDBAccess)
	select {
	case <-quiesced:
		t.Fatal("quiesced before final transaction completed")
	default:
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	release()
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	select {
	case <-quiesced:
	default:
		t.Fatal("runtime not quiesced")
	}
	assertImportIdentity(t, "uploaded")
	assertImportMarker(t, db, 71)
	if !<-decision {
		t.Fatal("successful import canceled its scheduled restart")
	}
	if !RecoveryBlocked() {
		t.Fatal("requests resumed with old runtime/tokens")
	}
	finish, err := BeginApplicationStart()
	if err != nil {
		t.Fatal(err)
	}
	finish(true)
	if RecoveryBlocked() {
		t.Fatal("startup did not reopen access")
	}
}

func TestCoordinatedImportAbortBoundaries(t *testing.T) {
	for _, failure := range []string{"invalid-upload", "quiesce", "resume", "schedule", "marker"} {
		t.Run(failure, func(t *testing.T) {
			t.Cleanup(func() { applicationDBAccess.finishRecovery(false) })
			source, data := importFixture(t)
			if failure == "invalid-upload" {
				data = []byte("not sqlite")
			}
			quiesces, resumes := 0, 0
			var decision <-chan bool
			err := coordinatedImport(context.Background(), backupUpload{bytes.NewReader(data)}, RecoveryHooks{
				Quiesce: func() error {
					quiesces++
					if failure == "quiesce" || failure == "resume" {
						return errors.New("stop failed")
					}
					return nil
				},
				ResumeAfterAbort: func() error {
					resumes++
					if failure == "resume" {
						return errors.New("resume failed")
					}
					return nil
				},
			}, func(done <-chan bool) error {
				decision = done
				if failure == "schedule" {
					return errors.New("schedule failed")
				}
				if failure == "marker" {
					return db.Exec("CREATE TRIGGER refuse_import BEFORE INSERT ON changes WHEN NEW.action = 'import' BEGIN SELECT RAISE(ABORT, 'no marker'); END").Error
				}
				return nil
			})
			if err == nil {
				t.Fatal("failure reported success")
			}
			if decision != nil && <-decision {
				t.Fatal("failed import dispatched a signal")
			}
			switch failure {
			case "invalid-upload":
				if quiesces != 0 || resumes != 0 || RecoveryBlocked() {
					t.Fatal("invalid upload stopped runtime or kept access paused")
				}
				if db != source {
					t.Fatal("invalid upload closed old DB")
				}
			case "quiesce":
				if quiesces != 1 || resumes != 1 || RecoveryBlocked() || db != source {
					t.Fatal("pre-swap abort did not restore unchanged runtime")
				}
			case "resume", "schedule", "marker":
				if !RecoveryBlocked() || !strings.Contains(err.Error(), "access remains paused") {
					t.Fatalf("unsafe failure reopened access: %v", err)
				}
			}
		})
	}
}

func TestCoordinatedCanceledDrainDoesNotCloseOrQuiesce(t *testing.T) {
	t.Cleanup(func() { applicationDBAccess.finishRecovery(false) })
	source, data := importFixture(t)
	release, _ := TryAcquireAccess()
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := coordinatedImport(ctx, backupUpload{bytes.NewReader(data)}, RecoveryHooks{
		Quiesce:          func() error { t.Fatal("quiesce after cancellation"); return nil },
		ResumeAfterAbort: func() error { t.Fatal("resume unchanged runtime"); return nil },
	}, func(<-chan bool) error { t.Fatal("schedule after cancellation"); return nil })
	if !errors.Is(err, context.Canceled) || db != source || RecoveryBlocked() {
		t.Fatalf("canceled import changed running state: %v", err)
	}
	assertImportIdentity(t, "old")
}

func TestImportSessionKeyRotationIsAtomicWithSuccessMarker(t *testing.T) {
	for _, failMarker := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "marker-failure"}[failMarker], func(t *testing.T) {
			source, data := importFixture(t)
			if err := source.Create(&model.Setting{Key: "secret", Value: "old-session-secret"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := source.Create(&model.Setting{Key: "secret", Value: "old-duplicate-secret"}).Error; err != nil {
				t.Fatal(err)
			}
			candidate := readBackup(t, data)
			if err := candidate.Create(&model.Setting{Key: "secret", Value: "old-session-secret"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := candidate.Create(&model.Setting{Key: "secret", Value: "old-duplicate-secret"}).Error; err != nil {
				t.Fatal(err)
			}
			data = closedBackupBytes(t, candidate)
			err := importDB(backupUpload{bytes.NewReader(data)}, func() error {
				if failMarker {
					return db.Exec("CREATE TRIGGER refuse_session_import BEFORE INSERT ON changes WHEN NEW.action = 'import' BEGIN SELECT RAISE(ABORT, 'no marker'); END").Error
				}
				return nil
			})
			if (err != nil) != failMarker {
				t.Fatalf("import outcome: %v", err)
			}
			var settings []model.Setting
			if err := db.Where("key = ?", "secret").Order("id").Find(&settings).Error; err != nil || len(settings) != 2 {
				t.Fatalf("session rows missing: %v", err)
			}
			if failMarker {
				if settings[0].Value != "old-session-secret" || settings[1].Value != "old-duplicate-secret" {
					t.Fatal("marker failure did not roll back key rotation")
				}
			} else if len(settings[0].Value) != 32 || settings[0].Value == "old-session-secret" || settings[0].Value != settings[1].Value {
				t.Fatal("successful import did not invalidate every old session key")
			}
		})
	}
}
