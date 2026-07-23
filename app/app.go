package app

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/core"
	"github.com/yongwei9527-art/s-ui-go/cronjob"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/service"
	"github.com/yongwei9527-art/s-ui-go/sub"
	"github.com/yongwei9527-art/s-ui-go/web"

	"github.com/op/go-logging"
)

type APP struct {
	service.SettingService
	configService    *service.ConfigService
	xrayXHTTPService *service.XrayXHTTPService
	webServer        *web.Server
	subServer        *sub.Server
	cronJob          *cronjob.CronJob
	logger           *logging.Logger
	core             *core.Core
}

func NewApp() *APP {
	return &APP{}
}

func (a *APP) Init() error {
	log.Printf("%v %v", config.GetName(), config.GetVersion())

	a.initLog()

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		return err
	}
	if initialPassword := database.TakeGeneratedInitialAdminPassword(); initialPassword != "" {
		fmt.Fprintf(os.Stderr, "\nIMPORTANT: first-start S-UI administrator credentials\nusername: %s\npassword: %s\nChange this password after the first login.\n\n", config.GetInitialAdminUsername(), initialPassword)
	}

	// Init Setting
	a.SettingService.GetAllSetting()

	a.core = core.NewCore()

	a.cronJob = cronjob.NewCronJob()
	a.webServer = web.NewServer()
	a.subServer = sub.NewServer()

	a.configService = service.NewConfigService(a.core)
	a.xrayXHTTPService = &service.XrayXHTTPService{}

	return nil
}

func (a *APP) Start() error {
	loc, err := a.SettingService.GetTimeLocation()
	if err != nil {
		return err
	}

	trafficAge, err := a.SettingService.GetTrafficAge()
	if err != nil {
		return err
	}

	err = a.cronJob.Start(loc, trafficAge)
	if err != nil {
		return err
	}

	err = a.webServer.Start()
	if err != nil {
		return err
	}

	err = a.subServer.Start()
	if err != nil {
		return err
	}

	err = a.configService.StartCore()
	if err != nil {
		if proxyCoresRequired() {
			return fmt.Errorf("start required sing-box core: %w", err)
		}
		logger.Error(err)
	}
	if err = a.xrayXHTTPService.StartIfEnabled(); err != nil {
		// The Linux package sets SUI_REQUIRE_PROXY_CORES only after a fresh
		// installation has verified the Reality target and default ports. Do
		// not let systemd report a healthy panel while either promised core is
		// unavailable.
		if proxyCoresRequired() {
			return fmt.Errorf("start required embedded Xray Reality runtime: %w", err)
		}
		logger.Warning("embedded Xray XHTTP runtime was not started: ", err)
	}

	return nil
}

func proxyCoresRequired() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SUI_REQUIRE_PROXY_CORES"))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func (a *APP) Stop() {
	a.cronJob.Stop()
	err := a.subServer.Stop()
	if err != nil {
		logger.Warning("stop Sub Server err:", err)
	}
	err = a.webServer.Stop()
	if err != nil {
		logger.Warning("stop Web Server err:", err)
	}
	err = a.xrayXHTTPService.Stop()
	if err != nil {
		logger.Warning("stop embedded Xray XHTTP runtime err:", err)
	}
	err = a.configService.StopCore()
	if err != nil {
		logger.Warning("stop Core err:", err)
	}
}

func (a *APP) initLog() {
	switch config.GetLogLevel() {
	case config.Debug:
		logger.InitLogger(logging.DEBUG)
	case config.Info:
		logger.InitLogger(logging.INFO)
	case config.Warn:
		logger.InitLogger(logging.WARNING)
	case config.Error:
		logger.InitLogger(logging.ERROR)
	default:
		log.Fatal("unknown log level:", config.GetLogLevel())
	}
}

func (a *APP) RestartApp() error {
	a.Stop()
	return a.Start()
}

func (a *APP) GetCore() *core.Core {
	return a.core
}
