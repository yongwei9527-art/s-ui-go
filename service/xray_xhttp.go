package service

import (
	"bytes"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/internal/xrayruntime"
	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/util/common"
	"gorm.io/gorm"
)

const xrayXHTTPConfigName = "xray-xhttp.json"

type XrayXHTTPSaveRequest struct {
	Inbound   model.XrayXHTTPInbound `json:"inbound"`
	ClientIDs []uint                 `json:"clientIds"`
}

type xrayXHTTPRuntime struct {
	mu              sync.Mutex
	server          *xrayruntime.Server
	active          []byte
	lastError       string
	lastExitAt      int64
	reloadSuspended bool
}

var xrayXHTTPProcess xrayXHTTPRuntime

type XrayXHTTPService struct{}

func (s *XrayXHTTPService) GetAll() ([]model.XrayXHTTPInbound, error) {
	var inbounds []model.XrayXHTTPInbound
	if err := database.GetDB().Order("id").Find(&inbounds).Error; err != nil {
		return nil, err
	}
	return inbounds, nil
}

func (s *XrayXHTTPService) GetClientIDs(inboundID uint) ([]uint, error) {
	var rows []model.XrayXHTTPClient
	if err := database.GetDB().Where("inbound_id = ?", inboundID).Order("id").Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ClientID)
	}
	return ids, nil
}

func (s *XrayXHTTPService) Status() map[string]interface{} {
	xrayXHTTPProcess.mu.Lock()
	defer xrayXHTTPProcess.mu.Unlock()
	status := map[string]interface{}{
		"running":          xrayXHTTPProcess.server != nil,
		"engine":           "embedded",
		"version":          xrayruntime.Version(),
		"sourceIntegrated": true,
		"configured":       true,
		"configPath":       s.configPath(),
	}
	if xrayXHTTPProcess.lastError != "" {
		status["runtimeError"] = xrayXHTTPProcess.lastError
	}
	if xrayXHTTPProcess.lastExitAt != 0 {
		status["lastExitAt"] = xrayXHTTPProcess.lastExitAt
	}
	return status
}

func (s *XrayXHTTPService) GetData() (map[string]interface{}, error) {
	inbounds, err := s.GetAll()
	if err != nil {
		return nil, err
	}
	assignments := make(map[uint][]uint, len(inbounds))
	for _, inb := range inbounds {
		ids, err := s.GetClientIDs(inb.Id)
		if err != nil {
			return nil, err
		}
		assignments[inb.Id] = ids
	}
	// Never return Reality private keys to the browser or token API. Existing
	// keys are retained when an edit submits an empty privateKey field.
	views := make([]model.XrayXHTTPInbound, len(inbounds))
	copy(views, inbounds)
	for i := range views {
		views[i].PrivateKey = ""
	}
	return map[string]interface{}{"inbounds": views, "clientIds": assignments, "status": s.Status()}, nil
}

