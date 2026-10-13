package service

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/op/go-logging"
	"github.com/yongwei9527-art/s-ui-go/core"
	"github.com/yongwei9527-art/s-ui-go/logger"
)

func waitRestartWorkerIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		configRestartMu.Lock()
		running := configRestartWorkerRunning
		configRestartMu.Unlock()
		if !running {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("restart worker did not exit")
		}
		time.Sleep(time.Millisecond)
	}
}

// Channels pin the worker in retry backoff until StopCore has returned. No
// scheduler timing is needed to make the stale retry cross the shutdown boundary.
func TestStopCoreCancelsRestartWorkerInBackoff(t *testing.T) {
	isolateRestartQueue(t)
	logger.InitLogger(logging.ERROR)
	oldCore := corePtr
	corePtr = nil
	enteredWait := make(chan struct{})
	releaseWait := make(chan struct{})
	var releaseOnce sync.Once
	var attempts int
	s := &ConfigService{
		restartConfigFn: func(json.RawMessage) error {
			attempts++
			if attempts == 1 {
				return errors.New("force retry")
			}
			return nil
		},
		restartConfigRetryWait: func(time.Duration) {
			close(enteredWait)
			<-releaseWait
		},
	}
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(releaseWait) })
		waitRestartWorkerIdle(t)
		corePtr = oldCore
	})
	s.enqueueConfigRestart(json.RawMessage(`{"name":"pending"}`))
	select {
	case <-enteredWait:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not enter retry backoff")
	}
	if err := s.StopCore(); err != nil {
		t.Fatal(err)
	}
	releaseOnce.Do(func() { close(releaseWait) })
	waitRestartWorkerIdle(t)
	if attempts != 1 {
		t.Fatalf("restart attempts = %d, want 1: worker restarted after StopCore returned", attempts)
	}
}

func isolateRestartQueue(t *testing.T) {
	t.Helper()
	waitRestartWorkerIdle(t)
	configRestartMu.Lock()
	oldPending := append(json.RawMessage(nil), pendingConfigRestart...)
	oldVersion, oldSuspended := pendingConfigRestartVersion, configRestartSuspended
	pendingConfigRestart = nil
	configRestartSuspended = false
	configRestartMu.Unlock()
	t.Cleanup(func() {
		waitRestartWorkerIdle(t)
		configRestartMu.Lock()
		pendingConfigRestart, pendingConfigRestartVersion, configRestartSuspended = oldPending, oldVersion, oldSuspended
		configRestartMu.Unlock()
		drainConfigRestartRetryNotification()
	})
}

func TestRestartQueueResumesLatestAfterExplicitStart(t *testing.T) {
	for _, backoff := range []bool{false, true} {
		name := "stopped"
		if backoff {
			name = "canceled-worker"
		}
		t.Run(name, func(t *testing.T) {
			isolateRestartQueue(t)
			logger.InitLogger(logging.ERROR)
			oldCore := corePtr
			instance := core.NewCore()
			corePtr = instance
			valid := []byte(`{"log":{"disabled":true},"outbounds":[{"type":"direct","tag":"direct"}]}`)
			if err := instance.Start(valid); err != nil {
				t.Fatal(err)
			}
			release, entered := make(chan struct{}), make(chan struct{})
			var once sync.Once
			applied := make(chan string, 2)
			s := &ConfigService{
				restartConfigFn: func(config json.RawMessage) error {
					if string(config) == `{"name":"A"}` {
						return errors.New("retry A")
					}
					applied <- string(config)
					return nil
				},
				restartConfigRetryWait: func(time.Duration) { close(entered); <-release },
			}
			t.Cleanup(func() {
				once.Do(func() { close(release) })
				waitRestartWorkerIdle(t)
				_ = instance.Stop()
				corePtr = oldCore
			})
			if backoff {
				s.enqueueConfigRestart(json.RawMessage(`{"name":"A"}`))
				select {
				case <-entered:
				case <-time.After(2 * time.Second):
					t.Fatal("retry did not start")
				}
			}
			if err := s.StopCore(); err != nil {
				t.Fatal(err)
			}
			s.enqueueConfigRestart(json.RawMessage(`{"name":"B"}`))
			select {
			case got := <-applied:
				t.Fatalf("queued start crossed stopped state: %s", got)
			case <-time.After(25 * time.Millisecond):
			}
			// A running isolated core exercises StartCore's successful lifecycle branch
			// without loading unrelated seeded default listeners from a real database.
			if err := instance.Start(valid); err != nil {
				t.Fatal(err)
			}
			if err := s.StartCore(); err != nil {
				t.Fatal(err)
			}
			once.Do(func() { close(release) })
			select {
			case got := <-applied:
				if got != `{"name":"B"}` {
					t.Fatalf("applied stale config: %s", got)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("resume lost latest configuration")
			}
			waitRestartWorkerIdle(t)
		})
	}
}
