package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yongwei9527-art/s-ui-go/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func newLoginTestRouter(config loginRateLimiterConfig, authenticate func(username, password, remoteIP string) (*service.LoginResult, error)) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(sessions.Sessions("s-ui", cookie.NewStore([]byte("login-rate-limit-test-secret"))))
	handler := &APIHandler{
		loginLimiter:       newLoginRateLimiter(config),
		loginAuthenticator: authenticate,
		sessionMaxAge: func() (int, error) {
			return 30, nil
		},
	}
	handler.initRouter(router.Group("/api"))
	return router
}

func loginRequest(router *gin.Engine, username, password, clientIP string) *httptest.ResponseRecorder {
	form := url.Values{"user": {username}, "pass": {password}}
	request := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.RemoteAddr = net.JoinHostPort(clientIP, "4321")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func decodeLoginMessage(t *testing.T, response *httptest.ResponseRecorder) Msg {
	t.Helper()
	var message Msg
	if err := json.Unmarshal(response.Body.Bytes(), &message); err != nil {
		t.Fatalf("invalid login JSON response: %v; body = %s", err, response.Body.String())
	}
	return message
}

func TestLoginRateLimiterAllowsNormalLogin(t *testing.T) {
	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	router := newLoginTestRouter(loginRateLimiterConfig{Now: func() time.Time { return now }}, func(username, password, remoteIP string) (*service.LoginResult, error) {
		if username == "alice" && password == "correct horse battery staple" {
			return &service.LoginResult{Username: username}, nil
		}
		return nil, errors.New("invalid credentials")
	})

	response := loginRequest(router, "alice", "correct horse battery staple", "198.51.100.10")
	if response.Code != http.StatusOK {
		t.Fatalf("POST /api/login status = %d, want %d; body = %s", response.Code, http.StatusOK, response.Body.String())
	}
	message := decodeLoginMessage(t, response)
	if !message.Success {
		t.Fatalf("normal login success = false, want true; message = %q", message.Msg)
	}
	if cookies := response.Result().Cookies(); len(cookies) != 1 || cookies[0].Name != "s-ui" {
		t.Fatalf("normal login cookies = %#v, want one s-ui session cookie", cookies)
	}
}

func TestLoginRateLimiterBlocksAfterConsecutiveFailures(t *testing.T) {
	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	attempts := 0
	router := newLoginTestRouter(loginRateLimiterConfig{
		MaxFailures: 2,
		BaseDelay:   time.Minute,
		MaxDelay:    time.Minute,
		Now:         func() time.Time { return now },
	}, func(username, password, remoteIP string) (*service.LoginResult, error) {
		attempts++
		return nil, errors.New("credentials were wrong: " + password)
	})

	for attempt := 1; attempt <= 2; attempt++ {
		response := loginRequest(router, "alice", "secret-password", "198.51.100.10")
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d status = %d, want %d", attempt, response.Code, http.StatusUnauthorized)
		}
		message := decodeLoginMessage(t, response)
		if message.Msg != invalidLoginMessage || strings.Contains(response.Body.String(), "secret-password") {
			t.Fatalf("failure %d returned a non-uniform or sensitive error: %s", attempt, response.Body.String())
		}
	}

	response := loginRequest(router, "alice", "secret-password", "198.51.100.10")
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limited status = %d, want %d; body = %s", response.Code, http.StatusTooManyRequests, response.Body.String())
	}
	if retryAfter := response.Header().Get("Retry-After"); retryAfter != "60" {
		t.Fatalf("Retry-After = %q, want 60", retryAfter)
	}
	message := decodeLoginMessage(t, response)
	if message.Msg != loginRateLimitedMessage || strings.Contains(response.Body.String(), "secret-password") {
		t.Fatalf("rate-limited response was non-uniform or sensitive: %s", response.Body.String())
	}
	if attempts != 2 {
		t.Fatalf("authenticator attempts = %d, want 2; rate-limited request should not authenticate", attempts)
	}
}

func TestLoginRateLimiterSuccessfulLoginClearsFailures(t *testing.T) {
	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	config := loginRateLimiterConfig{MaxFailures: 2, BaseDelay: time.Minute, MaxDelay: time.Minute, Now: func() time.Time { return now }}
	var handler *APIHandler
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("s-ui", cookie.NewStore([]byte("login-rate-limit-reset-secret"))))
	handler = &APIHandler{
		loginLimiter: newLoginRateLimiter(config),
		loginAuthenticator: func(username, password, remoteIP string) (*service.LoginResult, error) {
			if password == "correct" {
				return &service.LoginResult{Username: username}, nil
			}
			return nil, errors.New("invalid credentials")
		},
		sessionMaxAge: func() (int, error) { return 30, nil },
	}
	handler.initRouter(router.Group("/api"))

	if response := loginRequest(router, "alice", "wrong", "198.51.100.10"); response.Code != http.StatusUnauthorized {
		t.Fatalf("first failed login status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if response := loginRequest(router, "alice", "correct", "198.51.100.10"); response.Code != http.StatusOK {
		t.Fatalf("successful login status = %d, want %d", response.Code, http.StatusOK)
	}
	if retryAfter := handler.loginLimiter.RetryAfter("198.51.100.10", "alice"); retryAfter != 0 {
		t.Fatalf("retry after successful login = %s, want 0", retryAfter)
	}

	if response := loginRequest(router, "alice", "wrong", "198.51.100.10"); response.Code != http.StatusUnauthorized {
		t.Fatalf("first post-success failure status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if response := loginRequest(router, "alice", "wrong", "198.51.100.10"); response.Code != http.StatusUnauthorized {
		t.Fatalf("second post-success failure status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestLoginRateLimiterIsolatesIPAndUsername(t *testing.T) {
	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	router := newLoginTestRouter(loginRateLimiterConfig{
		MaxFailures: 2,
		BaseDelay:   time.Minute,
		MaxDelay:    time.Minute,
		Now:         func() time.Time { return now },
	}, func(username, password, remoteIP string) (*service.LoginResult, error) {
		return nil, errors.New("invalid credentials")
	})

	for range 2 {
		if response := loginRequest(router, "alice", "wrong", "198.51.100.10"); response.Code != http.StatusUnauthorized {
			t.Fatalf("initial failure status = %d, want %d", response.Code, http.StatusUnauthorized)
		}
	}
	if response := loginRequest(router, "alice", "wrong", "198.51.100.10"); response.Code != http.StatusTooManyRequests {
		t.Fatalf("same IP and username status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
	if response := loginRequest(router, "alice", "wrong", "198.51.100.11"); response.Code != http.StatusUnauthorized {
		t.Fatalf("different IP status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if response := loginRequest(router, "bob", "wrong", "198.51.100.10"); response.Code != http.StatusUnauthorized {
		t.Fatalf("different username status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestLoginRateLimiterConcurrentAccess(t *testing.T) {
	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	limiter := newLoginRateLimiter(loginRateLimiterConfig{
		MaxFailures: 1000,
		BaseDelay:   time.Second,
		MaxDelay:    time.Minute,
		Now:         func() time.Time { return now },
	})

	const attempts = 100
	var waitGroup sync.WaitGroup
	for range attempts {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			limiter.RecordFailure("198.51.100.10", "alice")
			_ = limiter.RetryAfter("198.51.100.10", "alice")
		}()
	}
	waitGroup.Wait()

	limiter.mu.Lock()
	entry := limiter.entries[newLoginRateLimitKey("198.51.100.10", "alice")]
	limiter.mu.Unlock()
	if entry.failures != attempts {
		t.Fatalf("concurrent failure count = %d, want %d", entry.failures, attempts)
	}
}
