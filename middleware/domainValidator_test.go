package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
		ok   bool
	}{
		{name: "domain", host: "example.com", want: "example.com", ok: true},
		{name: "domain case", host: "EXAMPLE.COM", want: "example.com", ok: true},
		{name: "domain trailing dot", host: "example.com.", want: "example.com", ok: true},
		{name: "domain trailing dots", host: "example.com..", want: "example.com", ok: true},
		{name: "domain port", host: "EXAMPLE.COM.:8443", want: "example.com", ok: true},
		{name: "ipv4", host: "192.0.2.10", want: "192.0.2.10", ok: true},
		{name: "ipv4 port", host: "192.0.2.10:8080", want: "192.0.2.10", ok: true},
		{name: "ipv6 bracket", host: "[2001:db8::1]", want: "2001:db8::1", ok: true},
		{name: "ipv6 bracket port", host: "[2001:db8::1]:8443", want: "2001:db8::1", ok: true},
		{name: "ipv6 raw", host: "2001:db8::1", want: "2001:db8::1", ok: true},
		{name: "empty", host: "", ok: false},
		{name: "leading space", host: " example.com", ok: false},
		{name: "trailing space", host: "example.com ", ok: false},
		{name: "malformed port", host: "example.com:notaport", ok: false},
		{name: "empty port", host: "example.com:", ok: false},
		{name: "large port", host: "example.com:99999", ok: false},
		{name: "malformed ipv6 bracket", host: "[2001:db8::1", ok: false},
		{name: "domain with slash", host: "example.com/path", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeHost(tt.host)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("host = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDomainValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		domain     string
		host       string
		wantStatus int
	}{
		{name: "exact domain", domain: "example.com", host: "example.com", wantStatus: http.StatusOK},
		{name: "case-insensitive host", domain: "example.com", host: "EXAMPLE.COM", wantStatus: http.StatusOK},
		{name: "configured case normalized", domain: "EXAMPLE.COM", host: "example.com", wantStatus: http.StatusOK},
		{name: "trailing dot host", domain: "example.com", host: "example.com.", wantStatus: http.StatusOK},
		{name: "trailing dot config", domain: "example.com.", host: "example.com", wantStatus: http.StatusOK},
		{name: "host with port", domain: "example.com", host: "example.com:8080", wantStatus: http.StatusOK},
		{name: "case trailing dot port", domain: "example.com", host: "EXAMPLE.COM.:8443", wantStatus: http.StatusOK},
		{name: "ipv4 exact", domain: "192.0.2.10", host: "192.0.2.10", wantStatus: http.StatusOK},
		{name: "ipv4 with port", domain: "192.0.2.10", host: "192.0.2.10:8080", wantStatus: http.StatusOK},
		{name: "ipv6 bracketed", domain: "2001:db8::1", host: "[2001:db8::1]", wantStatus: http.StatusOK},
		{name: "ipv6 bracketed config", domain: "[2001:db8::1]", host: "2001:db8::1", wantStatus: http.StatusOK},
		{name: "ipv6 bracketed with port", domain: "2001:db8::1", host: "[2001:db8::1]:8443", wantStatus: http.StatusOK},
		{name: "ipv6 unbracketed", domain: "2001:db8::1", host: "2001:db8::1", wantStatus: http.StatusOK},
		{name: "suffix confusion subdomain", domain: "example.com", host: "evil.example.com", wantStatus: http.StatusForbidden},
		{name: "suffix confusion appended", domain: "example.com", host: "example.com.evil.com", wantStatus: http.StatusForbidden},
		{name: "suffix confusion prefix", domain: "example.com", host: "badexample.com", wantStatus: http.StatusForbidden},
		{name: "different domain", domain: "example.com", host: "other.com", wantStatus: http.StatusForbidden},
		{name: "empty config denies", domain: "", host: "example.com", wantStatus: http.StatusForbidden},
		{name: "empty host denies", domain: "example.com", host: "", wantStatus: http.StatusForbidden},
		{name: "malformed port denies", domain: "example.com", host: "example.com:notaport", wantStatus: http.StatusForbidden},
		{name: "malformed bracketed ipv6 denies", domain: "2001:db8::1", host: "[2001:db8::1", wantStatus: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(DomainValidator(tt.domain))
			router.GET("/", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = tt.host
			router.ServeHTTP(recorder, req)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}
