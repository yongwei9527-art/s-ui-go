package cronjob

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yongwei9527-art/s-ui-go/database"
)

func TestDatabaseJobSkipsRecoveryTickWithoutWaitingForStop(t *testing.T) {
	restore := database.RegisterRecoveryHooks(database.RecoveryHooks{Quiesce: func() error { t.Error("unexpected quiesce"); return nil }, ResumeAfterAbort: func() error { return nil }})
	defer restore()
	file, err := os.Create(filepath.Join(t.TempDir(), "unused.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	release, ok := database.TryAcquireAccess()
	if !ok {
		t.Fatal("initial operation rejected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- database.ImportDBContext(ctx, file) }()
	defer func() {
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Errorf("drain error: %v", err)
		}
		release()
	}()
	deadline := time.After(2 * time.Second)
	for !database.RecoveryBlocked() {
		select {
		case <-deadline:
			t.Fatal("recovery did not start")
		case <-time.After(time.Millisecond):
		}
	}
	runs := 0
	job := databaseJob(cron.FuncJob(func() { runs++ }))
	job.Run()
	if runs != 0 {
		t.Fatal("cron tick crossed recovery gate")
	}
}

func TestDatabaseJobReleasesLeaseAfterPanic(t *testing.T) {
	job := databaseJob(cron.FuncJob(func() { panic("test panic") }))
	func() { defer func() { _ = recover() }(); job.Run() }()
	restore := database.RegisterRecoveryHooks(database.RecoveryHooks{Quiesce: func() error { return nil }, ResumeAfterAbort: func() error { return nil }})
	defer restore()
	file, err := os.Create(filepath.Join(t.TempDir(), "invalid.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := database.ImportDBContext(ctx, file); err == nil || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, database.ErrRecoveryInProgress) {
		t.Fatalf("panic leaked a job lease: %v", err)
	}
}
