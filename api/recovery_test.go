package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yongwei9527-art/s-ui-go/database"
)

func TestRecoveryReadinessIsPublicUncachedAndNotAnAuthBypass(t *testing.T) {
	router := newAPIHandlerTestRouter()
	ready := httptest.NewRecorder()
	router.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/api/recovery-ready", nil))
	if ready.Code != http.StatusNoContent || ready.Body.Len() != 0 || ready.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("readiness = %d %q %#v", ready.Code, ready.Body.String(), ready.Header())
	}
	for _, request := range []struct{ method, path string }{
		{http.MethodPost, "/api/recovery-ready"},
		{http.MethodGet, "/api/load"},
		{http.MethodGet, "/api/not-recovery-ready"},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(request.method, request.path, nil))
		if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != "/login" {
			t.Fatalf("authentication bypass for %s %s: %d %q", request.method, request.path, response.Code, response.Body.String())
		}
	}
}

func TestRecoveryReadinessDoesNotPassTheRecoveryGate(t *testing.T) {
	// Hold one operation so import can only enter draining, never validate,
	// replace a database or dispatch a real restart signal.
	release, ok := database.TryAcquireAccess()
	if !ok {
		t.Fatal("initial operation rejected")
	}
	defer release()
	restore := database.RegisterRecoveryHooks(database.RecoveryHooks{
		Quiesce:          func() error { t.Error("unexpected quiesce"); return nil },
		ResumeAfterAbort: func() error { return nil },
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
	defer func() {
		cancel()
		if err := <-completed; !errors.Is(err, context.Canceled) {
			t.Errorf("drain cancellation = %v", err)
		}
	}()
	deadline := time.After(2 * time.Second)
	for !database.RecoveryBlocked() {
		select {
		case <-deadline:
			t.Fatal("recovery did not enter draining")
		case <-time.After(time.Millisecond):
		}
	}
	response := httptest.NewRecorder()
	newAPIHandlerTestRouter().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/recovery-ready", nil))
	if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "3" {
		t.Fatalf("readiness passed recovery: %d %q", response.Code, response.Body.String())
	}
}
