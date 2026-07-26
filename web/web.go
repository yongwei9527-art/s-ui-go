package web

import (
	"context"
	"crypto/tls"
	"errors"
	"html/template"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/yongwei9527-art/s-ui-go/api"
	"github.com/yongwei9527-art/s-ui-go/config"
	frontend "github.com/yongwei9527-art/s-ui-go/frontend"
	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/middleware"
	"github.com/yongwei9527-art/s-ui-go/network"
	"github.com/yongwei9527-art/s-ui-go/service"
	"github.com/yongwei9527-art/s-ui-go/util/common"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

const (
	// These limits bound the time and memory a client can consume before a
	// request is handled. Keep them together so resource-protection changes
	// are reviewed as a single server policy.
	serverReadHeaderTimeout = 10 * time.Second
	serverReadTimeout       = 30 * time.Second
	serverWriteTimeout      = 60 * time.Second
	serverIdleTimeout       = 120 * time.Second
	serverMaxHeaderBytes    = 1 << 20 // 1 MiB

	securityContentSecurityPolicy = "frame-ancestors 'self'"
	securityReferrerPolicy        = "strict-origin-when-cross-origin"
)

const trustedProxiesEnvironment = "S_UI_TRUSTED_PROXIES"

// configureTrustedProxyPolicy makes forwarded client and protocol headers opt-in.
// By default, the application treats the directly connected peer as the client.
func configureTrustedProxyPolicy(engine *gin.Engine) (gin.HandlerFunc, error) {
	return newTrustedProxyPolicy(engine, os.Getenv(trustedProxiesEnvironment))
}

func newTrustedProxyPolicy(engine *gin.Engine, rawProxies string) (gin.HandlerFunc, error) {
	proxyStrings := make([]string, 0)
	proxyPrefixes := make([]netip.Prefix, 0)

	for _, rawProxy := range strings.Split(rawProxies, ",") {
		proxy := strings.TrimSpace(rawProxy)
		if proxy == "" {
			continue
		}

		prefix, err := netip.ParsePrefix(proxy)
		if err != nil {
			address, addressErr := netip.ParseAddr(proxy)
			if addressErr != nil {
				return nil, common.NewError("invalid trusted proxy ", proxy)
			}
			prefix = netip.PrefixFrom(address, address.BitLen())
		}
		prefix = prefix.Masked()
		if prefix.Bits() == 0 {
			return nil, common.NewError("trusted proxy must not trust every address")
		}

		proxyStrings = append(proxyStrings, proxy)
		proxyPrefixes = append(proxyPrefixes, prefix)
	}

	if err := engine.SetTrustedProxies(proxyStrings); err != nil {
		return nil, common.NewError("configure trusted proxies", err)
	}

	return func(c *gin.Context) {
		api.SetTrustedProxy(c, remoteAddressIsTrusted(c.Request.RemoteAddr, proxyPrefixes))
		c.Next()
	}, nil
}

func remoteAddressIsTrusted(remoteAddress string, trustedProxies []netip.Prefix) bool {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}

	address, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	address = address.Unmap()
	for _, trustedProxy := range trustedProxies {
		if trustedProxy.Contains(address) {
			return true
		}
	}
	return false
}

type Server struct {
	httpServer     *http.Server
	listener       net.Listener
	ctx            context.Context
	cancel         context.CancelFunc
	settingService service.SettingService
}

