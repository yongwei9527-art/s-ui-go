package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGetRemoteIPUsesForwardedHeaderOnlyForConfiguredProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("direct client cannot choose its IP", func(t *testing.T) {
		router := gin.New()
		if err := router.SetTrustedProxies(nil); err != nil {
			t.Fatalf("SetTrustedProxies: %v", err)
		}
		router.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, getRemoteIp(c)) })

		request := httptest.NewRequest(http.MethodGet, "/ip", nil)
		request.RemoteAddr = "198.51.100.40:1234"
		request.Header.Set("X-Forwarded-For", "203.0.113.250")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if got, want := response.Body.String(), "198.51.100.40"; got != want {
			t.Fatalf("remote IP = %q, want direct peer %q", got, want)
		}
	})

	t.Run("explicit loopback proxy may forward client IP", func(t *testing.T) {
		router := gin.New()
		if err := router.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
			t.Fatalf("SetTrustedProxies: %v", err)
		}
		router.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, getRemoteIp(c)) })

		request := httptest.NewRequest(http.MethodGet, "/ip", nil)
		request.RemoteAddr = "127.0.0.1:1234"
		request.Header.Set("X-Forwarded-For", "203.0.113.7")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if got, want := response.Body.String(), "203.0.113.7"; got != want {
			t.Fatalf("remote IP = %q, want forwarded client %q", got, want)
		}
	})
}

func TestSessionOptionsOnlyTrustsForwardedProtoFromExplicitProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newContext := func() *gin.Context {
		context, _ := gin.CreateTestContext(httptest.NewRecorder())
		context.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		context.Request.Header.Set("X-Forwarded-Proto", "https")
		return context
	}

	direct := newContext()
	if sessionOptions(direct, 0).Secure {
		t.Fatal("direct client X-Forwarded-Proto unexpectedly set Secure")
	}

	proxied := newContext()
	SetTrustedProxy(proxied, true)
	if !sessionOptions(proxied, 0).Secure {
		t.Fatal("explicit proxy X-Forwarded-Proto did not set Secure")
	}

	tlsRequest := newContext()
	tlsRequest.Request.TLS = &tls.ConnectionState{}
	if !sessionOptions(tlsRequest, 0).Secure {
		t.Fatal("TLS request did not set Secure")
	}
}