func (s *XrayXHTTPService) Save(req XrayXHTTPSaveRequest) error {
	xrayXHTTPProcess.mu.Lock()
	defer xrayXHTTPProcess.mu.Unlock()

	inb := req.Inbound
	var previousInbound *model.XrayXHTTPInbound
	var previousClients []model.XrayXHTTPClient
	// The read API intentionally redacts the private key. Preserve it for edits
	// unless the administrator explicitly supplies a replacement.
	if inb.Id != 0 {
		var existing model.XrayXHTTPInbound
		if err := database.GetDB().First(&existing, inb.Id).Error; err != nil {
			return err
		}
		previousInbound = &existing
		if strings.TrimSpace(inb.PrivateKey) == "" {
			inb.PrivateKey = existing.PrivateKey
		}
		if err := database.GetDB().Where("inbound_id = ?", inb.Id).Order("id").Find(&previousClients).Error; err != nil {
			return err
		}
	}
	normalizeXrayXHTTPInbound(&inb)
	if err := validateXrayXHTTPInbound(inb); err != nil {
		return err
	}
	if err := s.validateClientIDs(req.ClientIDs, inb.Enable); err != nil {
		return err
	}
	if err := s.validateListenConflicts(inb); err != nil {
		return err
	}

	db := database.GetDB()
	var change model.Changes
	err := db.Transaction(func(tx *gorm.DB) error {
		if inb.Id == 0 {
			if err := tx.Create(&inb).Error; err != nil {
				return err
			}
		} else {
			var exists model.XrayXHTTPInbound
			if err := tx.First(&exists, inb.Id).Error; err != nil {
				return err
			}
			if err := tx.Save(&inb).Error; err != nil {
				return err
			}
		}
		var existingClients []model.XrayXHTTPClient
		if err := tx.Where("inbound_id = ?", inb.Id).Find(&existingClients).Error; err != nil {
			return err
		}
		previousUUIDs := make(map[uint]string, len(existingClients))
		for _, client := range existingClients {
			previousUUIDs[client.ClientID] = client.UUID
		}
		if err := tx.Where("inbound_id = ?", inb.Id).Delete(&model.XrayXHTTPClient{}).Error; err != nil {
			return err
		}
		for _, clientID := range uniqueUint(req.ClientIDs) {
			clientUUID := previousUUIDs[clientID]
			if clientUUID == "" {
				clientUUID = uuid.NewString()
			}
			row := model.XrayXHTTPClient{InboundID: inb.Id, ClientID: clientID, Enable: true, UUID: clientUUID}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		change = model.Changes{DateTime: time.Now().Unix(), Actor: "system", Key: "xray-xhttp", Action: "save", Obj: json.RawMessage(fmt.Sprintf(`{"tag":%q}`, inb.Tag))}
		return tx.Create(&change).Error
	})
	if err != nil {
		return err
	}
	publishLastUpdate(change.Id)
	if err := s.restartExplicitLocked(); err != nil {
		restoreErr := restoreXrayInboundDatabase(inb.Id, previousInbound, previousClients, "save")
		if restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore Xray database after failed save: %w", restoreErr))
		}
		return err
	}
	return nil
}

func (s *XrayXHTTPService) Delete(id uint) error {
	if id == 0 {
		return common.NewError("invalid XHTTP inbound id")
	}
	xrayXHTTPProcess.mu.Lock()
	defer xrayXHTTPProcess.mu.Unlock()

	var previousInbound model.XrayXHTTPInbound
	if err := database.GetDB().First(&previousInbound, id).Error; err != nil {
		return err
	}
	var previousClients []model.XrayXHTTPClient
	if err := database.GetDB().Where("inbound_id = ?", id).Order("id").Find(&previousClients).Error; err != nil {
		return err
	}
	var change model.Changes
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("inbound_id = ?", id).Delete(&model.XrayXHTTPClient{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&model.XrayXHTTPInbound{}, id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return common.NewError("XHTTP inbound not found")
		}
		change = model.Changes{DateTime: time.Now().Unix(), Actor: "system", Key: "xray-xhttp", Action: "delete", Obj: json.RawMessage(fmt.Sprintf(`{"id":%d}`, id))}
		return tx.Create(&change).Error
	})
	if err != nil {
		return err
	}
	publishLastUpdate(change.Id)
	if err := s.restartExplicitLocked(); err != nil {
		restoreErr := restoreXrayInboundDatabase(id, &previousInbound, previousClients, "delete")
		if restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore Xray database after failed delete: %w", restoreErr))
		}
		return err
	}
	return nil
}

