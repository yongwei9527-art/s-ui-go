package util

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/op/go-logging"
	projectlogger "github.com/yongwei9527-art/s-ui-go/logger"
)

var initExternalSubscriptionLogger sync.Once

func initializeExternalSubscriptionLogger() {
	initExternalSubscriptionLogger.Do(func() {
		projectlogger.InitLogger(logging.ERROR)
	})
}

func TestGetExternalLinkAcceptsSuccessfulResponse(t *testing.T) {
	initializeExternalSubscriptionLogger()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		_, _ = w.Write([]byte("trojan://example"))
	}))
	defer server.Close()

	if got := GetExternalLink(server.URL); got != "trojan://example" {
		t.Fatalf("GetExternalLink() = %q, want subscription content", got)
	}
}

func TestGetExternalLinkRejectsBadStatusAndOversizedResponse(t *testing.T) {
	initializeExternalSubscriptionLogger()
	t.Run("bad status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}))
		defer server.Close()

		if got := GetExternalLink(server.URL); got != "" {
			t.Fatalf("GetExternalLink() = %q, want empty result for HTTP failure", got)
		}
	})

	t.Run("oversized response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("a", maxExternalSubscriptionBytes+1)))
		}))
		defer server.Close()

		if got := GetExternalLink(server.URL); got != "" {
			t.Fatalf("GetExternalLink() = %q, want empty result for oversized response", got)
		}
	})
}

func TestGetExternalLinkHonorsTimeout(t *testing.T) {
	initializeExternalSubscriptionLogger()
	oldTimeout := externalSubscriptionTimeout
	externalSubscriptionTimeout = 50 * time.Millisecond
	t.Cleanup(func() { externalSubscriptionTimeout = oldTimeout })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	start := time.Now()
	if got := GetExternalLink(server.URL); got != "" {
		t.Fatalf("GetExternalLink() = %q, want timeout failure", got)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("GetExternalLink() took %s, want it to honor timeout", elapsed)
	}
}
