package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/yongwei9527-art/s-ui-go/logger"

	"github.com/gin-gonic/gin"
)

const trustedProxyContextKey = "s-ui-trusted-proxy"

type Msg struct {
	Success bool        `json:"success"`
	Msg     string      `json:"msg"`
	Obj     interface{} `json:"obj"`
}

// SetTrustedProxy records whether the immediate TCP peer was an explicitly
// configured reverse proxy. Web server setup owns this decision; API handlers
// use it only for proxy-provided transport metadata such as X-Forwarded-Proto.
func SetTrustedProxy(c *gin.Context, trusted bool) {
	c.Set(trustedProxyContextKey, trusted)
}

func isTrustedProxy(c *gin.Context) bool {
	trusted, ok := c.Get(trustedProxyContextKey)
	return ok && trusted == true
}

func getRemoteIp(c *gin.Context) string {
	// ClientIP honors Gin's configured trusted-proxy list. The server configures
	// that list explicitly and defaults to trusting no proxy, so raw client
	// X-Forwarded-For headers cannot influence rate limiting or audit records.
	return c.ClientIP()
}

func getHostname(c *gin.Context) string {
	host := c.Request.Host
	if strings.Contains(host, ":") {
		if parsed, _, err := net.SplitHostPort(host); err == nil {
			host = parsed
		}
		if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
			host = "[" + host + "]"
		}
	}
	return host
}

func jsonMsg(c *gin.Context, msg string, err error) {
	jsonMsgObj(c, msg, nil, err)
}

func jsonObj(c *gin.Context, obj interface{}, err error) {
	jsonMsgObj(c, "", obj, err)
}

func jsonMsgObj(c *gin.Context, msg string, obj interface{}, err error) {
	m := Msg{
		Obj: obj,
	}
	if err == nil {
		m.Success = true
		if msg != "" {
			m.Msg = msg
		}
	} else {
		m.Success = false
		m.Msg = msg + ": " + err.Error()
		logger.Warning("failed :", err)
	}
	c.JSON(http.StatusOK, m)
}

func pureJsonMsg(c *gin.Context, success bool, msg string) {
	if success {
		c.JSON(http.StatusOK, Msg{
			Success: true,
			Msg:     msg,
		})
	} else {
		c.JSON(http.StatusOK, Msg{
			Success: false,
			Msg:     msg,
		})
	}
}

func checkLogin(c *gin.Context) {
	if !IsLogin(c) {
		if c.GetHeader("X-Requested-With") == "XMLHttpRequest" {
			pureJsonMsg(c, false, "Invalid login")
		} else {
			loginPath := c.GetString("s-ui-login-url")
			if loginPath == "" {
				loginPath = "/login"
			}
			c.Redirect(http.StatusTemporaryRedirect, loginPath)
		}
		c.Abort()
	} else {
		c.Next()
	}
}