func restoreXrayInboundDatabase(id uint, inbound *model.XrayXHTTPInbound, clients []model.XrayXHTTPClient, failedAction string) error {
	var change model.Changes
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("inbound_id = ?", id).Delete(&model.XrayXHTTPClient{}).Error; err != nil {
			return err
		}
		if inbound == nil {
			if err := tx.Delete(&model.XrayXHTTPInbound{}, id).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Save(inbound).Error; err != nil {
				return err
			}
			if len(clients) > 0 {
				if err := tx.Create(&clients).Error; err != nil {
					return err
				}
			}
		}
		obj := json.RawMessage(fmt.Sprintf(`{"id":%d,"failedAction":%q}`, id, failedAction))
		change = model.Changes{DateTime: time.Now().Unix(), Actor: "system", Key: "xray-xhttp", Action: "rollback", Obj: obj}
		return tx.Create(&change).Error
	})
	if err == nil {
		publishLastUpdate(change.Id)
	}
	return err
}

func (s *XrayXHTTPService) Restart() error {
	xrayXHTTPProcess.mu.Lock()
	defer xrayXHTTPProcess.mu.Unlock()
	return s.restartExplicitLocked()
}

// restartExplicitLocked resumes client-triggered reloads only after an explicit
// lifecycle/admin start succeeds. xrayXHTTPProcess.mu must be held.
func (s *XrayXHTTPService) restartExplicitLocked() error {
	if err := s.restartLocked(); err != nil {
		return err
	}
	xrayXHTTPProcess.reloadSuspended = false
	return nil
}

func (s *XrayXHTTPService) StartIfEnabled() error {
	xrayXHTTPProcess.mu.Lock()
	defer xrayXHTTPProcess.mu.Unlock()
	var count int64
	if err := database.GetDB().Model(&model.XrayXHTTPInbound{}).Where("enable = ?", true).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		xrayXHTTPProcess.reloadSuspended = false
		return nil
	}
	return s.restartExplicitLocked()
}

// ReloadAfterClientChange uses the current database snapshot, never a cached
// config. A queued reload must not reopen the runtime after APP.Stop.
func (s *XrayXHTTPService) ReloadAfterClientChange() error {
	xrayXHTTPProcess.mu.Lock()
	defer xrayXHTTPProcess.mu.Unlock()
	if xrayXHTTPProcess.reloadSuspended {
		return nil
	}
	var count int64
	if err := database.GetDB().Model(&model.XrayXHTTPInbound{}).Where("enable = ?", true).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	return s.restartLocked()
}

func (s *XrayXHTTPService) Stop() error {
	xrayXHTTPProcess.mu.Lock()
	defer xrayXHTTPProcess.mu.Unlock()
	xrayXHTTPProcess.reloadSuspended = true
	return stopXrayLocked()
}

func (s *XrayXHTTPService) restartLocked() error {
	configBytes, enabled, err := s.BuildConfig()
	if err != nil {
		recordXrayRuntimeError(err)
		return err
	}
	if !enabled {
		if err := stopXrayLocked(); err != nil {
			return err
		}
		// The rendered file contains a Reality private key. Once all Xray
		// inbounds are disabled, there is no reason to retain it on disk.
		if err := os.Remove(s.configPath()); err != nil && !os.IsNotExist(err) {
			recordXrayRuntimeError(err)
			return fmt.Errorf("remove inactive embedded Xray config: %w", err)
		}
		return nil
	}
	if err := xrayruntime.Validate(configBytes); err != nil {
		err = common.NewErrorf("embedded Xray config test failed: %v", err)
		recordXrayRuntimeError(err)
		return err
	}
	if err := os.MkdirAll(config.GetDBFolderPath(), 0750); err != nil {
		recordXrayRuntimeError(err)
		return err
	}
	previousConfig := append([]byte(nil), xrayXHTTPProcess.active...)
	hadPrevious := xrayXHTTPProcess.server != nil
	if err := stopXrayLocked(); err != nil {
		return err
	}
	candidate, err := xrayruntime.New(configBytes)
	if err != nil {
		startErr := common.NewErrorf("create embedded Xray instance: %v", err)
		return s.failAndRestoreLocked(startErr, previousConfig, hadPrevious)
	}
	candidateOwned := true
	defer func() {
		if candidateOwned {
			_ = candidate.Close()
		}
	}()
	if err := candidate.Start(); err != nil {
		closeErr := candidate.Close()
		if closeErr == nil {
			candidateOwned = false
		} else {
			err = errors.Join(err, closeErr)
		}
		return s.failAndRestoreLocked(err, previousConfig, hadPrevious)
	}
	configPath := s.configPath()
	if err := writePrivateFileAtomic(configPath, configBytes); err != nil {
		closeErr := candidate.Close()
		if closeErr == nil {
			candidateOwned = false
		} else {
			err = errors.Join(err, closeErr)
		}
		return s.failAndRestoreLocked(fmt.Errorf("write embedded Xray config: %w", err), previousConfig, hadPrevious)
	}
	xrayXHTTPProcess.server = candidate
	xrayXHTTPProcess.active = append([]byte(nil), configBytes...)
	xrayXHTTPProcess.lastError = ""
	xrayXHTTPProcess.lastExitAt = 0
	candidateOwned = false
	logger.Info("Embedded Xray runtime started on configured Reality inbound(s)")
	return nil
}

