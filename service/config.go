package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/yongwei9527-art/s-ui-go/core"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/util/common"
)

var (
	// LastUpdate is protected by lastUpdateMu. Keep all access behind
	// loadLastUpdate and publishLastUpdate so a committed change is never
	// observed concurrently with an unsynchronised write.
	LastUpdate            uint64
	lastUpdateInitialized bool
	lastUpdateMu          sync.RWMutex
	corePtr               *core.Core
	coreOperationMu       sync.Mutex
	lastSuccessfulConfig  json.RawMessage

	startCoreMu         sync.Mutex
	startCoreInProgress bool
	lastStartFailTime   time.Time
	startFailCount      int
	startCooldown       = 15 * time.Second
	maxStartCooldown    = 5 * time.Minute

	configRestartMu                sync.Mutex
	pendingConfigRestart           json.RawMessage
	pendingConfigRestartVersion    uint64
	configRestartWorkerRunning     bool
	configRestartSuspended         bool
	configRestartWorkerCancel      chan struct{}
	configRestartRetryNotification = make(chan struct{}, 1)
)

const (
	configRestartInitialBackoff = 100 * time.Millisecond
	configRestartMaximumBackoff = 30 * time.Second
)

var errPreviousCoreRestored = errors.New("previous core configuration restored; waiting for a corrected configuration")

func loadLastUpdate() uint64 {
	lastUpdateMu.RLock()
	defer lastUpdateMu.RUnlock()
	return LastUpdate
}

func loadLastUpdateState() (uint64, bool) {
	lastUpdateMu.RLock()
	defer lastUpdateMu.RUnlock()
	return LastUpdate, lastUpdateInitialized
}

func publishLastUpdate(revision uint64) {
	lastUpdateMu.Lock()
	if revision > LastUpdate {
		LastUpdate = revision
	}
	lastUpdateInitialized = true
	lastUpdateMu.Unlock()
}

func latestChangeRevision() (uint64, error) {
	var latest model.Changes
	result := database.GetDB().Order("id DESC").Limit(1).Find(&latest)
	if result.Error != nil {
		return 0, result.Error
	}
	return latest.Id, nil
}

func publishLatestChangeRevision() {
	revision, err := latestChangeRevision()
	if err != nil {
		logger.Warning("unable to load latest change revision: ", err)
		return
	}
	publishLastUpdate(revision)
}

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

	// restartConfigFn is a test seam; it still runs under the serialized Core lifecycle lock.
	restartConfigFn func(json.RawMessage) error
	// restartConfigRetryWait is a test seam for bounded retry backoff.
	restartConfigRetryWait func(time.Duration)
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
	return s.getConfigFromDB(data, database.GetDB())
}

func (s *ConfigService) getConfigFromDB(data string, db *gorm.DB) (*[]byte, error) {
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

	singboxConfig.Inbounds, err = s.InboundService.GetAllConfig(db)
	if err != nil {
		return nil, err
	}
	singboxConfig.Outbounds, err = s.OutboundService.GetAllConfig(db)
	if err != nil {
		return nil, err
	}
	singboxConfig.Services, err = s.ServicesService.GetAllConfig(db)
	if err != nil {
		return nil, err
	}
	singboxConfig.Endpoints, err = s.EndpointService.GetAllConfig(db)
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
	coreOperationMu.Lock()
	defer coreOperationMu.Unlock()
	if err := s.startCoreLocked(); err != nil {
		return err
	}
	// Only an explicit successful start reopens the asynchronous queue.
	configRestartMu.Lock()
	configRestartSuspended = false
	s.startConfigRestartWorkerLocked()
	configRestartMu.Unlock()
	return nil
}

