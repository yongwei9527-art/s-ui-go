package cmd

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type auditFakeIPTransport struct{}

func (auditFakeIPTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("<html>temporarily unavailable</html>")), Request: r}, nil
}
func TestPublicIPRejectsErrorHTML(t *testing.T) {
	saved := http.DefaultTransport
	http.DefaultTransport = auditFakeIPTransport{}
	defer func() { http.DefaultTransport = saved }()
	got := getPublicIP()
	t.Logf("all APIs return HTTP 503 HTML; getPublicIP()=%q", got)
	if got != "" {
		t.Fatalf("invalid response accepted: %q", got)
	}
	t.Logf("formatted URI host=%q", formatHostForURI(got))
}