func (s *XrayXHTTPService) failAndRestoreLocked(cause error, previousConfig []byte, hadPrevious bool) error {
	recordXrayRuntimeError(cause)
	if !hadPrevious || len(previousConfig) == 0 {
		return cause
	}
	previous, err := xrayruntime.New(previousConfig)
	if err != nil {
		return errors.Join(cause, fmt.Errorf("restore previous embedded Xray instance: %w", err))
	}
	if err := previous.Start(); err != nil {
		_ = previous.Close()
		return errors.Join(cause, fmt.Errorf("restart previous embedded Xray instance: %w", err))
	}
	xrayXHTTPProcess.server = previous
	xrayXHTTPProcess.active = append([]byte(nil), previousConfig...)
	if err := writePrivateFileAtomic(s.configPath(), previousConfig); err != nil {
		return errors.Join(cause, fmt.Errorf("restore previous embedded Xray config file: %w", err))
	}
	logger.Warning("Embedded Xray update failed; the previous runtime configuration was restored: ", cause)
	return cause
}

func stopXrayLocked() error {
	if xrayXHTTPProcess.server == nil {
		return nil
	}
	server := xrayXHTTPProcess.server
	if err := server.Close(); err != nil {
		recordXrayRuntimeError(err)
		return err
	}
	xrayXHTTPProcess.server = nil
	xrayXHTTPProcess.active = nil
	return nil
}

func recordXrayRuntimeError(err error) {
	if err == nil {
		return
	}
	xrayXHTTPProcess.lastError = err.Error()
	xrayXHTTPProcess.lastExitAt = time.Now().Unix()
}

func writePrivateFileAtomic(path string, data []byte) (err error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".xray-xhttp-*.tmp")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer func() {
		_ = file.Close()
		_ = os.Remove(tempPath)
	}()
	if err = file.Chmod(0600); err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(tempPath, path); err != nil {
		// Unix replaces atomically. Windows does not replace an existing file,
		// so use a narrow fallback there while preserving the private mode.
		if runtime.GOOS != "windows" {
			return err
		}
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return err
		}
		if err = os.Rename(tempPath, path); err != nil {
			return err
		}
	}
	return os.Chmod(path, 0600)
}

func (s *XrayXHTTPService) configPath() string {
	return filepath.Join(config.GetDBFolderPath(), xrayXHTTPConfigName)
}

func (s *XrayXHTTPService) BuildConfig() ([]byte, bool, error) {
	var data []byte
	var enabled bool
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		var err error
		data, enabled, err = s.buildConfigSnapshot(tx)
		return err
	})
	return data, enabled, err
}

