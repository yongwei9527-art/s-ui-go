package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
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
	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/util/common"
	"gorm.io/gorm"
)

const (
	xrayXHTTPConfigName         = "xray-xhttp.json"
	xrayXHTTPStartupGracePeriod = time.Second
)

type XrayXHTTPSaveRequest struct {
	Inbound   model.XrayXHTTPInbound `json:"inbound"`
	ClientIDs []uint                 `json:"clientIds"`
}

type xrayXHTTPRuntime struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	done       chan struct{}
	stopping   bool
	lastError  string
	lastExitAt int64
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
	binary, err := findXrayBinary()
	status := map[string]interface{}{
		"running":    xrayXHTTPProcess.cmd != nil && xrayXHTTPProcess.cmd.Process != nil,
		"binary":     binary,
		"configured": err == nil,
		"configPath": s.configPath(),
	}
	if err != nil {
		status["error"] = err.Error()
	}
	if xrayXHTTPProcess.lastError != "" {
		status["sidecarError"] = xrayXHTTPProcess.lastError
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
	inb := req.Inbound
	// The read API intentionally redacts the private key. Preserve it for edits
	// unless the administrator explicitly supplies a replacement.
	if inb.Id != 0 && strings.TrimSpace(inb.PrivateKey) == "" {
		var existing model.XrayXHTTPInbound
		if err := database.GetDB().First(&existing, inb.Id).Error; err != nil {
			return err
		}
		inb.PrivateKey = existing.PrivateKey
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

	// Enabling needs a real Xray binary. This avoids committing a configuration
	// which looks healthy in the UI but cannot ever be started.
	if inb.Enable {
		if _, err := findXrayBinary(); err != nil {
			return err
		}
	}

	db := database.GetDB()
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
		return tx.Create(&model.Changes{DateTime: time.Now().Unix(), Actor: "system", Key: "xray-xhttp", Action: "save", Obj: json.RawMessage(fmt.Sprintf(`{"tag":%q}`, inb.Tag))}).Error
	})
	if err != nil {
		return err
	}
	return s.Restart()
}

func (s *XrayXHTTPService) Delete(id uint) error {
	if id == 0 {
		return common.NewError("invalid XHTTP inbound id")
	}
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
		return tx.Create(&model.Changes{DateTime: time.Now().Unix(), Actor: "system", Key: "xray-xhttp", Action: "delete", Obj: json.RawMessage(fmt.Sprintf(`{"id":%d}`, id))}).Error
	})
	if err != nil {
		return err
	}
	return s.Restart()
}

func (s *XrayXHTTPService) Restart() error {
	xrayXHTTPProcess.mu.Lock()
	defer xrayXHTTPProcess.mu.Unlock()
	return s.restartLocked()
}

func (s *XrayXHTTPService) StartIfEnabled() error {
	var count int64
	if err := database.GetDB().Model(&model.XrayXHTTPInbound{}).Where("enable = ?", true).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	return s.Restart()
}

// ReloadAfterClientChange refreshes the running sidecar after S-UI client
// enablement or membership changes, so expired/disabled users lose access.
func (s *XrayXHTTPService) ReloadAfterClientChange() error {
	var count int64
	if err := database.GetDB().Model(&model.XrayXHTTPInbound{}).Where("enable = ?", true).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	return s.Restart()
}

func (s *XrayXHTTPService) Stop() error {
	xrayXHTTPProcess.mu.Lock()
	defer xrayXHTTPProcess.mu.Unlock()
	return stopXrayLocked()
}

func (s *XrayXHTTPService) restartLocked() error {
	configBytes, enabled, err := s.BuildConfig()
	if err != nil {
		return err
	}
	if !enabled {
		return stopXrayLocked()
	}
	binary, err := findXrayBinary()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(config.GetDBFolderPath(), 0750); err != nil {
		return err
	}
	// Test an isolated temporary file before stopping the live sidecar or
	// replacing its last known-good configuration.
	testFile, err := os.CreateTemp(config.GetDBFolderPath(), "xray-xhttp-*.json")
	if err != nil {
		return err
	}
	testPath := testFile.Name()
	defer os.Remove(testPath)
	if err := testFile.Chmod(0600); err != nil {
		_ = testFile.Close()
		return err
	}
	if _, err := testFile.Write(configBytes); err != nil {
		_ = testFile.Close()
		return err
	}
	if err := testFile.Close(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	test := exec.CommandContext(ctx, binary, "run", "-test", "-c", testPath)
	if output, err := test.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return common.NewError("Xray config test timed out")
		}
		return common.NewErrorf("Xray config test failed: %v: %s", err, strings.TrimSpace(string(output)))
	}
	if err := stopXrayLocked(); err != nil {
		return err
	}
	configPath := s.configPath()
	if err := os.WriteFile(configPath, configBytes, 0600); err != nil {
		return err
	}
	cmd := exec.Command(binary, "run", "-c", configPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan struct{})
	xrayXHTTPProcess.cmd = cmd
	xrayXHTTPProcess.done = done
	xrayXHTTPProcess.stopping = false
	xrayXHTTPProcess.lastError = ""
	xrayXHTTPProcess.lastExitAt = 0
	go func() {
		err := cmd.Wait()
		// Close before acquiring the runtime mutex: stopXrayLocked waits for
		// process exit while holding that mutex.
		close(done)
		xrayXHTTPProcess.mu.Lock()
		unexpectedExit := false
		if xrayXHTTPProcess.cmd == cmd {
			unexpectedExit = !xrayXHTTPProcess.stopping
			xrayXHTTPProcess.cmd = nil
			xrayXHTTPProcess.done = nil
			xrayXHTTPProcess.stopping = false
			xrayXHTTPProcess.lastExitAt = time.Now().Unix()
			if unexpectedExit {
				if err != nil {
					xrayXHTTPProcess.lastError = err.Error()
				} else {
					xrayXHTTPProcess.lastError = "process exited unexpectedly"
				}
			}
		}
		xrayXHTTPProcess.mu.Unlock()
		if err != nil && unexpectedExit {
			logger.Warning("Xray XHTTP sidecar stopped unexpectedly: ", err)
		}
	}()
	if err := waitForXrayXHTTPStartup(done, xrayXHTTPStartupGracePeriod); err != nil {
		return err
	}
	logger.Info("Xray XHTTP sidecar started on configured XHTTP inbound(s)")
	return nil
}

