package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/core"
	"github.com/yongwei9527-art/s-ui-go/cronjob"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/service"
	"github.com/yongwei9527-art/s-ui-go/sub"
	"github.com/yongwei9527-art/s-ui-go/web"

	"github.com/op/go-logging"
)

type appLifecycleHooks struct {
	getTimeLocation       func() (*time.Location, error)
	getTrafficAge         func() (int, error)
	startCron             func(*time.Location, int) error
	stopCron              func()
	startWeb              func() error
	stopWeb               func() error
	startSub              func() error
	stopSub               func() error
	startCore             func() error
	stopCore              func() error
	hasEnabledXrayInbound func() (bool, error)
	startXray             func() error
	stopXray              func() error
}

type startedComponents struct {
	cron bool
	web  bool
	sub  bool
	core bool
	xray bool
}

type APP struct {
	service.SettingService
	configService    *service.ConfigService
	xrayXHTTPService *service.XrayXHTTPService
	webServer        *web.Server
	subServer        *sub.Server
	cronJob          *cronjob.CronJob
	logger           *logging.Logger
	core             *core.Core
	lifecycle        appLifecycleHooks
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
	if _, err := a.SettingService.GetAllSetting(); err != nil {
		return err
	}

	a.core = core.NewCore()

	a.cronJob = cronjob.NewCronJob()
	a.webServer = web.NewServer()
	a.subServer = sub.NewServer()

	a.configService = service.NewConfigService(a.core)
	a.xrayXHTTPService = &service.XrayXHTTPService{}

	return nil
}

func (a *APP) Start() error {
	loc, err := a.getTimeLocation()
	if err != nil {
		return err
	}

	trafficAge, err := a.getTrafficAge()
	if err != nil {
		return err
	}

	started := startedComponents{}
	if err = a.startCron(loc, trafficAge); err != nil {
		return err
	}
	started.cron = true

	if err = a.startWeb(); err != nil {
		a.stopStarted(started)
		return err
	}
	started.web = true

	if err = a.startSub(); err != nil {
		a.stopStarted(started)
		return err
	}
	started.sub = true

	if err = a.startCore(); err != nil {
		if proxyCoresRequired() {
			a.stopStarted(started)
			return fmt.Errorf("start required sing-box core: %w", err)
		}
		logger.Error(err)
	} else {
		started.core = true
	}

	enabledXray, err := a.hasEnabledXrayInbound()
	if err != nil {
		a.stopStarted(started)
		return fmt.Errorf("check enabled embedded Xray Reality inbound: %w", err)
	}
	if enabledXray {
		if err = a.startXray(); err != nil {
			a.stopStarted(started)
			return fmt.Errorf("start embedded Xray XHTTP runtime: %w", err)
		}
		started.xray = true
	}

	return nil
}

// stopStarted first drains background jobs, then releases successfully started
// servers and cores in reverse order. A cron job must not restart a core while
// failed startup cleanup is closing it.
func (a *APP) stopStarted(started startedComponents) {
	if started.cron {
		a.stopCron()
	}
	if started.xray {
		if err := a.stopXray(); err != nil {
			logger.Warning("stop embedded Xray XHTTP runtime err:", err)
		}
	}
	if started.core {
		if err := a.stopCore(); err != nil {
			logger.Warning("stop Core err:", err)
		}
	}
	if started.sub {
		if err := a.stopSub(); err != nil {
			logger.Warning("stop Sub Server err:", err)
		}
	}
	if started.web {
		if err := a.stopWeb(); err != nil {
			logger.Warning("stop Web Server err:", err)
		}
	}
}

func (a *APP) getTimeLocation() (*time.Location, error) {
	if a.lifecycle.getTimeLocation != nil {
		return a.lifecycle.getTimeLocation()
	}
	return a.SettingService.GetTimeLocation()
}

func (a *APP) getTrafficAge() (int, error) {
	if a.lifecycle.getTrafficAge != nil {
		return a.lifecycle.getTrafficAge()
	}
	return a.SettingService.GetTrafficAge()
}

func (a *APP) startCron(loc *time.Location, trafficAge int) error {
	if a.lifecycle.startCron != nil {
		return a.lifecycle.startCron(loc, trafficAge)
	}
	return a.cronJob.Start(loc, trafficAge)
}

func (a *APP) stopCron() {
	if a.lifecycle.stopCron != nil {
		a.lifecycle.stopCron()
		return
	}
	a.cronJob.Stop()
}

func (a *APP) startWeb() error {
	if a.lifecycle.startWeb != nil {
		return a.lifecycle.startWeb()
	}
	return a.webServer.Start()
}

func (a *APP) stopWeb() error {
	if a.lifecycle.stopWeb != nil {
		return a.lifecycle.stopWeb()
	}
	return a.webServer.Stop()
}

func (a *APP) startSub() error {
	if a.lifecycle.startSub != nil {
		return a.lifecycle.startSub()
	}
	return a.subServer.Start()
}

func (a *APP) stopSub() error {
	if a.lifecycle.stopSub != nil {
		return a.lifecycle.stopSub()
	}
	return a.subServer.Stop()
}

func (a *APP) startCore() error {
	if a.lifecycle.startCore != nil {
		return a.lifecycle.startCore()
	}
	return a.configService.StartCore()
}

func (a *APP) stopCore() error {
	if a.lifecycle.stopCore != nil {
		return a.lifecycle.stopCore()
	}
	return a.configService.StopCore()
}

func (a *APP) hasEnabledXrayInbound() (bool, error) {
	if a.lifecycle.hasEnabledXrayInbound != nil {
		return a.lifecycle.hasEnabledXrayInbound()
	}
	var count int64
	if err := database.GetDB().Model(&model.XrayXHTTPInbound{}).Where("enable = ?", true).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (a *APP) startXray() error {
	if a.lifecycle.startXray != nil {
		return a.lifecycle.startXray()
	}
	return a.xrayXHTTPService.StartIfEnabled()
}

func (a *APP) stopXray() error {
	if a.lifecycle.stopXray != nil {
		return a.lifecycle.stopXray()
	}
	return a.xrayXHTTPService.Stop()
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
	a.stopCron()
	if err := a.stopSub(); err != nil {
		logger.Warning("stop Sub Server err:", err)
	}
	if err := a.stopWeb(); err != nil {
		logger.Warning("stop Web Server err:", err)
	}
	if err := a.stopXray(); err != nil {
		logger.Warning("stop embedded Xray XHTTP runtime err:", err)
	}
	if err := a.stopCore(); err != nil {
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
	if err := a.Start(); err != nil {
		// Start cleans up its own partial attempt. Retry once so a transient
		// restart failure does not leave the process permanently offline.
		if rollbackErr := a.Start(); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("restore application after failed reload: %w", rollbackErr))
		}
		return nil
	}
	return nil
}

func (a *APP) GetCore() *core.Core {
	return a.core
}