func (s *XrayXHTTPService) buildConfigSnapshot(tx *gorm.DB) ([]byte, bool, error) {
	var inbounds []model.XrayXHTTPInbound
	if err := tx.Where("enable = ?", true).Order("id").Find(&inbounds).Error; err != nil {
		return nil, false, err
	}
	if len(inbounds) == 0 {
		return []byte(`{"inbounds":[],"outbounds":[{"protocol":"freedom","tag":"direct"}]}`), false, nil
	}

	xrayInbounds := make([]map[string]interface{}, 0, len(inbounds))
	for _, inb := range inbounds {
		// Existing XHTTP-only records predate the transport field. Normalize them
		// while rendering so an upgrade never turns a valid saved configuration
		// into an invalid embedded Xray configuration.
		normalizeXrayXHTTPInbound(&inb)
		if err := validateXrayXHTTPInbound(inb); err != nil {
			return nil, false, err
		}
		var rows []struct {
			UUID string
			Name string
		}
		err := tx.Table("xray_x_http_clients").Select("xray_x_http_clients.uuid, clients.name").Joins("join clients on clients.id = xray_x_http_clients.client_id").Where("xray_x_http_clients.inbound_id = ? and xray_x_http_clients.enable = ? and clients.enable = ?", inb.Id, true, true).Scan(&rows).Error
		if err != nil {
			return nil, false, err
		}
		clients := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			clients = append(clients, xrayClientConfig(inb.Transport, row.UUID, row.Name))
		}
		// A client can be disabled, expired, or deleted after this inbound was
		// created. Omit an empty inbound so a subsequent reload revokes access
		// instead of retaining stale UUIDs in the running Xray instance.
		if len(clients) == 0 {
			continue
		}
		xrayInbounds = append(xrayInbounds, xrayInboundConfig(inb, clients))
	}
	if len(xrayInbounds) == 0 {
		return []byte(`{"inbounds":[],"outbounds":[{"protocol":"freedom","tag":"direct"}]}`), false, nil
	}
	cfg := map[string]interface{}{
		"log":       map[string]string{"loglevel": "warning"},
		"inbounds":  xrayInbounds,
		"outbounds": []map[string]string{{"protocol": "freedom", "tag": "direct"}, {"protocol": "blackhole", "tag": "blocked"}},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	return data, true, err
}

func xrayInboundConfig(inb model.XrayXHTTPInbound, clients []map[string]string) map[string]interface{} {
	transport := strings.ToLower(strings.TrimSpace(inb.Transport))
	if transport == "" {
		transport = "xhttp"
	}
	streamSettings := map[string]interface{}{
		"network": transport, "security": "reality",
		"realitySettings": map[string]interface{}{"show": false, "dest": inb.Dest, "xver": 0, "serverNames": []string{inb.ServerName}, "privateKey": inb.PrivateKey, "shortIds": []string{inb.ShortID}},
	}
	if transport == "xhttp" {
		streamSettings["xhttpSettings"] = map[string]string{"path": inb.Path, "mode": inb.Mode}
	}
	return map[string]interface{}{
		"tag": inb.Tag, "listen": inb.Listen, "port": inb.Port, "protocol": "vless",
		"settings":       map[string]interface{}{"decryption": "none", "clients": clients},
		"streamSettings": streamSettings,
	}
}

// xrayClientConfig applies VLESS flow settings required by the selected
// transport. XTLS Vision is valid only for the direct TCP Reality template.
func xrayClientConfig(transport, id, email string) map[string]string {
	client := map[string]string{"id": id, "email": email}
	if strings.EqualFold(strings.TrimSpace(transport), "tcp") {
		client["flow"] = "xtls-rprx-vision"
	}
	return client
}

type xrayClientSubscriptionRow struct {
	model.XrayXHTTPInbound
	UUID string
}

