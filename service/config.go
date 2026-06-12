package service

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/yongwei9527-art/s-ui-go/core"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/util/common"
)

var (
	LastUpdate          int64
	corePtr             *core.Core
	startCoreMu         sync.Mutex
	startCoreInProgress bool
	lastStartFailTime   time.Time
	startFailCount      int
	startCooldown       = 15 * time.Second
	maxStartCooldown    = 5 * time.Minute
)

func coreStartCooldown() time.Duration {
	cooldown := startCooldown
	if startFailCount > 0 {
		cooldown = startCooldown * time.Duration(1<<min(startFailCount-1, 5))
	}
	if cooldown > maxStartCooldown {
		return maxStartCooldown
	}
	return cooldown
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

type ConfigService struct {
	ClientService
	TlsService
	SettingService
	InboundService
	OutboundService
	ServicesService
	EndpointService
}

type SingBoxConfig struct {
	Log          json.RawMessage   `json:"log"`
	Dns          json.RawMessage   `json:"dns"`
	Ntp          json.RawMessage   `json:"ntp"`
	Inbounds     []json.RawMessage `json:"inbounds"`
	Outbounds    []json.RawMessage `json:"outbounds"`
	Services     []json.RawMessage `json:"services"`
	Endpoints    []json.RawMessage `json:"endpoints"`
	Route        json.RawMessage   `json:"route"`
	Experimental json.RawMessage   `json:"experimental"`
}

func NewConfigService(core *core.Core) *ConfigService {
	corePtr = core
	return &ConfigService{}
}

func (s *ConfigService) GetConfig(data string) (*[]byte, error) {
	var err error
	if len(data) == 0 {
		data, err = s.SettingService.GetNormalizedConfig()
		if err != nil {
			return nil, err
		}
	} else {
		normalized, err := s.SettingService.normalizeCoreConfig(json.RawMessage(data))
		if err != nil {
			return nil, err
		}
		data = string(normalized)
	}
	singboxConfig := SingBoxConfig{}
	err = json.Unmarshal([]byte(data), &singboxConfig)
	if err != nil {
		return nil, err
	}

	singboxConfig.Inbounds, err = s.InboundService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	singboxConfig.Outbounds, err = s.OutboundService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	singboxConfig.Services, err = s.ServicesService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	singboxConfig.Endpoints, err = s.EndpointService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	rawConfig, err := json.MarshalIndent(singboxConfig, "", "  ")
	if err != nil {
		return nil, err
	}
	return &rawConfig, nil
}

func (s *ConfigService) StartCore() error {
	if corePtr == nil {
		return common.NewError("core is not initialized")
	}
	if corePtr != nil && corePtr.IsRunning() {
		return nil
	}
	startCoreMu.Lock()
	if startCoreInProgress {
		startCoreMu.Unlock()
		return nil
	}
	cooldown := coreStartCooldown()
	if time.Since(lastStartFailTime) < cooldown {
		logger.Info("start core cooldown ", cooldown/time.Second, " seconds")
		startCoreMu.Unlock()
		return nil
	}
	startCoreInProgress = true
	startCoreMu.Unlock()
	defer func() {
		startCoreMu.Lock()
		startCoreInProgress = false
		startCoreMu.Unlock()
	}()

	logger.Info("starting core")
	rawConfig, err := s.GetConfig("")
	if err != nil {
		return err
	}
	err = corePtr.Start(*rawConfig)
	if err != nil {
		startCoreMu.Lock()
		lastStartFailTime = time.Now()
		startFailCount++
		startCoreMu.Unlock()
		logger.Error("start sing-box err:", err.Error())
		return err
	}
	startCoreMu.Lock()
	startFailCount = 0
	startCoreMu.Unlock()
	logger.Info("sing-box started")
	return nil
}

func (s *ConfigService) RestartCore() error {
	err := s.StopCore()
	if err != nil {
		return err
	}
	return s.StartCore()
}

func (s *ConfigService) restartCoreWithConfig(config json.RawMessage) error {
	if corePtr == nil {
		return common.NewError("core is not initialized")
	}
	startCoreMu.Lock()
	if startCoreInProgress {
		startCoreMu.Unlock()
		return nil
	}
	startCoreInProgress = true
	startCoreMu.Unlock()
	defer func() {
		startCoreMu.Lock()
		startCoreInProgress = false
		startCoreMu.Unlock()
	}()

	if corePtr != nil && corePtr.IsRunning() {
		if err := corePtr.Stop(); err != nil {
			logger.Error("restart sing-box err (stop):", err.Error())
			return err
		}
	}
	rawConfig, err := s.GetConfig(string(config))
	if err != nil {
		logger.Error("restart sing-box err (get config):", err.Error())
		return err
	}
	if err := corePtr.Start(*rawConfig); err != nil {
		logger.Error("restart sing-box err (start):", err.Error())
		return err
	}
	logger.Info("sing-box restarted with new config")
	return nil
}

func (s *ConfigService) StopCore() error {
	if corePtr == nil {
		return nil
	}
	err := corePtr.Stop()
	if err != nil {
		return err
	}
	logger.Info("sing-box stopped")
	return nil
}

func (s *ConfigService) CheckOutbound(tag string, link string) core.CheckOutboundResult {
	if tag == "" {
		return core.CheckOutboundResult{Error: "missing query parameter: tag"}
	}
	if corePtr == nil || !corePtr.IsRunning() {
		return core.CheckOutboundResult{Error: "core not running"}
	}
	return core.CheckOutbound(corePtr.GetCtx(), tag, link)
}

func (s *ConfigService) Save(obj string, act string, data json.RawMessage, initUsers string, loginUser string, hostname string) ([]string, error) {
	var err error
	var objs []string = []string{obj}
	var restartConfigAfterCommit json.RawMessage
	var restartCurrentConfigAfterCommit bool

	db := database.GetDB()
	tx := db.Begin()
	defer func() {
		if err == nil {
			tx.Commit()
			if len(restartConfigAfterCommit) > 0 {
				go func(configData json.RawMessage) { _ = s.restartCoreWithConfig(configData) }(restartConfigAfterCommit)
			} else if restartCurrentConfigAfterCommit {
				go func() {
					if corePtr != nil && corePtr.IsRunning() {
						_ = s.RestartCore()
					} else {
						_ = s.StartCore()
					}
				}()
			} else if corePtr == nil || !corePtr.IsRunning() {
				s.StartCore()
			}
		} else {
			tx.Rollback()
		}
	}()

	switch obj {
	case "clients":
		var inboundIds []uint
		inboundIds, err = s.ClientService.Save(tx, act, data, hostname)
		if err == nil && len(inboundIds) > 0 {
			objs = append(objs, "inbounds")
			err = s.InboundService.RestartInbounds(tx, inboundIds)
			if err != nil {
				return nil, common.NewErrorf("failed to update users for inbounds: %v", err)
			}
		}
	case "tls":
		err = s.TlsService.Save(tx, act, data, hostname)
		objs = append(objs, "clients", "inbounds")
	case "inbounds":
		err = s.InboundService.Save(tx, act, data, initUsers, hostname)
		objs = append(objs, "clients")
	case "outbounds":
		err = s.OutboundService.Save(tx, act, data)
	case "services":
		err = s.ServicesService.Save(tx, act, data)
	case "endpoints":
		err = s.EndpointService.Save(tx, act, data)
	case "config":
		err = s.SettingService.SaveConfig(tx, data)
		if err != nil {
			return nil, err
		}
		restartConfigAfterCommit = make(json.RawMessage, len(data))
		copy(restartConfigAfterCommit, data)
	case "settings":
		restartCurrentConfigAfterCommit = settingsChangeDNSLeakGuardMode(data)
		err = s.SettingService.Save(tx, data)
	default:
		return nil, common.NewError("unknown object: ", obj)
	}
	if err != nil {
		return nil, err
	}

	dt := time.Now().Unix()
	err = tx.Create(&model.Changes{
		DateTime: dt,
		Actor:    loginUser,
		Key:      obj,
		Action:   act,
		Obj:      data,
	}).Error
	if err != nil {
		return nil, err
	}

	LastUpdate = time.Now().Unix()

	return objs, nil
}

func settingsChangeDNSLeakGuardMode(data json.RawMessage) bool {
	var settings map[string]string
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}
	_, ok := settings["dnsLeakGuardMode"]
	return ok
}

func (s *ConfigService) CheckChanges(lu string) (bool, error) {
	if lu == "" {
		return true, nil
	}
	intLu, err := strconv.ParseInt(lu, 10, 64)
	if err != nil {
		return true, err
	}
	if LastUpdate == 0 {
		db := database.GetDB()
		var count int64
		err := db.Model(model.Changes{}).Where("date_time > ?", intLu).Count(&count).Error
		if err == nil {
			LastUpdate = time.Now().Unix()
		}
		return count > 0, err
	} else {
		return LastUpdate > intLu, nil
	}
}

func (s *ConfigService) GetChanges(actor string, chngKey string, count string) []model.Changes {
	c, _ := strconv.Atoi(count)
	if c <= 0 {
		c = 20
	}
	query := database.GetDB().Model(model.Changes{})
	if len(actor) > 0 {
		query = query.Where("actor = ?", actor)
	}
	if len(chngKey) > 0 {
		query = query.Where("key = ?", chngKey)
	}
	var chngs []model.Changes
	err := query.Order("`id` desc").Limit(c).Scan(&chngs).Error
	if err != nil {
		logger.Warning(err)
	}
	return chngs
}
