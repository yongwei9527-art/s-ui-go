package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/op/go-logging"
	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/logger"
)

func TestRecoveryDiscardsOldQueuedConfigAndRollbackSnapshot(t *testing.T) {
	isolateRestartQueue(t)
	coreOperationMu.Lock()
	oldConfig := lastSuccessfulConfig
	lastSuccessfulConfig = json.RawMessage(`{"old":"rollback"}`)
	coreOperationMu.Unlock()
	startCoreMu.Lock()
	oldFailTime, oldFailCount := lastStartFailTime, startFailCount
	lastStartFailTime, startFailCount = time.Now(), 4
	startCoreMu.Unlock()
	t.Cleanup(func() {
		coreOperationMu.Lock()
		lastSuccessfulConfig = oldConfig
		coreOperationMu.Unlock()
		startCoreMu.Lock()
		lastStartFailTime, startFailCount = oldFailTime, oldFailCount
		startCoreMu.Unlock()
	})
	configRestartMu.Lock()
	configRestartSuspended = true
	pendingConfigRestart = json.RawMessage(`{"old":"queued"}`)
	version := pendingConfigRestartVersion
	configRestartMu.Unlock()
	(&ConfigService{}).DiscardQueuedConfigForRecovery()
	configRestartMu.Lock()
	if len(pendingConfigRestart) != 0 || pendingConfigRestartVersion <= version || !configRestartSuspended {
		t.Error("old queued settings survived recovery")
	}
	configRestartMu.Unlock()
	coreOperationMu.Lock()
	if len(lastSuccessfulConfig) != 0 {
		t.Error("old rollback snapshot survived recovery")
	}
	coreOperationMu.Unlock()
	startCoreMu.Lock()
	if !lastStartFailTime.IsZero() || startFailCount != 0 {
		t.Error("new DB inherited old startup cooldown")
	}
	startCoreMu.Unlock()
}