func (s *ConfigService) startCoreLocked() error {
	if corePtr == nil {
		return common.NewError("core is not initialized")
	}
	if corePtr.IsRunning() {
		return nil
	}
	startCoreMu.Lock()
	if startCoreInProgress {
		startCoreMu.Unlock()
		return common.NewError("sing-box core start is already in progress")
	}
	cooldown := coreStartCooldown()
	if elapsed := time.Since(lastStartFailTime); elapsed < cooldown {
		remaining := cooldown - elapsed
		logger.Info("start core cooldown ", cooldown/time.Second, " seconds")
		startCoreMu.Unlock()
		return common.NewErrorf("sing-box core start is cooling down after a failure; retry in %s", remaining.Round(time.Second))
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
	lastStartFailTime = time.Time{}
	startCoreMu.Unlock()
	lastSuccessfulConfig = append(json.RawMessage(nil), (*rawConfig)...)
	logger.Info("sing-box started")
	return nil
}

func (s *ConfigService) RestartCore() error {
	coreOperationMu.Lock()
	defer coreOperationMu.Unlock()
	if corePtr == nil {
		return common.NewError("core is not initialized")
	}
	rawConfig, err := s.GetConfig("")
	if err != nil {
		return err
	}
	return s.restartCoreWithRawConfigLocked(*rawConfig)
}

func (s *ConfigService) restartCoreWithConfig(config json.RawMessage) error {
	coreOperationMu.Lock()
	defer coreOperationMu.Unlock()
	if s.restartConfigFn != nil {
		return s.restartConfigFn(config)
	}
	return s.restartCoreWithConfigLocked(config)
}

func (s *ConfigService) restartCoreWithConfigLocked(config json.RawMessage) error {
	if corePtr == nil {
		return common.NewError("core is not initialized")
	}
	// Build the pending configuration before stopping the running core. If the
	// stored configuration cannot be generated, the currently working core
	// remains available instead of being unnecessarily taken offline.
	rawConfig, err := s.GetConfig(string(config))
	if err != nil {
		logger.Error("restart sing-box err (get config):", err.Error())
		return err
	}
	return s.restartCoreWithRawConfigLocked(*rawConfig)
}

func (s *ConfigService) restartCoreWithRawConfigLocked(rawConfig json.RawMessage) error {
	startCoreMu.Lock()
	startCoreInProgress = true
	startCoreMu.Unlock()
	defer func() {
		startCoreMu.Lock()
		startCoreInProgress = false
		startCoreMu.Unlock()
	}()

	wasRunning := corePtr.IsRunning()
	rollbackConfig := append(json.RawMessage(nil), lastSuccessfulConfig...)
	if wasRunning {
		if err := corePtr.Stop(); err != nil {
			logger.Error("restart sing-box err (stop):", err.Error())
			return err
		}
	}
	if err := corePtr.Start(rawConfig); err != nil {
		logger.Error("restart sing-box err (start):", err.Error())
		if wasRunning && len(rollbackConfig) > 0 {
			if rollbackErr := corePtr.Start(rollbackConfig); rollbackErr != nil {
				logger.Error("restore previous sing-box config err:", rollbackErr.Error())
				return errors.Join(err, common.NewError(rollbackErr, "restore previous sing-box config"))
			}
			logger.Warning("new sing-box config failed; previous running config restored")
			return errors.Join(err, errPreviousCoreRestored)
		}
		return err
	}
	lastSuccessfulConfig = append(json.RawMessage(nil), rawConfig...)
	logger.Info("sing-box restarted with new config")
	return nil
}

// EnqueueCurrentConfigRestart schedules a coalesced, retrying restart using the
// latest committed database state. It replaces the removed per-component hot update API.
func (s *ConfigService) EnqueueCurrentConfigRestart() error {
	config, err := s.SettingService.GetNormalizedConfig()
	if err != nil {
		return err
	}
	s.enqueueConfigRestart(json.RawMessage(config))
	return nil
}

func (s *ConfigService) enqueueConfigRestart(config json.RawMessage) {
	configCopy := append(json.RawMessage(nil), config...)

	configRestartMu.Lock()
	pendingConfigRestart = configCopy
	pendingConfigRestartVersion++
	if configRestartWorkerRunning {
		configRestartMu.Unlock()
		select {
		case configRestartRetryNotification <- struct{}{}:
		default:
		}
		return
	}
	s.startConfigRestartWorkerLocked()
	configRestartMu.Unlock()
}

// configRestartMu must be held. A canceled generation is never reused, even
// when StartCore resumes the queue before the old goroutine has exited.
func (s *ConfigService) startConfigRestartWorkerLocked() {
	if configRestartSuspended || configRestartWorkerRunning || len(pendingConfigRestart) == 0 {
		return
	}
	cancel := make(chan struct{})
	configRestartWorkerCancel = cancel
	configRestartWorkerRunning = true
	go s.processConfigRestartQueue(cancel)
}

func restartWorkerCanceled(cancel <-chan struct{}) bool {
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}

var errConfigRestartSuperseded = errors.New("queued core restart canceled or superseded")

func (s *ConfigService) restartQueuedConfig(config json.RawMessage, version uint64, cancel <-chan struct{}) error {
	coreOperationMu.Lock()
	defer coreOperationMu.Unlock()
	// Lock order is always coreOperationMu -> configRestartMu. Never acquire
	// the lifecycle lock while holding the queue lock or wait for the worker.
	configRestartMu.Lock()
	stale := configRestartSuspended || restartWorkerCanceled(cancel) || pendingConfigRestartVersion != version
	configRestartMu.Unlock()
	if stale {
		return errConfigRestartSuperseded
	}
	if s.restartConfigFn != nil {
		return s.restartConfigFn(config)
	}
	return s.restartCoreWithConfigLocked(config)
}

func (s *ConfigService) processConfigRestartQueue(cancel chan struct{}) {
	defer func() {
		configRestartMu.Lock()
		configRestartWorkerRunning = false
		configRestartWorkerCancel = nil
		// A resume may have happened while the canceled worker was exiting.
		// Hand off the latest submission rather than abandoning it.
		s.startConfigRestartWorkerLocked()
		configRestartMu.Unlock()
	}()
	var retryCount int
	var retryVersion uint64

	for {
		configRestartMu.Lock()
		if restartWorkerCanceled(cancel) || configRestartSuspended || len(pendingConfigRestart) == 0 {
			configRestartMu.Unlock()
			return
		}
		config := append(json.RawMessage(nil), pendingConfigRestart...)
		version := pendingConfigRestartVersion
		configRestartMu.Unlock()

		if retryVersion != version {
			retryVersion = version
			retryCount = 0
		}
		drainConfigRestartRetryNotification()

		if err := s.restartQueuedConfig(config, version, cancel); err == nil {
			configRestartMu.Lock()
			if !restartWorkerCanceled(cancel) && pendingConfigRestartVersion == version {
				pendingConfigRestart = nil
			}
			configRestartMu.Unlock()
			retryCount = 0
			continue
		} else {
			if errors.Is(err, errConfigRestartSuperseded) {
				continue
			}
			logger.Error("restart sing-box err:", err.Error())
			if errors.Is(err, errPreviousCoreRestored) {
				configRestartMu.Lock()
				if !restartWorkerCanceled(cancel) && pendingConfigRestartVersion == version {
					pendingConfigRestart = nil
				}
				configRestartMu.Unlock()
				continue
			}
		}

		configRestartMu.Lock()
		if pendingConfigRestartVersion != version {
			configRestartMu.Unlock()
			continue
		}
		retryCount++
		backoff := configRestartBackoff(retryCount)
		configRestartMu.Unlock()

		s.waitForConfigRestartRetry(backoff, cancel)
	}
}

func configRestartBackoff(retry int) time.Duration {
	backoff := configRestartInitialBackoff
	for attempt := 1; attempt < retry && backoff < configRestartMaximumBackoff; attempt++ {
		backoff *= 2
	}
	if backoff > configRestartMaximumBackoff {
		return configRestartMaximumBackoff
	}
	return backoff
}

func drainConfigRestartRetryNotification() {
	select {
	case <-configRestartRetryNotification:
	default:
	}
}

func (s *ConfigService) waitForConfigRestartRetry(backoff time.Duration, cancel <-chan struct{}) {
	if s.restartConfigRetryWait != nil {
		s.restartConfigRetryWait(backoff)
		return
	}

	timer := time.NewTimer(backoff)
	defer timer.Stop()
	select {
	case <-cancel:
	case <-timer.C:
	case <-configRestartRetryNotification:
	}
}

func (s *ConfigService) StopCore() error {
	coreOperationMu.Lock()
	defer coreOperationMu.Unlock()
	// Disable queued starts before stopping the instance. In-flight restarts
	// have already released coreOperationMu; waiting workers recheck their
	// immutable cancellation signal after acquiring it. Keep the latest
	// submission for a subsequent successful StartCore, including submissions
	// made while stopped. Do not wait for a worker while holding this lock.
	configRestartMu.Lock()
	configRestartSuspended = true
	if configRestartWorkerCancel != nil && !restartWorkerCanceled(configRestartWorkerCancel) {
		close(configRestartWorkerCancel)
	}
	configRestartMu.Unlock()
	return s.stopCoreLocked()
}

func (s *ConfigService) stopCoreLocked() error {
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
	if corePtr == nil {
		return core.CheckOutboundResult{Error: "core not running"}
	}
	return corePtr.CheckOutbound(tag, link)
}

func (s *ConfigService) Save(obj string, act string, data json.RawMessage, initUsers string, loginUser string, hostname string) ([]string, error) {
	coreOperationMu.Lock()
	defer coreOperationMu.Unlock()
	var err error
	objs := []string{obj}
	var restartConfigAfterCommit json.RawMessage
	var restartCurrentConfigAfterCommit bool
	var currentConfigBase json.RawMessage
	var reloadXrayAfterCommit bool

	db := database.GetDB()
	tx := db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	switch obj {
	case "clients":
		// Xray XHTTP keeps a separate UUID list. Rebuild it after the database
		// commit so client disablement/deletion becomes effective immediately.
		reloadXrayAfterCommit = true
		var inboundIds []uint
		inboundIds, err = s.ClientService.Save(tx, act, data, hostname)
		if err == nil && len(inboundIds) > 0 {
			objs = append(objs, "inbounds")
			err = s.InboundService.RestartInbounds(tx, inboundIds)
			if err != nil {
				return nil, common.NewErrorf("failed to update users for inbounds: %v", err)
			}
		}
		restartCurrentConfigAfterCommit = true
	case "tls":
		err = s.TlsService.Save(tx, act, data, hostname)
		objs = append(objs, "clients", "inbounds")
		restartCurrentConfigAfterCommit = true
	case "inbounds":
		err = s.InboundService.Save(tx, act, data, initUsers, hostname)
		objs = append(objs, "clients")
		restartCurrentConfigAfterCommit = true
	case "outbounds":
		err = s.OutboundService.Save(tx, act, data)
		restartCurrentConfigAfterCommit = true
	case "services":
		err = s.ServicesService.Save(tx, act, data)
		restartCurrentConfigAfterCommit = true
	case "endpoints":
		err = s.EndpointService.Save(tx, act, data)
		restartCurrentConfigAfterCommit = true
	case "config":
		err = s.SettingService.SaveConfig(tx, data)
		if err != nil {
			return nil, err
		}
		restartConfigAfterCommit = append(json.RawMessage(nil), data...)
	case "settings":
		err = s.SettingService.Save(tx, data)
	default:
		return nil, common.NewError("unknown object: ", obj)
	}
	if err != nil {
		return nil, err
	}
	if restartCurrentConfigAfterCommit {
		config, configErr := s.SettingService.GetNormalizedConfig()
		if configErr != nil {
			return nil, common.NewError(configErr, "load current core config before commit")
		}
		currentConfigBase = json.RawMessage(config)
	}

	dt := time.Now().Unix()
	change := model.Changes{
		DateTime: dt,
		Actor:    loginUser,
		Key:      obj,
		Action:   act,
		Obj:      data,
	}
	err = tx.Create(&change).Error
	if err != nil {
		return nil, err
	}
	var previousConfig json.RawMessage
	wasRunning := corePtr != nil && corePtr.IsRunning()
	needsCoreUpdate := restartCurrentConfigAfterCommit || len(restartConfigAfterCommit) > 0
	if needsCoreUpdate && corePtr != nil {
		base := currentConfigBase
		if len(restartConfigAfterCommit) > 0 {
			base = restartConfigAfterCommit
		}
		rawConfig, configErr := s.getConfigFromDB(string(base), tx)
		if configErr != nil {
			return nil, configErr
		}
		previousConfig = append(json.RawMessage(nil), lastSuccessfulConfig...)
		if startErr := s.restartCoreWithRawConfigLocked(*rawConfig); startErr != nil {
			return nil, fmt.Errorf("configuration was not saved because core startup failed: %w", startErr)
		}
	}
	if err := tx.Commit().Error; err != nil {
		if needsCoreUpdate && corePtr != nil {
			if wasRunning && len(previousConfig) > 0 {
				if restoreErr := s.restartCoreWithRawConfigLocked(previousConfig); restoreErr != nil {
					return nil, errors.Join(err, restoreErr)
				}
			} else {
				_ = corePtr.Stop()
			}
		}
		return nil, err
	}
	committed = true
	publishLastUpdate(change.Id)

	if reloadXrayAfterCommit {
		go func() {
			if err := (&XrayXHTTPService{}).ReloadAfterClientChange(); err != nil {
				logger.Warning("unable to refresh Xray XHTTP clients: ", err)
			}
		}()
	}
	if !needsCoreUpdate && corePtr != nil && !corePtr.IsRunning() {
		if err := s.startCoreLocked(); err != nil {
			logger.Warning("configuration saved, but core startup failed: ", err)
		}
	}

	return objs, nil
}
func (s *ConfigService) CheckChanges(lu string) (bool, error) {
	if lu == "" {
		return true, nil
	}
	clientRevision, err := strconv.ParseUint(lu, 10, 64)
	if err != nil {
		return true, err
	}
	currentRevision, err := s.currentChangeRevision()
	if err != nil {
		return true, err
	}
	// Old browsers sent a Unix timestamp. It will always be much larger than a
	// change ID, so force one full reload to migrate them to the revision token.
	if clientRevision > currentRevision {
		return true, nil
	}
	return currentRevision > clientRevision, nil
}

// CurrentChangeRevision exposes the durable change-history ID used by API
// clients. A zero value is a valid revision for a newly initialized database.
func (s *ConfigService) CurrentChangeRevision() uint64 {
	revision, err := s.currentChangeRevision()
	if err != nil {
		logger.Warning("unable to read current change revision: ", err)
		return loadLastUpdate()
	}
	return revision
}

func (s *ConfigService) currentChangeRevision() (uint64, error) {
	if revision, initialized := loadLastUpdateState(); initialized {
		return revision, nil
	}
	revision, err := latestChangeRevision()
	if err != nil {
		return 0, err
	}
	publishLastUpdate(revision)
	return revision, nil
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