func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Server) initRouter() (*gin.Engine, error) {
	if config.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.Default()
	trustedProxyMiddleware, err := configureTrustedProxyPolicy(engine)
	if err != nil {
		return nil, err
	}
	engine.Use(trustedProxyMiddleware)
	engine.Use(securityHeaders())

	// Load the HTML template
	t := template.New("").Funcs(engine.FuncMap)
	template, err := t.ParseFS(frontend.Content, "dist/index.html")
	if err != nil {
		return nil, err
	}
	engine.SetHTMLTemplate(template)

	base_url, err := s.settingService.GetWebPath()
	if err != nil {
		return nil, err
	}

	webDomain, err := s.settingService.GetWebDomain()
	if err != nil {
		return nil, err
	}

	if webDomain != "" {
		engine.Use(middleware.DomainValidator(webDomain))
	}

	secret, err := s.settingService.GetSecret()
	if err != nil {
		return nil, err
	}

	engine.Use(gzip.Gzip(gzip.DefaultCompression))
	assetsBasePath := base_url + "assets/"

	store := cookie.NewStore(secret)
	engine.Use(sessions.Sessions("s-ui", store))

	engine.Use(func(c *gin.Context) {
		uri := c.Request.RequestURI
		if strings.HasPrefix(uri, assetsBasePath) {
			c.Header("Cache-Control", "max-age=31536000")
		}
	})

	// Serve the assets folder
	assetsFS, err := fs.Sub(frontend.Content, "dist/assets")
	if err != nil {
		return nil, common.NewError("load web assets", err)
	}

	engine.StaticFS(assetsBasePath, http.FS(assetsFS))

	group_apiv2 := engine.Group(base_url + "apiv2")
	apiv2 := api.NewAPIv2Handler(group_apiv2)

	group_api := engine.Group(base_url + "api")
	api.NewAPIHandler(group_api, apiv2)

	// Serve index.html as the entry point
	// Handle all other routes by serving index.html
	engine.NoRoute(func(c *gin.Context) {
		if c.Request.URL.Path == strings.TrimSuffix(base_url, "/") {
			c.Redirect(http.StatusTemporaryRedirect, base_url)
			return
		}
		if !strings.HasPrefix(c.Request.URL.Path, base_url) {
			c.String(404, "")
			return
		}
		if c.Request.URL.Path != base_url+"login" && !api.IsLogin(c) {
			c.Redirect(http.StatusTemporaryRedirect, base_url+"login")
			return
		}
		if c.Request.URL.Path == base_url+"login" && api.IsLogin(c) {
			c.Redirect(http.StatusTemporaryRedirect, base_url)
			return
		}
		c.HTML(http.StatusOK, "index.html", gin.H{"BASE_URL": base_url})
	})

	return engine, nil
}

// securityHeaders applies response headers that protect browser clients without
// constraining the single-page application's scripts, styles, API calls, or assets.
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", securityReferrerPolicy)
		c.Header("Content-Security-Policy", securityContentSecurityPolicy)
		c.Next()
	}
}

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
		MaxHeaderBytes:    serverMaxHeaderBytes,
	}
}

func (s *Server) Start() (err error) {
	//This is an anonymous function, no function name
	defer func() {
		if err != nil {
			s.Stop()
		}
	}()

	engine, err := s.initRouter()
	if err != nil {
		return err
	}

	certFile, err := s.settingService.GetCertFile()
	if err != nil {
		return err
	}
	keyFile, err := s.settingService.GetKeyFile()
	if err != nil {
		return err
	}
	listen, err := s.settingService.GetListen()
	if err != nil {
		return err
	}
	port, err := s.settingService.GetPort()
	if err != nil {
		return err
	}
	listenAddr := net.JoinHostPort(listen, strconv.Itoa(port))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	useTLS := certFile != "" && keyFile != ""
	if certFile != "" && keyFile == "" || certFile == "" && keyFile != "" {
		listener.Close()
		return common.NewError("web TLS certificate and key files must be configured together")
	}
	if useTLS {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			listener.Close()
			return err
		}
		c := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		listener = network.NewAutoHttpsListener(listener)
		listener = tls.NewListener(listener, c)
	}

	if useTLS {
		logger.Info("web server run https on", listener.Addr())
	} else {
		logger.Info("web server run http on", listener.Addr())
	}
	s.listener = listener

	s.httpServer = newHTTPServer(engine)

	go func() {
		if serveErr := s.httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Error("web server stopped unexpectedly:", serveErr)
		}
	}()

	return nil
}

func (s *Server) Stop() error {
	var err error
	if s.httpServer != nil {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
		err = s.httpServer.Shutdown(shutdownCtx)
		cancelShutdown()
		if err != nil {
			s.cancel()
			if s.listener != nil {
				_ = s.listener.Close()
			}
			return err
		}
	} else if s.listener != nil {
		err = s.listener.Close()
		if err != nil {
			s.cancel()
			return err
		}
	}
	s.cancel()
	return nil
}

func (s *Server) GetCtx() context.Context {
	return s.ctx
}
