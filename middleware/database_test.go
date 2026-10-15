package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/yongwei9527-art/s-ui-go/database"
)

func blockedDatabaseForTest(t *testing.T) func() {
	t.Helper()
	restore := database.RegisterRecoveryHooks(database.RecoveryHooks{Quiesce: func() error { t.Error("unexpected quiesce"); return nil }, ResumeAfterAbort: func() error { return nil }})
	t.Cleanup(restore)
	file, err := os.Create(filepath.Join(t.TempDir(), "upload.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	release, ok := database.TryAcquireAccess()
	if !ok {
		t.Fatal("initial lease rejected")
	}
	ctx, cancel := context.WithCancel(context.Background())
	completed := make(chan error, 1)
	go func() { completed <- database.ImportDBContext(ctx, file) }()
	deadline := time.After(2 * time.Second)
	for !database.RecoveryBlocked() {
		select {
		case <-deadline:
			cancel()
			release()
			t.Fatal("drain did not begin")
		case <-time.After(time.Millisecond):
		}
	}
	return func() {
		cancel()
		if err := <-completed; !errors.Is(err, context.Canceled) {
			t.Errorf("drain error = %v", err)
		}
		release()
	}
}

func TestDatabaseAccessRejectsNewAPIAndSubscriptionRequestsDuringRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	finish := blockedDatabaseForTest(t)
	defer finish()
	for _, allowImport := range []bool{true, false} {
		router := gin.New()
		g := router.Group("/api")
		g.Use(DatabaseAccess(allowImport))
		called := false
		g.Any("/:postAction", func(c *gin.Context) { called = true; c.Status(http.StatusOK) })
		for _, request := range []struct{ method, action string }{{"GET", "load"}, {"POST", "save"}, {"POST", "importdb"}} {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(request.method, "/api/"+request.action, nil))
			if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "3" || called {
				t.Fatalf("request crossed recovery gate: %s %s status=%d called=%v", request.method, request.action, response.Code, called)
			}
		}
	}
}

func TestOnlyPOSTImportEndpointBypassesItsOwnReadLease(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		allow       bool
		method      string
		wantBlocked bool
	}{{true, "POST", false}, {true, "GET", true}, {false, "POST", true}} {
		router := gin.New()
		g := router.Group("/api")
		g.Use(DatabaseAccess(tc.allow))
		g.Any("/:postAction", func(c *gin.Context) {
			restore := database.RegisterRecoveryHooks(database.RecoveryHooks{Quiesce: func() error { return nil }, ResumeAfterAbort: func() error { return nil }})
			defer restore()
			file, err := os.Create(filepath.Join(t.TempDir(), "invalid.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			err = database.ImportDBContext(ctx, file) // Empty upload never swaps or sends a signal.
			if blocked := errors.Is(err, context.DeadlineExceeded); blocked != tc.wantBlocked {
				t.Fatalf("self read lease: allow=%v method=%s err=%v", tc.allow, tc.method, err)
			}
			c.Status(http.StatusOK)
		})
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, "/api/importdb", nil))
	}
}
