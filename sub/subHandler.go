package sub

import (
	"net"
	"strings"
	"unicode/utf8"

	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/service"

	"github.com/gin-gonic/gin"
)

const (
	subscriptionWarningHeader    = "X-SUI-Subscription-Warning"
	maxSubscriptionWarningHeader = 2048
)

type SubHandler struct {
	service.SettingService
	SubService
	JsonService
	ClashService

	// getClashHeadersForHead and getLightweightHeadersForHead keep the
	// HTTP handler testable without changing its public API. Production uses the
	// embedded services when these hooks are nil.
	getClashHeadersForHead       func(string, string) ([]string, error)
	getLightweightHeadersForHead func(string) ([]string, error)
}

func NewSubHandler(g *gin.RouterGroup) {
	a := &SubHandler{}
	a.initRouter(g)
}

func (s *SubHandler) initRouter(g *gin.RouterGroup) {
	g.GET("/:subid", s.subs)
	g.HEAD("/:subid", s.subHeaders)
}

func (s *SubHandler) subs(c *gin.Context) {
	var headers []string
	var result *string
	var err error
	subId := c.Param("subid")
	hostname := getRequestHostname(c)
	format, isFormat := c.GetQuery("format")
	if isFormat {
		switch format {
		case "json":
			result, headers, err = s.JsonService.GetJson(subId, format, hostname)
		case "clash":
			result, headers, err = s.ClashService.GetClash(subId, hostname)
		}
		if err != nil || result == nil {
			logger.Error(err)
			c.String(400, "Error!")
			return
		}
	} else {
		result, headers, err = s.SubService.GetSubs(subId, hostname)
		if err != nil || result == nil {
			logger.Error(err)
			c.String(400, "Error!")
			return
		}
	}

	s.addHeaders(c, headers)

	c.String(200, *result)
}

func getRequestHostname(c *gin.Context) string {
	host := c.Request.Host
	if strings.Contains(host, ":") {
		if splitHost, _, err := net.SplitHostPort(host); err == nil {
			host = splitHost
			if strings.Contains(host, ":") {
				host = "[" + strings.Trim(host, "[]") + "]"
			}
		}
	}
	return host
}

func (s *SubHandler) subHeaders(c *gin.Context) {
	subId := c.Param("subid")
	if c.Query("format") == "clash" {
		headers, err := s.clashHeadersForHead(subId, getRequestHostname(c))
		if err != nil {
			logger.Error(err)
			// A successful HEAD response must never carry the generated Clash
			// body. Keep the diagnostic failure bodyless as well.
			c.AbortWithStatus(400)
			return
		}
		s.addHeaders(c, headers)
		c.Status(200)
		return
	}

	headers, err := s.lightweightHeadersForHead(subId)
	if err != nil {
		logger.Error(err)
		c.String(400, "Error!")
		return
	}
	s.addHeaders(c, headers)
	c.Status(200)
}

func (s *SubHandler) clashHeadersForHead(subId string, hostname string) ([]string, error) {
	if s.getClashHeadersForHead != nil {
		return s.getClashHeadersForHead(subId, hostname)
	}
	_, headers, err := s.ClashService.GetClash(subId, hostname)
	return headers, err
}

func (s *SubHandler) lightweightHeadersForHead(subId string) ([]string, error) {
	if s.getLightweightHeadersForHead != nil {
		return s.getLightweightHeadersForHead(subId)
	}
	client, err := s.SubService.getClientBySubId(subId)
	if err != nil {
		return nil, err
	}
	return s.SubService.getClientHeaders(client), nil
}

func (s *SubHandler) addHeaders(c *gin.Context, headers []string) {
	if len(headers) > 0 {
		c.Writer.Header().Set("Subscription-Userinfo", headers[0])
	}
	if len(headers) > 1 {
		c.Writer.Header().Set("Profile-Update-Interval", headers[1])
	}
	if len(headers) > 2 {
		c.Writer.Header().Set("Profile-Title", headers[2])
	}
	if len(headers) > 3 {
		if warnings := subscriptionWarningHeaderValue(headers[3:]); warnings != "" {
			c.Writer.Header().Set(subscriptionWarningHeader, warnings)
		}
	}
}

// subscriptionWarningHeaderValue turns report warnings into one bounded header
// value. It removes control characters to prevent header injection, removes
// duplicate messages while retaining their first-seen order, and truncates on
// a UTF-8 boundary so proxies with conservative header limits can consume it.
func subscriptionWarningHeaderValue(warnings []string) string {
	unique := make([]string, 0, len(warnings))
	seen := make(map[string]struct{}, len(warnings))
	for _, warning := range warnings {
		warning = strings.TrimSpace(strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f {
				return ' '
			}
			return r
		}, warning))
		if warning == "" {
			continue
		}
		if _, ok := seen[warning]; ok {
			continue
		}
		seen[warning] = struct{}{}
		unique = append(unique, warning)
	}

	return truncateSubscriptionWarningHeader(strings.Join(unique, " | "))
}

func truncateSubscriptionWarningHeader(value string) string {
	if len(value) <= maxSubscriptionWarningHeader {
		return value
	}
	limit := maxSubscriptionWarningHeader - len("...")
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit] + "..."
}
