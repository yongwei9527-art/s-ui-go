package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestServerResourceProtectionDefaults(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	httpServer := newHTTPServer(handler)

	response := httptest.NewRecorder()
	httpServer.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusTeapot {
		t.Fatalf("handler response status = %d, want %d", response.Code, http.StatusTeapot)
	}

	if httpServer.ReadHeaderTimeout != 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %s, want %s", httpServer.ReadHeaderTimeout, 10*time.Second)
	}
	if httpServer.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %s, want %s", httpServer.ReadTimeout, 30*time.Second)
	}
	if httpServer.WriteTimeout != 60*time.Second {
		t.Errorf("WriteTimeout = %s, want %s", httpServer.WriteTimeout, 60*time.Second)
	}
	if httpServer.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %s, want %s", httpServer.IdleTimeout, 120*time.Second)
	}
	if httpServer.MaxHeaderBytes != 1<<20 {
		t.Errorf("MaxHeaderBytes = %d, want %d", httpServer.MaxHeaderBytes, 1<<20)
	}
}

func TestSecurityHeadersOnOrdinaryRequest(t *testing.T) {
	t.Parallel()

	engine := gin.New()
	engine.Use(securityHeaders())
	engine.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("response status = %d, want %d", response.Code, http.StatusNoContent)
	}

	wantHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "SAMEORIGIN",
		"Referrer-Policy":         securityReferrerPolicy,
		"Content-Security-Policy": securityContentSecurityPolicy,
	}
	for name, want := range wantHeaders {
		if got := response.Header().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}