func (s *XrayXHTTPService) subscriptionRowsForClient(clientID uint) ([]xrayClientSubscriptionRow, error) {
	var rows []xrayClientSubscriptionRow
	err := database.GetDB().
		Table("xray_x_http_inbounds").
		Select("xray_x_http_inbounds.*, xray_x_http_clients.uuid").
		Joins("join xray_x_http_clients on xray_x_http_clients.inbound_id = xray_x_http_inbounds.id").
		Where("xray_x_http_inbounds.enable = ? and xray_x_http_clients.enable = ? and xray_x_http_clients.client_id = ?", true, true, clientID).
		Order("xray_x_http_inbounds.id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *XrayXHTTPService) LinksForClient(clientID uint, hostname string) ([]string, error) {
	rows, err := s.subscriptionRowsForClient(clientID)
	if err != nil {
		return nil, err
	}
	links := make([]string, 0, len(rows))
	for _, row := range rows {
		links = append(links, xrayVLESSLink(row.XrayXHTTPInbound, row.UUID, hostname))
	}
	return links, nil
}

// SubscriptionOutboundsForClient returns the same enabled Xray nodes as the
// standard VLESS URI subscription, but in the neutral outbound shape consumed
// by S-UI's Mihomo/Clash converter. It keeps protocol generation in one place
// and avoids parsing our own URI output back into a lossy structure.
func (s *XrayXHTTPService) SubscriptionOutboundsForClient(clientID uint, hostname string) ([]map[string]interface{}, error) {
	rows, err := s.subscriptionRowsForClient(clientID)
	if err != nil {
		return nil, err
	}
	outbounds := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		outbounds = append(outbounds, xraySubscriptionOutbound(row.XrayXHTTPInbound, row.UUID, hostname))
	}
	return outbounds, nil
}

func xraySubscriptionOutbound(inb model.XrayXHTTPInbound, uuid, hostname string) map[string]interface{} {
	normalizeXrayXHTTPInbound(&inb)
	host := strings.Trim(strings.TrimSpace(hostname), "[]")
	if host == "" {
		host = inb.ServerName
	}
	outbound := map[string]interface{}{
		"type":        "vless",
		"tag":         inb.Tag,
		"server":      host,
		"server_port": inb.Port,
		"uuid":        uuid,
		// Mihomo defaults proxy UDP support to false. The embedded Reality
		// outbounds support UDP, so expose that capability explicitly.
		"udp": true,
		"tls": map[string]interface{}{
			"enabled":     true,
			"server_name": inb.ServerName,
			"reality": map[string]interface{}{
				"enabled":    true,
				"public_key": inb.PublicKey,
				"short_id":   inb.ShortID,
			},
			"utls": map[string]interface{}{
				"enabled":     true,
				"fingerprint": inb.Fingerprint,
			},
		},
	}
	if inb.Transport == "tcp" {
		outbound["flow"] = "xtls-rprx-vision"
	} else {
		outbound["transport"] = map[string]interface{}{
			"type": "xhttp",
			"path": inb.Path,
			"mode": inb.Mode,
		}
	}
	return outbound
}

func xrayVLESSLink(inb model.XrayXHTTPInbound, uuid, hostname string) string {
	normalizeXrayXHTTPInbound(&inb)
	host := strings.TrimSpace(hostname)
	if host == "" {
		host = inb.ServerName
	}
	host = strings.Trim(host, "[]")
	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("security", "reality")
	q.Set("sni", inb.ServerName)
	q.Set("fp", inb.Fingerprint)
	q.Set("pbk", inb.PublicKey)
	q.Set("sid", inb.ShortID)
	transport := inb.Transport
	q.Set("type", transport)
	if transport == "tcp" {
		q.Set("flow", "xtls-rprx-vision")
	} else {
		q.Set("path", inb.Path)
		q.Set("mode", inb.Mode)
	}
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", uuid, formatLinkHost(host), inb.Port, q.Encode(), url.QueryEscape(inb.Tag))
}
func formatLinkHost(host string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]"
	}
	return host
}

