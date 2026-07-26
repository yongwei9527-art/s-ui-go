package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func newAPIHandlerTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(sessions.Sessions("s-ui", cookie.NewStore([]byte("logout-route-test-secret"))))
	NewAPIHandler(router.Group("/api"), nil)
	return router
}

func TestAPIHandlerLogoutRequiresPOST(t *testing.T) {
	router := newAPIHandlerTestRouter()

	t.Run("POST clears the session", func(t *testing.T) {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/api/logout", nil)

		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("POST /api/logout status = %d, want %d", response.Code, http.StatusOK)
		}

		var msg Msg
		if err := json.Unmarshal(response.Body.Bytes(), &msg); err != nil {
			t.Fatalf("POST /api/logout returned invalid JSON: %v", err)
		}
		if !msg.Success {
			t.Fatalf("POST /api/logout success = false, want true; msg = %q", msg.Msg)
		}

		cookies := response.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("POST /api/logout set %d cookies, want 1 session-clearing cookie", len(cookies))
		}
		if cookies[0].Name != "s-ui" || cookies[0].MaxAge >= 0 {
			t.Fatalf("POST /api/logout cookie = %#v, want cleared s-ui session cookie", cookies[0])
		}
	})

	t.Run("GET is rejected without clearing the session", func(t *testing.T) {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/logout", nil)

		router.ServeHTTP(response, request)

		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("GET /api/logout status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
		}
		if allow := response.Header().Get("Allow"); allow != http.MethodPost {
			t.Fatalf("GET /api/logout Allow = %q, want %q", allow, http.MethodPost)
		}
		if cookies := response.Result().Cookies(); len(cookies) != 0 {
			t.Fatalf("GET /api/logout set %d cookies, want none", len(cookies))
		}
	})
}
