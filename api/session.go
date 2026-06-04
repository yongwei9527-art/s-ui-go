package api

import (
	"encoding/gob"
	"net/http"
	"strings"

	"github.com/yongwei9527-art/s-ui-go/database/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	loginUser = "LOGIN_USER"
)

func init() {
	gob.Register(model.User{})
}

func sessionOptions(c *gin.Context, maxAge int) sessions.Options {
	options := sessions.Options{
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https"),
	}
	if maxAge > 0 {
		options.MaxAge = maxAge * 60
	}
	return options
}

func SetLoginUser(c *gin.Context, userName string, maxAge int) error {
	options := sessionOptions(c, maxAge)

	s := sessions.Default(c)
	s.Set(loginUser, userName)
	s.Options(options)

	return s.Save()
}

func SetMaxAge(c *gin.Context) error {
	s := sessions.Default(c)
	s.Options(sessionOptions(c, 0))
	return s.Save()
}

func GetLoginUser(c *gin.Context) string {
	s := sessions.Default(c)
	obj := s.Get(loginUser)
	if obj == nil {
		return ""
	}
	objStr, ok := obj.(string)
	if !ok {
		return ""
	}
	return objStr
}

func IsLogin(c *gin.Context) bool {
	return GetLoginUser(c) != ""
}

func ClearSession(c *gin.Context) {
	s := sessions.Default(c)
	s.Clear()
	options := sessionOptions(c, 0)
	options.MaxAge = -1
	s.Options(options)
	s.Save()
}