// validateListenConflicts prevents an embedded Xray configuration from using an
// endpoint that another XHTTP or TCP-capable sing-box inbound already owns.
func (s *XrayXHTTPService) validateListenConflicts(candidate model.XrayXHTTPInbound) error {
	if !candidate.Enable {
		return nil
	}
	var xrayInbounds []model.XrayXHTTPInbound
	if err := database.GetDB().Where("enable = ? AND id <> ?", true, candidate.Id).Find(&xrayInbounds).Error; err != nil {
		return err
	}
	for _, existing := range xrayInbounds {
		if existing.Port == candidate.Port && listenerAddressesOverlap(existing.Listen, candidate.Listen) {
			return common.NewErrorf("XHTTP listener %s:%d conflicts with XHTTP inbound %q", candidate.Listen, candidate.Port, existing.Tag)
		}
	}

	var inbounds []model.Inbound
	if err := database.GetDB().Find(&inbounds).Error; err != nil {
		return err
	}
	for _, existing := range inbounds {
		if singBoxInboundUDPOnly(existing.Type) {
			continue
		}
		var listen struct {
			Listen     string `json:"listen"`
			ListenPort uint16 `json:"listen_port"`
		}
		if err := json.Unmarshal(existing.Options, &listen); err != nil {
			continue
		}
		if listen.ListenPort == candidate.Port && listenerAddressesOverlap(listen.Listen, candidate.Listen) {
			return common.NewErrorf("XHTTP listener %s:%d conflicts with sing-box inbound %q", candidate.Listen, candidate.Port, existing.Tag)
		}
	}
	return nil
}

func singBoxInboundUDPOnly(inboundType string) bool {
	switch inboundType {
	case "hysteria", "hysteria2", "tuic":
		return true
	default:
		return false
	}
}

func listenerAddressesOverlap(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" || left == right {
		return true
	}
	return left == "0.0.0.0" || left == "::" || right == "0.0.0.0" || right == "::"
}
func (s *XrayXHTTPService) validateClientIDs(ids []uint, requireEnabled bool) error {
	ids = uniqueUint(ids)
	if len(ids) == 0 {
		return common.NewError("select at least one S-UI client for XHTTP")
	}
	query := database.GetDB().Model(&model.Client{}).Where("id in ?", ids)
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(ids)) {
		return common.NewError("one or more selected XHTTP clients do not exist")
	}
	if requireEnabled {
		var enabledCount int64
		if err := database.GetDB().Model(&model.Client{}).Where("id in ? AND enable = ?", ids, true).Count(&enabledCount).Error; err != nil {
			return err
		}
		if enabledCount == 0 {
			return common.NewError("enable at least one selected S-UI client before enabling XHTTP")
		}
	}
	return nil
}

func normalizeXrayXHTTPInbound(inb *model.XrayXHTTPInbound) {
	inb.Tag = strings.TrimSpace(inb.Tag)
	inb.Listen = strings.TrimSpace(inb.Listen)
	inb.ServerName = strings.TrimSpace(inb.ServerName)
	inb.Dest = strings.TrimSpace(inb.Dest)
	inb.PrivateKey = strings.TrimSpace(inb.PrivateKey)
	inb.PublicKey = strings.TrimSpace(inb.PublicKey)
	inb.ShortID = strings.TrimSpace(inb.ShortID)
	inb.Fingerprint = strings.ToLower(strings.TrimSpace(inb.Fingerprint))
	inb.Transport = strings.ToLower(strings.TrimSpace(inb.Transport))
	inb.Path = strings.TrimSpace(inb.Path)
	inb.Mode = strings.TrimSpace(inb.Mode)
	if inb.Listen == "" {
		inb.Listen = "0.0.0.0"
	}
	if inb.Port == 0 {
		inb.Port = 2088
	}
	if inb.Fingerprint == "" {
		inb.Fingerprint = "chrome"
	}
	if inb.Transport == "" {
		// Rows created before the TCP Reality preset existed were XHTTP-only.
		inb.Transport = "xhttp"
	}
	if inb.Transport == "xhttp" {
		if inb.Path == "" {
			inb.Path = "/xhttp"
		}
		if inb.Mode == "" {
			inb.Mode = "auto"
		}
	}
}

