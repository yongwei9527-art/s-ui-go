package service

import (
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/op/go-logging"
	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	projectlogger "github.com/yongwei9527-art/s-ui-go/logger"
)

func TestConfigRestartQueueRetriesLatestPendingConfig(t *testing.T) {
	projectlogger.InitLogger(logging.ERROR)

	oldCore := corePtr
	corePtr = nil
	t.Cleanup(func() {
		corePtr = oldCore
	})

	configRestartMu.Lock()
	if configRestartWorkerRunning {
		configRestartMu.Unlock()
		t.Fatal("config restart worker was already running")
	}
	pendingConfigRestart = nil
	configRestartMu.Unlock()
	drainConfigRestartRetryNotification()

	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var releaseFirstOnce sync.Once
	t.Cleanup(func() {
		releaseFirstOnce.Do(func() { close(releaseFirst) })
	})

	applied := make(chan string, 3)
	var configAttemptsMu sync.Mutex
	configAttempts := make(map[string]int)
	var backoffsMu sync.Mutex
	var backoffs []time.Duration
	service := &ConfigService{
		restartConfigFn: func(raw json.RawMessage) error {
			config := string(raw)
			applied <- config

			configAttemptsMu.Lock()
			configAttempts[config]++
			attempt := configAttempts[config]
			configAttemptsMu.Unlock()

			switch config {
			case `{"name":"A"}`:
				close(firstStarted)
				<-releaseFirst
			case `{"name":"C"}`:
				if attempt == 1 {
					return errors.New("first C restart fails")
				}
			}
			return nil
		},
		restartConfigRetryWait: func(backoff time.Duration) {
			backoffsMu.Lock()
			backoffs = append(backoffs, backoff)
			backoffsMu.Unlock()
		},
	}

	service.enqueueConfigRestart(json.RawMessage(`{"name":"A"}`))
	select {
	case <-firstStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first configuration restart did not start")
	}

	manualRestart := make(chan error, 1)
	go func() {
		manualRestart <- service.RestartCore()
	}()
	select {
	case err := <-manualRestart:
		t.Fatalf("manual restart interleaved with config restart: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	service.enqueueConfigRestart(json.RawMessage(`{"name":"B"}`))
	service.enqueueConfigRestart(json.RawMessage(`{"name":"C"}`))
	releaseFirstOnce.Do(func() { close(releaseFirst) })

	want := []string{`{"name":"A"}`, `{"name":"C"}`, `{"name":"C"}`}
	for _, expected := range want {
		select {
		case actual := <-applied:
			if actual != expected {
				t.Fatalf("applied config = %s, want %s", actual, expected)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for config %s", expected)
		}
	}

	select {
	case err := <-manualRestart:
		if err == nil {
			t.Fatal("manual restart unexpectedly succeeded without an initialized core")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("manual restart did not resume after config restart completed")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		configRestartMu.Lock()
		running := configRestartWorkerRunning
		pending := append(json.RawMessage(nil), pendingConfigRestart...)
		configRestartMu.Unlock()
		if !running {
			if len(pending) != 0 {
				t.Fatalf("pending config = %s after successful retry, want empty", pending)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("config restart worker did not drain")
		}
		time.Sleep(time.Millisecond)
	}

	configAttemptsMu.Lock()
	cAttempts := configAttempts[`{"name":"C"}`]
	configAttemptsMu.Unlock()
	if cAttempts != 2 {
		t.Fatalf("C restart attempts = %d, want 2", cAttempts)
	}
	backoffsMu.Lock()
	gotBackoffs := append([]time.Duration(nil), backoffs...)
	backoffsMu.Unlock()
	if len(gotBackoffs) != 1 || gotBackoffs[0] != configRestartInitialBackoff {
		t.Fatalf("retry backoffs = %v, want [%s]", gotBackoffs, configRestartInitialBackoff)
	}
}
func TestConfigRestartQueueNewSubmissionWakesRetry(t *testing.T) {
	projectlogger.InitLogger(logging.ERROR)

	oldCore := corePtr
	corePtr = nil
	t.Cleanup(func() {
		corePtr = oldCore
	})

	configRestartMu.Lock()
	if configRestartWorkerRunning {
		configRestartMu.Unlock()
		t.Fatal("config restart worker was already running")
	}
	pendingConfigRestart = nil
	configRestartMu.Unlock()
	drainConfigRestartRetryNotification()

	oldAttempted := make(chan struct{})
	newApplied := make(chan struct{})
	var oldAttemptedOnce sync.Once
	var newAppliedOnce sync.Once
	service := &ConfigService{
		restartConfigFn: func(raw json.RawMessage) error {
			switch string(raw) {
			case `{"name":"old"}`:
				oldAttemptedOnce.Do(func() { close(oldAttempted) })
				return errors.New("old config fails")
			case `{"name":"new"}`:
				newAppliedOnce.Do(func() { close(newApplied) })
				return nil
			default:
				return errors.New("unexpected config: " + string(raw))
			}
		},
	}

	service.enqueueConfigRestart(json.RawMessage(`{"name":"old"}`))
	select {
	case <-oldAttempted:
	case <-time.After(2 * time.Second):
		t.Fatal("old config did not fail")
	}

	// The running worker is either about to wait or already waiting for its backoff.
	// Enqueuing a newer config must replace old and wake that wait immediately.
	service.enqueueConfigRestart(json.RawMessage(`{"name":"new"}`))
	select {
	case <-newApplied:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("new config did not wake and replace the failed retry")
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		configRestartMu.Lock()
		running := configRestartWorkerRunning
		pending := append(json.RawMessage(nil), pendingConfigRestart...)
		configRestartMu.Unlock()
		if !running {
			if len(pending) != 0 {
				t.Fatalf("pending config = %s after replacement succeeds, want empty", pending)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("replacement config worker did not drain")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestPublishLastUpdateIsSynchronizedAndMonotonic(t *testing.T) {
	lastUpdateMu.Lock()
	original := LastUpdate
	originalInitialized := lastUpdateInitialized
	LastUpdate = 0
	lastUpdateInitialized = false
	lastUpdateMu.Unlock()
	t.Cleanup(func() {
		lastUpdateMu.Lock()
		LastUpdate = original
		lastUpdateInitialized = originalInitialized
		lastUpdateMu.Unlock()
	})

	const writers = 64
	start := make(chan struct{})
	var wg sync.WaitGroup
	for update := uint64(1); update <= writers; update++ {
		wg.Add(1)
		go func(update uint64) {
			defer wg.Done()
			<-start
			publishLastUpdate(update)
			_ = loadLastUpdate()
		}(update)
	}
	close(start)
	wg.Wait()

	if got := loadLastUpdate(); got != writers {
		t.Fatalf("LastUpdate = %d, want %d", got, writers)
	}
}

func TestConfigSavePublishesLastUpdateOnlyAfterSuccessfulCommit(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", "test-only-initial-password")
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("initialize database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := database.GetDB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	lastUpdateMu.Lock()
	original := LastUpdate
	originalInitialized := lastUpdateInitialized
	LastUpdate = 0
	lastUpdateInitialized = false
	lastUpdateMu.Unlock()
	t.Cleanup(func() {
		lastUpdateMu.Lock()
		LastUpdate = original
		lastUpdateInitialized = originalInitialized
		lastUpdateMu.Unlock()
	})

	service := &ConfigService{}
	if _, err := service.Save("settings", "edit", json.RawMessage(`{"webPath":"/after-commit/"}`), "", "test", ""); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if got := loadLastUpdate(); got == 0 {
		t.Fatal("LastUpdate was not published after a successful commit")
	}

	var changes int64
	if err := database.GetDB().Model(&model.Changes{}).Count(&changes).Error; err != nil {
		t.Fatalf("count changes: %v", err)
	}
	if changes != 1 {
		t.Fatalf("committed change count = %d, want 1", changes)
	}

	lastUpdateMu.Lock()
	LastUpdate = 0
	lastUpdateMu.Unlock()
	if _, err := service.Save("settings", "edit", json.RawMessage(`{`), "", "test", ""); err == nil {
		t.Fatal("Save() accepted invalid settings JSON")
	}
	if got := loadLastUpdate(); got != 0 {
		t.Fatalf("LastUpdate = %d after failed save, want 0", got)
	}
}

func TestCheckChangesUsesPersistentRevisionForSameSecondUpdates(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", "test-only-initial-password")
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("initialize database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := database.GetDB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	lastUpdateMu.Lock()
	original := LastUpdate
	originalInitialized := lastUpdateInitialized
	LastUpdate = 0
	lastUpdateInitialized = false
	lastUpdateMu.Unlock()
	t.Cleanup(func() {
		lastUpdateMu.Lock()
		LastUpdate = original
		lastUpdateInitialized = originalInitialized
		lastUpdateMu.Unlock()
	})

	service := &ConfigService{}
	if _, err := service.Save("settings", "edit", json.RawMessage(`{"webPath":"/same-second-one/"}`), "", "test", ""); err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	firstRevision := service.CurrentChangeRevision()
	if firstRevision == 0 {
		t.Fatal("first committed change did not publish a revision")
	}

	if _, err := service.Save("settings", "edit", json.RawMessage(`{"webPath":"/same-second-two/"}`), "", "test", ""); err != nil {
		t.Fatalf("second Save() error = %v", err)
	}
	secondRevision := service.CurrentChangeRevision()
	if secondRevision <= firstRevision {
		t.Fatalf("second revision = %d, want > %d", secondRevision, firstRevision)
	}

	changed, err := service.CheckChanges(strconv.FormatUint(firstRevision, 10))
	if err != nil {
		t.Fatalf("CheckChanges() error = %v", err)
	}
	if !changed {
		t.Fatal("same-second second change was not visible to the first revision")
	}

	legacyChanged, err := service.CheckChanges(strconv.FormatInt(time.Now().Unix(), 10))
	if err != nil {
		t.Fatalf("legacy CheckChanges() error = %v", err)
	}
	if !legacyChanged {
		t.Fatal("legacy timestamp client was not forced to refresh into revision mode")
	}
}