func recoveryXrayFixture(t *testing.T) (*XrayXHTTPService, model.Client, string) {
	t.Helper()
	logger.InitLogger(logging.ERROR)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", "test-only-password")
	t.Setenv("SUI_AUTOCONFIGURE_XRAY_REALITY", "0")
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatal(err)
	}
	sidecar := &XrayXHTTPService{}
	t.Cleanup(func() {
		_ = sidecar.Stop()
		if sqlDB, err := database.GetDB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	var client model.Client
	if err := database.GetDB().Where("name = ?", "default-user").First(&client).Error; err != nil {
		t.Fatal(err)
	}
	inbound := model.XrayXHTTPInbound{
		Enable: true, Tag: "recovery-auth-test", Listen: "127.0.0.1", Port: uint16(reserveAndReleaseTCPPort(t)),
		ServerName: "www.example.com", Dest: "127.0.0.1:1", Transport: "tcp",
		PrivateKey: testXrayRealityPrivateKey, PublicKey: testXrayRealityPublicKey,
		ShortID: "0123456789abcdef", Fingerprint: "chrome",
	}
	if err := sidecar.Save(XrayXHTTPSaveRequest{Inbound: inbound, ClientIDs: []uint{client.Id}}); err != nil {
		t.Fatal(err)
	}
	var assignment model.XrayXHTTPClient
	if err := database.GetDB().Where("tag = ?", inbound.Tag).First(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Where("client_id = ? AND inbound_id = ?", client.Id, inbound.Id).First(&assignment).Error; err != nil {
		t.Fatal(err)
	}
	assertRuntimeAuthorization(t, assignment.UUID, true)
	return sidecar, client, assignment.UUID
}

func assertRuntimeAuthorization(t *testing.T, uuid string, want bool) {
	t.Helper()
	xrayXHTTPProcess.mu.Lock()
	defer xrayXHTTPProcess.mu.Unlock()
	if xrayXHTTPProcess.server == nil && want {
		t.Fatal("expected an active isolated Xray runtime")
	}
	if got := bytes.Contains(xrayXHTTPProcess.active, []byte(uuid)); got != want {
		t.Fatalf("runtime authorization present=%v want=%v", got, want)
	}
}

func TestXrayClientRevocationCompletesWithinParentLeaseDuringAbortedRecovery(t *testing.T) {
	for _, abort := range []string{"cancel", "invalid-upload"} {
		t.Run(abort, func(t *testing.T) {
			sidecar, client, uuid := recoveryXrayFixture(t)
			release, ok := database.TryAcquireAccess() // cron/API parent operation
			if !ok {
				t.Fatal("parent operation rejected")
			}
			defer release()
			if err := database.GetDB().Model(&model.Client{}).Where("id = ?", client.Id).Update("enable", false).Error; err != nil {
				t.Fatal(err)
			}
			restore := database.RegisterRecoveryHooks(database.RecoveryHooks{
				Quiesce:          func() error { t.Error("invalid/canceled upload must not quiesce"); return nil },
				ResumeAfterAbort: func() error { t.Error("no runtime was quiesced"); return nil },
			})
			defer restore()
			file, err := os.Create(filepath.Join(t.TempDir(), "invalid.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			ctx, cancel := context.WithCancel(context.Background())
			completed := make(chan error, 1)
			go func() { completed <- database.ImportDBContext(ctx, file) }()
			defer func() { cancel(); release(); <-completed }()
			deadline := time.After(2 * time.Second)
			for !database.RecoveryBlocked() {
				select {
				case <-deadline:
					t.Fatal("import did not start draining")
				case <-time.After(time.Millisecond):
				}
			}
			// This synchronous child must inherit the existing lease: recovery
			// is waiting for the parent to finish applying the committed change.
			if err := sidecar.ReloadAfterClientChange(); err != nil {
				t.Fatal(err)
			}
			assertRuntimeAuthorization(t, uuid, false)
			if abort == "cancel" {
				cancel()
			} else {
				release() // validation fails after the parent operation is complete
			}
			// Consume here and replace the channel for unconditional cleanup.
			err = <-completed
			completed <- err
			if err == nil || (abort == "cancel" && !errors.Is(err, context.Canceled)) {
				t.Fatalf("unexpected import result: %v", err)
			}
			if database.RecoveryBlocked() {
				t.Fatal("aborted import kept access paused")
			}
			assertRuntimeAuthorization(t, uuid, false)
		})
	}
}

func TestClientSaveWaitsForXrayRefreshBeforeReturning(t *testing.T) {
	_, client, uuid := recoveryXrayFixture(t)
	oldCore := corePtr
	corePtr = nil
	t.Cleanup(func() { corePtr = oldCore })
	release, ok := database.TryAcquireAccess()
	if !ok {
		t.Fatal("parent request rejected")
	}
	defer release()
	client.Enable = false
	raw, err := json.Marshal(client)
	if err != nil {
		t.Fatal(err)
	}
	xrayXHTTPProcess.mu.Lock()
	var once sync.Once
	unlock := func() { once.Do(xrayXHTTPProcess.mu.Unlock) }
	defer unlock()
	completed := make(chan error, 1)
	go func() {
		_, err := (&ConfigService{}).Save("clients", "edit", raw, "", "test", "127.0.0.1")
		completed <- err
	}()
	deadline := time.After(3 * time.Second)
	for {
		var saved model.Client
		if err := database.GetDB().First(&saved, client.Id).Error; err != nil {
			t.Fatal(err)
		}
		if !saved.Enable {
			break
		}
		select {
		case <-deadline:
			t.Fatal("save did not commit")
		case <-time.After(time.Millisecond):
		}
	}
	select {
	case err := <-completed:
		unlock()
		t.Fatalf("save returned before the authorization refresh completed: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	unlock()
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("save did not finish after the runtime lock was released")
	}
	assertRuntimeAuthorization(t, uuid, false)
}
