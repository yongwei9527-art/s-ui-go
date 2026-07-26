package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yongwei9527-art/s-ui-go/service"
	"github.com/yongwei9527-art/s-ui-go/util/common"

	"github.com/gin-gonic/gin"
)

type APIHandler struct {
	ApiService
	apiv2              *APIv2Handler
	loginLimiter       *loginRateLimiter
	loginAuthenticator func(username, password, remoteIP string) (*service.LoginResult, error)
	sessionMaxAge      func() (int, error)
}

func NewAPIHandler(g *gin.RouterGroup, a2 *APIv2Handler) {
	a := &APIHandler{
		apiv2:        a2,
		loginLimiter: newLoginRateLimiter(defaultLoginRateLimiterConfig()),
	}
	a.loginAuthenticator = a.UserService.Login
	a.sessionMaxAge = a.SettingService.GetSessionMaxAge
	a.initRouter(g)
}

const (
	invalidLoginMessage        = "Invalid username or password."
	loginRateLimitedMessage    = "Too many login attempts. Please try again later."
	loginSessionFailureMessage = "Unable to create login session."
)

func (a *APIHandler) login(c *gin.Context) {
	username := c.Request.FormValue("user")
	password := c.Request.FormValue("pass")
	remoteIP := getRemoteIp(c)

	if retryAfter := a.loginLimiter.RetryAfter(remoteIP, username); retryAfter > 0 {
		seconds := int((retryAfter + time.Second - 1) / time.Second)
		c.Header("Retry-After", strconv.Itoa(seconds))
		c.JSON(http.StatusTooManyRequests, Msg{Success: false, Msg: loginRateLimitedMessage})
		return
	}

	loginResult, err := a.loginAuthenticator(username, password, remoteIP)
	if err != nil {
		a.loginLimiter.RecordFailure(remoteIP, username)
		c.JSON(http.StatusUnauthorized, Msg{Success: false, Msg: invalidLoginMessage})
		return
	}

	// Valid credentials reset the consecutive failure count before session
	// creation so a transient session-store error does not lock out the user.
	a.loginLimiter.Clear(remoteIP, username)

	sessionMaxAge, err := a.sessionMaxAge()
	if err != nil {
		sessionMaxAge = 0
	}
	if err := SetLoginUser(c, loginResult.Username, sessionMaxAge); err != nil {
		c.JSON(http.StatusInternalServerError, Msg{Success: false, Msg: loginSessionFailureMessage})
		return
	}

	jsonObj(c, loginResult, nil)
}

func (a *APIHandler) initRouter(g *gin.RouterGroup) {
	g.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasSuffix(path, "login") && !strings.HasSuffix(path, "logout") {
			checkLogin(c)
		}
	})
	g.POST("/:postAction", a.postHandler)
	g.GET("/:getAction", a.getHandler)
}

func (a *APIHandler) postHandler(c *gin.Context) {
	loginUser := GetLoginUser(c)
	action := c.Param("postAction")

	switch action {
	case "login":
		a.login(c)
	case "logout":
		a.ApiService.Logout(c)
	case "changePass":
		a.ApiService.ChangePass(c)
	case "save":
		a.ApiService.Save(c, loginUser)
	case "restartApp":
		a.ApiService.RestartApp(c)
	case "restartSb":
		a.ApiService.RestartSb(c)
	case "xray-xhttp-save":
		a.ApiService.SaveXrayXHTTP(c)
	case "xray-xhttp-delete":
		a.ApiService.DeleteXrayXHTTP(c)
	case "restartXrayXHTTP":
		a.ApiService.RestartXrayXHTTP(c)
	case "linkConvert":
		a.ApiService.LinkConvert(c)
	case "subConvert":
		a.ApiService.SubConvert(c)
	case "importdb":
		a.ApiService.ImportDb(c)
	case "addToken":
		a.ApiService.AddToken(c)
		a.apiv2.ReloadTokens()
	case "deleteToken":
		a.ApiService.DeleteToken(c)
		a.apiv2.ReloadTokens()
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}

func (a *APIHandler) getHandler(c *gin.Context) {
	action := c.Param("getAction")

	switch action {
	case "logout":
		c.Header("Allow", http.MethodPost)
		c.Status(http.StatusMethodNotAllowed)
	case "load":
		a.ApiService.LoadData(c)
	case "inbounds", "outbounds", "endpoints", "services", "tls", "clients", "config":
		err := a.ApiService.LoadPartialData(c, []string{action})
		if err != nil {
			jsonMsg(c, action, err)
		}
		return
	case "users":
		a.ApiService.GetUsers(c)
	case "settings":
		a.ApiService.GetSettings(c)
	case "stats":
		a.ApiService.GetStats(c)
	case "status":
		a.ApiService.GetStatus(c)
	case "onlines":
		a.ApiService.GetOnlines(c)
	case "logs":
		a.ApiService.GetLogs(c)
	case "changes":
		a.ApiService.CheckChanges(c)
	case "keypairs":
		a.ApiService.GetKeypairs(c)
	case "getdb":
		a.ApiService.GetDb(c)
	case "tokens":
		a.ApiService.GetTokens(c)
	case "singbox-config":
		a.ApiService.GetSingboxConfig(c)
	case "xray-xhttp":
		a.ApiService.GetXrayXHTTP(c)
	case "checkOutbound":
		a.ApiService.GetCheckOutbound(c)
	default:
		jsonMsg(c, "failed", common.NewError("unknown action: ", action))
	}
}
