package api

import (
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/yongwei9527-art/s-ui-go/util/common"

	"github.com/gin-gonic/gin"
)

func TestAPIv2TokenCacheConcurrentAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &APIv2Handler{
		tokens: []TokenInMemory{{
			Token:    common.HashToken("test-token"),
			Username: "alice",
		}},
	}

	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Token", "test-token")

	const readers = 8
	const iterations = 200
	var wg sync.WaitGroup
	for range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
				ctx.Request = request
				if username := handler.findUsername(ctx); username != "alice" {
					t.Errorf("findUsername() = %q, want alice", username)
					return
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range iterations {
			handler.tokensMu.Lock()
			handler.tokens = []TokenInMemory{{
				Token:    common.HashToken("test-token"),
				Username: "alice",
			}}
			handler.tokensMu.Unlock()
		}
	}()
	wg.Wait()
}