func waitForXrayXHTTPStartup(done <-chan struct{}, grace time.Duration) error {
	select {
	case <-done:
		return common.NewError("Xray XHTTP sidecar exited immediately after start; check the Xray log and listener port")
	case <-time.After(grace):
		return nil
	}
}

func stopXrayLocked() error {
	if xrayXHTTPProcess.cmd == nil || xrayXHTTPProcess.cmd.Process == nil {
		return nil
	}
	cmd := xrayXHTTPProcess.cmd
	done := xrayXHTTPProcess.done
	xrayXHTTPProcess.stopping = true
	if runtime.GOOS == "windows" {
		// /T terminates descendants too, which is important if a wrapper starts
		// the actual core process.
		_ = exec.Command("taskkill", "/PID", fmt.Sprint(cmd.Process.Pid), "/T", "/F").Run()
	} else {
		_ = cmd.Process.Signal(os.Interrupt)
	}

	if done != nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			// Graceful termination can be ignored by a broken child process.
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				return common.NewError("timed out waiting for Xray XHTTP sidecar to stop")
			}
		}
	}
	xrayXHTTPProcess.cmd = nil
	xrayXHTTPProcess.done = nil
	xrayXHTTPProcess.stopping = false
	return nil
}
func (s *XrayXHTTPService) configPath() string {
	return filepath.Join(config.GetDBFolderPath(), xrayXHTTPConfigName)
}

func findXrayBinary() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("SUI_XRAY_BINARY")); explicit != "" {
		if info, err := os.Stat(explicit); err == nil && !info.IsDir() {
			return explicit, nil
		}
		return "", common.NewError("SUI_XRAY_BINARY does not point to an executable file")
	}
	name := "xray"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", common.NewError("Xray binary not found; install xray-core and set SUI_XRAY_BINARY to its executable path")
}

func (s *XrayXHTTPService) BuildConfig() ([]byte, bool, error) {
	var inbounds []model.XrayXHTTPInbound
	if err := database.GetDB().Where("enable = ?", true).Order("id").Find(&inbounds).Error; err != nil {
		return nil, false, err
	}
	if len(inbounds) == 0 {
		return []byte(`{"inbounds":[],"outbounds":[{"protocol":"freedom","tag":"direct"}]}`), false, nil
	}

	xrayInbounds := make([]map[string]interface{}, 0, len(inbounds))
	for _, inb := range inbounds {
		// Existing XHTTP-only records predate the transport field. Normalize them
		// while rendering so an upgrade never turns a valid saved configuration
		// into an invalid sidecar configuration.
		normalizeXrayXHTTPInbound(&inb)
		if err := validateXrayXHTTPInbound(inb); err != nil {
			return nil, false, err
		}
		var rows []struct {
			UUID string
			Name string
		}
		err := database.GetDB().Table("xray_x_http_clients").Select("xray_x_http_clients.uuid, clients.name").Joins("join clients on clients.id = xray_x_http_clients.client_id").Where("xray_x_http_clients.inbound_id = ? and xray_x_http_clients.enable = ? and clients.enable = ?", inb.Id, true, true).Scan(&rows).Error
		if err != nil {
			return nil, false, err
		}
		clients := make([]map[string]string, 0, len(rows))
		for _, row := range rows {
			clients = append(clients, xrayClientConfig(inb.Transport, row.UUID, row.Name))
		}
		// A client can be disabled, expired, or deleted after this inbound was
		// created. Omit an empty inbound so a subsequent reload revokes access
		// instead of retaining stale UUIDs in a running sidecar.
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
func (s *XrayXHTTPService) LinksForClient(clientID uint, hostname string) ([]string, error) {
	var rows []struct {
		model.XrayXHTTPInbound
		UUID string
	}
	err := database.GetDB().Table("xray_x_http_inbounds").Select("xray_x_http_inbounds.*, xray_x_http_clients.uuid").Joins("join xray_x_http_clients on xray_x_http_clients.inbound_id = xray_x_http_inbounds.id").Where("xray_x_http_inbounds.enable = ? and xray_x_http_clients.enable = ? and xray_x_http_clients.client_id = ?", true, true, clientID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	links := make([]string, 0, len(rows))
	for _, row := range rows {
		links = append(links, xrayVLESSLink(row.XrayXHTTPInbound, row.UUID, hostname))
	}
	return links, nil
}

func xrayVLESSLink(inb model.XrayXHTTPInbound, uuid, hostname string) string {
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
	transport := strings.ToLower(strings.TrimSpace(inb.Transport))
	if transport == "" {
		transport = "xhttp"
	}
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

// validateListenConflicts prevents a saved sidecar configuration from using an
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
	inb.Fingerprint = strings.TrimSpace(inb.Fingerprint)
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
