package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yongwei9527-art/s-ui-go/api"
	"github.com/yongwei9527-art/s-ui-go/database"
)

func TestRecoveryQuiesceDoesNotShutdownItsOwnHTTPHandler(t *testing.T) {
	a, events := newLifecycleTestApp(func(string) error { return nil })
	if err := a.quiesceForDatabaseRecovery(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"stop cron", "stop xray", "stop core"}; !reflect.DeepEqual(*events, want) {
		t.Fatalf("events=%v want=%v", *events, want)
	}
	if err := a.resumeAfterRecoveryAbort(); err != nil {
		t.Fatal(err)
	}
	want := []string{"stop cron", "stop xray", "stop core", "start core", "start xray", "start cron"}
	if !reflect.DeepEqual(*events, want) {
		t.Fatalf("events=%v want=%v", *events, want)
	}
}

func TestReadinessWaitsForCompleteColdApplicationStartup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUI_REQUIRE_PROXY_CORES", "1")
	for _, component := range []string{"sub", "core", "xray"} {
		for _, fail := range []bool{false, true} {
			name := component + "-success"
			if fail {
				name = component + "-failure"
			}
			t.Run(name, func(t *testing.T) {
				// Reset the test process to the open, zero-gate cold-start state.
				reset := func() {
					finish, err := database.BeginApplicationStart()
					if err != nil {
						t.Fatal(err)
					}
					finish(true)
				}
				reset()
				t.Cleanup(reset)
				entered, resume, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(resume) }) }
				injected := errors.New("injected late startup failure")
				a, _ := newLifecycleTestApp(func(event string) error {
					if event == "start "+component {
						close(entered)
						<-resume
						if fail {
							return injected
						}
					}
					return nil
				})
				var startErr error
				go func() { defer close(done); startErr = a.Start() }()
				t.Cleanup(func() { unblock(); <-done })
				select {
				case <-entered:
				case <-time.After(2 * time.Second):
					t.Fatal("startup did not reach blocked component")
				}
				router := gin.New()
				api.NewAPIHandler(router.Group("/api"), nil)
				probe := func() int {
					response := httptest.NewRecorder()
					router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/recovery-ready", nil))
					return response.Code
				}
				if status := probe(); status != http.StatusServiceUnavailable {
					t.Fatalf("readiness passed before %s finished: %d", component, status)
				}
				unblock()
				<-done
				if fail {
					if !errors.Is(startErr, injected) || probe() != http.StatusServiceUnavailable {
						t.Fatalf("failed startup became ready: %v", startErr)
					}
				} else if startErr != nil || probe() != http.StatusNoContent {
					t.Fatalf("successful startup did not become ready: %v", startErr)
				}
			})
		}
	}
}

func TestRecoveryQuiescePropagatesEveryCoreStopFailure(t *testing.T) {
	xrayErr, coreErr := errors.New("xray stop failed"), errors.New("core stop failed")
	a, _ := newLifecycleTestApp(func(name string) error {
		if name == "stop xray" {
			return xrayErr
		}
		if name == "stop core" {
			return coreErr
		}
		return nil
	})
	err := a.quiesceForDatabaseRecovery()
	if !errors.Is(err, xrayErr) || !errors.Is(err, coreErr) {
		t.Fatalf("stop errors lost: %v", err)
	}
}
