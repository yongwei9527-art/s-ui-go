package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTrustedProxyPolicyMakesForwardedHeadersOptIn(t *testing.T) {
	gin.SetMode(gin.TestMode)

	serveClientIP := func(t *testing.T, trustedProxies string, peer string) string {
		t.Helper()
		router := gin.New()
		policy, err := newTrustedProxyPolicy(router, trustedProxies)
		if err != nil {
			t.Fatalf("newTrustedProxyPolicy: %v", err)
		}
		router.Use(policy)
		router.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

		request := httptest.NewRequest(http.MethodGet, "/ip", nil)
		request.RemoteAddr = peer
		request.Header.Set("X-Forwarded-For", "203.0.113.7")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response.Body.String()
	}

	if got, want := serveClientIP(t, "", "198.51.100.40:1234"), "198.51.100.40"; got != want {
		t.Errorf("untrusted forwarded client IP = %q, want direct peer %q", got, want)
	}
	if got, want := serveClientIP(t, "127.0.0.1", "127.0.0.1:1234"), "203.0.113.7"; got != want {
		t.Errorf("trusted forwarded client IP = %q, want %q", got, want)
	}
}

func TestTrustedProxyPolicyRejectsUnsafeOrInvalidNetworks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, trustedProxies := range []string{"0.0.0.0/0", "::/0", "not-an-address"} {
		t.Run(trustedProxies, func(t *testing.T) {
			if _, err := newTrustedProxyPolicy(gin.New(), trustedProxies); err == nil {
				t.Fatalf("newTrustedProxyPolicy(%q) unexpectedly succeeded", trustedProxies)
			}
		})
	}
}
