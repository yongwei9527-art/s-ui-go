package middleware

import (
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func DomainValidator(domain string) gin.HandlerFunc {
	normalizedDomain, domainOK := normalizeHost(domain)
	return func(c *gin.Context) {
		if !domainOK {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		host, hostOK := normalizeHost(c.Request.Host)
		if !hostOK || host != normalizedDomain {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		c.Next()
	}
}

func normalizeHost(value string) (string, bool) {
	if value == "" || strings.TrimSpace(value) != value {
		return "", false
	}

	host, ok := stripOptionalPort(value)
	if !ok || host == "" {
		return "", false
	}
	if strings.HasPrefix(host, "[") || strings.Contains(host, "]") {
		return "", false
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return addr.String(), true
	}

	host = strings.TrimRight(host, ".")
	if host == "" || strings.ContainsAny(host, "[]:/\\@") {
		return "", false
	}
	return strings.ToLower(host), true
}

func stripOptionalPort(value string) (string, bool) {
	if strings.HasPrefix(value, "[") {
		end := strings.Index(value, "]")
		if end == -1 {
			return "", false
		}
		inside := value[1:end]
		addr, err := netip.ParseAddr(inside)
		if err != nil || !addr.Is6() {
			return "", false
		}
		rest := value[end+1:]
		if rest == "" {
			return addr.String(), true
		}
		if !strings.HasPrefix(rest, ":") || !validPort(rest[1:]) {
			return "", false
		}
		return addr.String(), true
	}

	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.String(), true
	}

	switch strings.Count(value, ":") {
	case 0:
		return value, true
	case 1:
		host, port, _ := strings.Cut(value, ":")
		if host == "" || !validPort(port) {
			return "", false
		}
		return host, true
	default:
		return "", false
	}
}

func validPort(port string) bool {
	if port == "" {
		return false
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return false
		}
	}
	value, err := strconv.Atoi(port)
	return err == nil && value >= 0 && value <= 65535
}
