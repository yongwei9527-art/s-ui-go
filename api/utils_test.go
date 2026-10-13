package api

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Regression tests use an isolated HTTP router, without any real panel.
func TestUnauthenticatedRequestUsesCustomLoginPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("s-ui", cookie.NewStore([]byte("readonly-audit-test-secret"))))
	NewAPIHandler(r.Group("/panel/api"), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panel/api/settings", nil))
	t.Logf("status=%d Location=%q (desired: /panel/login)", w.Code, w.Header().Get("Location"))
	if w.Code != 307 || w.Header().Get("Location") != "/panel/login" {
		t.Fatalf("unexpected result: %d %s", w.Code, w.Header())
	}
}

func TestHostnameIPv6WithAndWithoutPort(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, host := range []string{"[2001:db8::1]", "[2001:db8::1]:443"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Request.Host = host
		got := getHostname(c)
		t.Logf("Host=%q getHostname=%q", host, got)
		if host == "[2001:db8::1]" && got != "[2001:db8::1]" {
			t.Fatalf("unexpected result: %q", got)
		}
		if host == "[2001:db8::1]:443" && got != "[2001:db8::1]" {
			t.Fatalf("control failed: %q", got)
		}
	}
}
