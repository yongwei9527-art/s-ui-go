package sub

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/op/go-logging"
	"github.com/yongwei9527-art/s-ui-go/logger"

	"github.com/gin-gonic/gin"
)

func TestSubHandlerWarningHeaderKeepsSubscriptionHeadersAndSanitizesWarnings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)

	(&SubHandler{}).addHeaders(context, []string{
		"upload=1; download=2; total=3; expire=4",
		"12",
		"example-title",
		"first warning",
		"first warning",
		"second\r\nwarning",
	})

	if got := recorder.Header().Get("Subscription-Userinfo"); got != "upload=1; download=2; total=3; expire=4" {
		t.Fatalf("Subscription-Userinfo = %q", got)
	}
	if got := recorder.Header().Get("Profile-Update-Interval"); got != "12" {
		t.Fatalf("Profile-Update-Interval = %q", got)
	}
	if got := recorder.Header().Get("Profile-Title"); got != "example-title" {
		t.Fatalf("Profile-Title = %q", got)
	}
	if got := recorder.Header().Get(subscriptionWarningHeader); got != "first warning | second  warning" {
		t.Fatalf("warning header = %q", got)
	}
}

func TestSubscriptionWarningHeaderValueTruncatesAtSafeUTF8Boundary(t *testing.T) {
	value := subscriptionWarningHeaderValue([]string{strings.Repeat("\u4e2d", maxSubscriptionWarningHeader)})
	if len(value) > maxSubscriptionWarningHeader {
		t.Fatalf("warning header has %d bytes, limit is %d", len(value), maxSubscriptionWarningHeader)
	}
	if !strings.HasSuffix(value, "...") {
		t.Fatalf("truncated warning header should end with ellipsis: %q", value[len(value)-10:])
	}
	if !utf8.ValidString(value) {
		t.Fatalf("warning header is not valid UTF-8")
	}
}

func TestSubHeadersClashUsesGetCompatibleWarningsWithoutBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var gotSubID, gotHostname string
	handler := &SubHandler{
		getClashHeadersForHead: func(subID string, hostname string) ([]string, error) {
			gotSubID = subID
			gotHostname = hostname
			// These are the headers emitted by the same Clash generation path as GET.
			return []string{
				"upload=1; download=2; total=3; expire=4",
				"12",
				"example-title",
				"Clash/Mihomo subscription skipped outbound \"naive\"",
			}, nil
		},
		getLightweightHeadersForHead: func(string) ([]string, error) {
			t.Fatal("Clash HEAD must not use the lightweight header-only path")
			return nil, nil
		},
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodHead, "http://subscription.example/client-a?format=clash", nil)
	context.Params = gin.Params{{Key: "subid", Value: "client-a"}}
	handler.subHeaders(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if gotSubID != "client-a" || gotHostname != "subscription.example" {
		t.Fatalf("Clash HEAD lookup = (%q, %q), want (%q, %q)", gotSubID, gotHostname, "client-a", "subscription.example")
	}
	if got := recorder.Header().Get(subscriptionWarningHeader); got != "Clash/Mihomo subscription skipped outbound \"naive\"" {
		t.Fatalf("warning header = %q", got)
	}
	if got := recorder.Body.String(); got != "" {
		t.Fatalf("HEAD response body = %q, want empty", got)
	}
}

func TestSubHeadersNormalAndJSONStayLightweight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, target := range []string{"http://subscription.example/client-a", "http://subscription.example/client-a?format=json"} {
		t.Run(target, func(t *testing.T) {
			handler := &SubHandler{
				getClashHeadersForHead: func(string, string) ([]string, error) {
					t.Fatal("normal and JSON HEAD must not generate a Clash subscription")
					return nil, nil
				},
				getLightweightHeadersForHead: func(subID string) ([]string, error) {
					if subID != "client-a" {
						t.Fatalf("lightweight lookup subID = %q", subID)
					}
					return []string{"upload=1", "12", "example-title"}, nil
				},
			}

			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodHead, target, nil)
			context.Params = gin.Params{{Key: "subid", Value: "client-a"}}
			handler.subHeaders(context)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if got := recorder.Header().Get(subscriptionWarningHeader); got != "" {
				t.Fatalf("normal/JSON HEAD warning header = %q, want empty", got)
			}
			if got := recorder.Body.String(); got != "" {
				t.Fatalf("HEAD response body = %q, want empty", got)
			}
		})
	}
}

func TestSubHeadersClashFailureHasNoBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger.InitLogger(logging.ERROR)
	handler := &SubHandler{
		getClashHeadersForHead: func(string, string) ([]string, error) {
			return nil, errors.New("Clash generation failed")
		},
	}

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodHead, "http://subscription.example/client-a?format=clash", nil)
	context.Params = gin.Params{{Key: "subid", Value: "client-a"}}
	handler.subHeaders(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if got := recorder.Body.String(); got != "" {
		t.Fatalf("failed Clash HEAD response body = %q, want empty", got)
	}
}