func validateXrayXHTTPInbound(inb model.XrayXHTTPInbound) error {
	// Validation is also called by tests and by config rendering, so keep old
	// persisted XHTTP-only rows compatible even before a save migrates them.
	normalizeXrayXHTTPInbound(&inb)
	if inb.Tag == "" {
		return common.NewError("Xray Reality tag is required")
	}
	if inb.Port == 0 {
		return common.NewError("Xray Reality port is required")
	}
	if inb.ServerName == "" || inb.Dest == "" || inb.PrivateKey == "" || inb.PublicKey == "" || inb.ShortID == "" {
		return common.NewError("Reality server name, destination, private/public key and short ID are required")
	}
	if err := validateRealityKeyPair(inb.PrivateKey, inb.PublicKey); err != nil {
		return err
	}
	if !xrayruntime.ValidFingerprint(inb.Fingerprint) {
		return common.NewError("Reality client fingerprint is not supported by the embedded Xray version")
	}
	if net.ParseIP(inb.Listen) == nil {
		return common.NewError("Xray Reality listen address must be an IP address")
	}
	if len(inb.ShortID) > 16 || len(inb.ShortID)%2 != 0 {
		return common.NewError("Reality short ID must contain an even number of hexadecimal characters (up to 16)")
	}
	for _, ch := range inb.ShortID {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return common.NewError("Reality short ID must be hexadecimal")
		}
	}
	if inb.Transport != "tcp" && inb.Transport != "xhttp" {
		return common.NewError("Xray Reality transport must be tcp or xhttp")
	}
	if inb.Transport == "xhttp" {
		if !strings.HasPrefix(inb.Path, "/") {
			return common.NewError("XHTTP path must start with /")
		}
		if inb.Mode != "auto" && inb.Mode != "packet-up" && inb.Mode != "stream-up" && inb.Mode != "stream-one" {
			return common.NewError("unsupported XHTTP mode")
		}
	}
	host, port, err := net.SplitHostPort(inb.Dest)
	if err != nil || strings.TrimSpace(host) == "" {
		return common.NewError("Reality destination must be host:port")
	}
	portNumber, err := strconv.ParseUint(port, 10, 16)
	if err != nil || portNumber == 0 {
		return common.NewError("Reality destination port must be between 1 and 65535")
	}
	return nil
}

func validateRealityKeyPair(privateKeyText, publicKeyText string) error {
	privateKeyBytes, err := base64.RawURLEncoding.DecodeString(privateKeyText)
	if err != nil || len(privateKeyBytes) != 32 {
		return common.NewError("Reality private key must be a 32-byte raw URL-safe Base64 X25519 key")
	}
	publicKeyBytes, err := base64.RawURLEncoding.DecodeString(publicKeyText)
	if err != nil || len(publicKeyBytes) != 32 {
		return common.NewError("Reality public key must be a 32-byte raw URL-safe Base64 X25519 key")
	}
	privateKey, err := ecdh.X25519().NewPrivateKey(privateKeyBytes)
	if err != nil {
		return common.NewError("Reality private key is not a valid X25519 key")
	}
	if !bytes.Equal(privateKey.PublicKey().Bytes(), publicKeyBytes) {
		return common.NewError("Reality public key does not match the private key")
	}
	return nil
}

func uniqueUint(values []uint) []uint {
	out := make([]uint, 0, len(values))
	seen := map[uint]bool{}
	for _, v := range values {
		if v > 0 && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
