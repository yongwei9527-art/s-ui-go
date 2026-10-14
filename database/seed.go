package database

import (
	"bytes"
	stdtls "crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/util"
	"github.com/yongwei9527-art/s-ui-go/util/common"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"
)

const (
	defaultTemplateClientName = "default-user"

	// Reserved non-resolving name for self-signed bootstrap TLS, not a third-party identity.
	defaultBootstrapTLSName = "bootstrap.s-ui.invalid"

	// selfSignedTLSBootstrapOnlySuffix is persisted in the TLS template name so
	// operators can distinguish the first-boot, self-signed templates from TLS
	// configurations that are suitable for production use.
	//
	// These templates intentionally use client insecure: true because their
	// generated certificate is self-signed. Replace the certificate and disable
	// insecure verification before exposing the corresponding inbound publicly.
	selfSignedTLSBootstrapOnlySuffix = "-self-signed-bootstrap-only"
)

type seededTLS struct {
	id   uint
	name string
}

func seedDefaultTlsAndInbounds() error {
	production, err := loadDefaultProductionTLS()
	if err != nil {
		return err
	}
	var tlsCount int64
	if err := db.Model(&model.Tls{}).Count(&tlsCount).Error; err != nil {
		return err
	}

	var inboundCount int64
	if err := db.Model(&model.Inbound{}).Count(&inboundCount).Error; err != nil {
		return err
	}

	if tlsCount > 0 || inboundCount > 0 {
		if os.Getenv("S_UI_DEFAULT_TLS_MIGRATE_BOOTSTRAP") == "true" {
			if production == nil {
				return fmt.Errorf("TLS migration requires all three S_UI_DEFAULT_TLS_* settings")
			}
			return migrateDefaultProductionTLS(db, production)
		}
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		var seedHost string
		if production != nil {
			seedHost = production.serverName
		} else {
			seedHost = detectDefaultSeedHost()
		}
		seeded, err := seedDefaultTlsTemplatesWithConfig(tx, production)
		if err != nil {
			return err
		}

		inbounds, err := seedDefaultInbounds(tx, seeded, seedHost)
		if err != nil {
			return err
		}

		if err := seedDefaultClient(tx, inbounds, seedHost); err != nil {
			return err
		}
		// A packaged Linux installation explicitly opts in to a ready-to-use
		// Xray Reality presets. Other bootstrap paths keep the Xray templates
		// as safe drafts until an administrator enables one.
		return seedDefaultXrayRealityTemplates(tx, inbounds)
	})
}

func seedDefaultTlsTemplates(tx *gorm.DB) (map[string]seededTLS, error) {
	production, err := loadDefaultProductionTLS()
	if err != nil {
		return nil, err
	}
	return seedDefaultTlsTemplatesWithConfig(tx, production)
}

func seedDefaultTlsTemplatesWithConfig(tx *gorm.DB, production *defaultProductionTLS) (map[string]seededTLS, error) {
	buildDefaultTLS := func(name, host string, alpn []string) (model.Tls, error) {
		if production != nil {
			return buildProductionTLS(name, alpn, production)
		}
		return buildBootstrapOnlySelfSignedTLS(name, host, alpn)
	}
	realityTLS, err := buildRealityTLS("reality-vless", "go.microsoft.com")
	if err != nil {
		return nil, err
	}
	hy2TLS, err := buildDefaultTLS("tls-hy2", defaultBootstrapTLSName, []string{"h3"})
	if err != nil {
		return nil, err
	}
	tuicTLS, err := buildDefaultTLS("tls-tuic", defaultBootstrapTLSName, []string{"h3"})
	if err != nil {
		return nil, err
	}
	trojanTLS, err := buildDefaultTLS("tls-trojan", defaultBootstrapTLSName, []string{"http/1.1"})
	if err != nil {
		return nil, err
	}
	trojanWSTLS, err := buildDefaultTLS("tls-trojan-ws", defaultBootstrapTLSName, []string{"http/1.1"})
	if err != nil {
		return nil, err
	}
	vmessWSTLS, err := buildDefaultTLS("tls-vmess-ws", defaultBootstrapTLSName, []string{"http/1.1"})
	if err != nil {
		return nil, err
	}
	naiveTLS, err := buildDefaultTLS("tls-naive", defaultBootstrapTLSName, []string{"h2", "http/1.1"})
	if err != nil {
		return nil, err
	}
	anyTLSTLS, err := buildDefaultTLS("tls-anytls", defaultBootstrapTLSName, []string{"http/1.1"})
	if err != nil {
		return nil, err
	}
	hysteriaTLS, err := buildDefaultTLS("tls-hysteria", defaultBootstrapTLSName, []string{"h3"})
	if err != nil {
		return nil, err
	}

	templates := []model.Tls{realityTLS, hy2TLS, tuicTLS, trojanTLS, trojanWSTLS, vmessWSTLS, naiveTLS, anyTLSTLS, hysteriaTLS}
	seeded := make(map[string]seededTLS, len(templates))

	for _, tpl := range templates {
		if err := tx.Create(&tpl).Error; err != nil {
			return nil, err
		}
		key := tpl.Name
		if production != nil && tpl.Name != "reality-vless" {
			// Stable internal lookup keys; persisted names identify production TLS.
			key = bootstrapOnlySelfSignedTLSName(tpl.Name)
		}
		seeded[key] = seededTLS{id: tpl.Id, name: tpl.Name}
	}

	return seeded, nil
}

func buildRealityTLS(name string, serverName string) (model.Tls, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return model.Tls{}, err
	}
	publicKey := privateKey.PublicKey()
	shortID, err := randomHex(8)
	if err != nil {
		return model.Tls{}, err
	}

	server := map[string]interface{}{
		"enabled":       true,
		"server_name":   serverName,
		"alpn":          []string{"h2", "http/1.1"},
		"min_version":   "1.2",
		"max_version":   "1.3",
		"cipher_suites": []string{},
		"reality": map[string]interface{}{
			"enabled": true,
			"handshake": map[string]interface{}{
				"server":      serverName,
				"server_port": 443,
			},
			"private_key": base64.RawURLEncoding.EncodeToString(privateKey[:]),
			"short_id":    []string{shortID},
		},
	}
	client := map[string]interface{}{
		"enabled":     true,
		"server_name": serverName,
		"utls": map[string]interface{}{
			"enabled":     true,
			"fingerprint": "chrome",
		},
		"reality": map[string]interface{}{
			"enabled":    true,
			"public_key": base64.RawURLEncoding.EncodeToString(publicKey[:]),
			"short_id":   shortID,
		},
	}

	return tlsModel(name, server, client)
}

// buildBootstrapOnlySelfSignedTLS creates an initial-connectivity TLS template.
// The name marker is a guardrail: a self-signed certificate requires client
// insecure verification, so it must not be presented as a production TLS default.
func buildBootstrapOnlySelfSignedTLS(baseName string, serverName string, alpn []string) (model.Tls, error) {
	name := bootstrapOnlySelfSignedTLSName(baseName)
	privateKeyPEM, certPEM, err := tls.GenerateCertificate(nil, nil, time.Now, serverName, time.Now().AddDate(1, 0, 0))
	if err != nil {
		return model.Tls{}, err
	}

	return buildCertificateTLS(name, serverName, alpn, certPEM, privateKeyPEM, true)
}

func buildCertificateTLS(name, serverName string, alpn []string, certPEM, privateKeyPEM []byte, insecure bool) (model.Tls, error) {
	server := map[string]interface{}{
		"enabled":       true,
		"server_name":   serverName,
		"alpn":          alpn,
		"min_version":   "1.2",
		"max_version":   "1.3",
		"cipher_suites": []string{},
		"certificate":   pemLines(certPEM),
		"key":           pemLines(privateKeyPEM),
	}
	client := map[string]interface{}{
		"enabled":       true,
		"server_name":   serverName,
		"insecure":      insecure,
		"alpn":          alpn,
		"min_version":   "1.2",
		"max_version":   "1.3",
		"cipher_suites": []string{},
		"utls": map[string]interface{}{
			"enabled":     true,
			"fingerprint": "chrome",
		},
	}

	return tlsModel(name, server, client)
}

func bootstrapOnlySelfSignedTLSName(baseName string) string {
	return baseName + selfSignedTLSBootstrapOnlySuffix
}

type legacyBootstrapOnlyTLSSpec struct {
	name       string
	serverName string
	alpn       []string
}

var legacyBootstrapOnlyTLSSpecs = []legacyBootstrapOnlyTLSSpec{
	{name: "tls-hy2", serverName: "tls1.tcm.mmbing.net", alpn: []string{"h3"}},
	{name: "tls-tuic", serverName: "snap.licdn.com", alpn: []string{"h3"}},
	{name: "tls-trojan", serverName: "snap.licdn.com", alpn: []string{"http/1.1"}},
	{name: "tls-trojan-ws", serverName: "snap.licdn.com", alpn: []string{"http/1.1"}},
	{name: "tls-vmess-ws", serverName: "snap.licdn.com", alpn: []string{"http/1.1"}},
	{name: "tls-naive", serverName: "snap.licdn.com", alpn: []string{"h2", "http/1.1"}},
	{name: "tls-anytls", serverName: "snap.licdn.com", alpn: []string{"http/1.1"}},
	{name: "tls-hysteria", serverName: "tls1.tcm.mmbing.net", alpn: []string{"h3"}},
}

// migrateLegacyBootstrapOnlyTLSTemplateNames upgrades only the old generated
// self-signed templates. The structural match is intentionally strict: a name
// match alone must never rename a user-maintained TLS template during startup.
//
// The update changes only the display name. Existing inbound TLS IDs and all
// certificate/configuration bytes remain untouched, so it is safe to run on
// every startup and naturally idempotent after the first successful update.
func migrateLegacyBootstrapOnlyTLSTemplateNames(tx *gorm.DB) error {
	for _, spec := range legacyBootstrapOnlyTLSSpecs {
		var candidates []model.Tls
		if err := tx.Where("name = ?", spec.name).Find(&candidates).Error; err != nil {
			return err
		}
		for _, candidate := range candidates {
			if !matchesLegacyBootstrapOnlyTLS(candidate, spec) {
				continue
			}
			if err := tx.Model(&model.Tls{}).
				Where("id = ? AND name = ?", candidate.Id, spec.name).
				Update("name", bootstrapOnlySelfSignedTLSName(spec.name)).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func matchesLegacyBootstrapOnlyTLS(template model.Tls, spec legacyBootstrapOnlyTLSSpec) bool {
	var server, client map[string]interface{}
	if json.Unmarshal(template.Server, &server) != nil || json.Unmarshal(template.Client, &client) != nil {
		return false
	}
	if !bootstrapOnlyTLSSettingsMatch(server, spec, true) || !bootstrapOnlyTLSSettingsMatch(client, spec, false) {
		return false
	}

	utls, ok := client["utls"].(map[string]interface{})
	if !ok || utls["enabled"] != true || utls["fingerprint"] != "chrome" {
		return false
	}
	return hasNonEmptyStringList(server["certificate"]) && hasNonEmptyStringList(server["key"])
}

func bootstrapOnlyTLSSettingsMatch(settings map[string]interface{}, spec legacyBootstrapOnlyTLSSpec, server bool) bool {
	if settings["enabled"] != true || settings["server_name"] != spec.serverName ||
		settings["min_version"] != "1.2" || settings["max_version"] != "1.3" ||
		!stringListMatches(settings["alpn"], spec.alpn) || !stringListMatches(settings["cipher_suites"], nil) {
		return false
	}
	if !server && settings["insecure"] != true {
		return false
	}
	_, hasReality := settings["reality"]
	return !hasReality
}

func stringListMatches(value interface{}, expected []string) bool {
	values, ok := value.([]interface{})
	if !ok || len(values) != len(expected) {
		return false
	}
	for index, expectedValue := range expected {
		if values[index] != expectedValue {
			return false
		}
	}
	return true
}

func hasNonEmptyStringList(value interface{}) bool {
	values, ok := value.([]interface{})
	if !ok || len(values) == 0 {
		return false
	}
	for _, value := range values {
		if stringValue, ok := value.(string); !ok || stringValue == "" {
			return false
		}
	}
	return true
}

func tlsModel(name string, server map[string]interface{}, client map[string]interface{}) (model.Tls, error) {
	serverJSON, err := json.MarshalIndent(server, "", "  ")
	if err != nil {
		return model.Tls{}, err
	}
	clientJSON, err := json.MarshalIndent(client, "", "  ")
	if err != nil {
		return model.Tls{}, err
	}
	return model.Tls{Name: name, Server: serverJSON, Client: clientJSON}, nil
}

func seedDefaultInbounds(tx *gorm.DB, tlsTemplates map[string]seededTLS, seedHost string) ([]model.Inbound, error) {
	vlessPort := availableListenPort("tcp", 443)
	hy2Port := availableListenPort("udp", 443)
	tuicPort := availableListenPort("udp", 8443)
	trojanPort := availableListenPort("tcp", 8443)
	vmessPort := availableListenPort("tcp", 2053)
	shadowsocksPort := availableListenPort("tcp", 8388)
	naivePort := availableListenPort("tcp", 2087)
	anyTLSPort := availableListenPort("tcp", 2097)
	hysteriaPort := availableListenPort("udp", 2083)
	shadowsocksServerPassword, err := randomBase64(16)
	if err != nil {
		return nil, err
	}

	inbounds := []json.RawMessage{
		inboundJSON("vless", inboundTag("vless-reality", vlessPort), tlsTemplates["reality-vless"].id, map[string]interface{}{
			"listen":      "0.0.0.0",
			"listen_port": vlessPort,
			"transport":   map[string]interface{}{},
		}, []map[string]interface{}{{"server": seedHost, "server_port": vlessPort, "remark": inboundTag("vless-reality", vlessPort)}}),
		inboundJSON("hysteria2", inboundTag("hy2", hy2Port), tlsTemplates[bootstrapOnlySelfSignedTLSName("tls-hy2")].id, map[string]interface{}{
			"listen":                  "0.0.0.0",
			"listen_port":             hy2Port,
			"udp_fragment":            true,
			"udp_timeout":             "5m",
			"ignore_client_bandwidth": true,
		}, []map[string]interface{}{{"server": seedHost, "server_port": hy2Port, "remark": inboundTag("hy2", hy2Port), "tls": true, "insecure": true}}),
		inboundJSON("tuic", inboundTag("tuic", tuicPort), tlsTemplates[bootstrapOnlySelfSignedTLSName("tls-tuic")].id, map[string]interface{}{
			"listen":             "0.0.0.0",
			"listen_port":        tuicPort,
			"congestion_control": "cubic",
			"auth_timeout":       "3s",
			"heartbeat":          "10s",
			"udp_fragment":       true,
			"udp_timeout":        "5m",
		}, []map[string]interface{}{{"server": seedHost, "server_port": tuicPort, "remark": inboundTag("tuic", tuicPort), "tls": true, "insecure": true}}),
		inboundJSON("trojan", inboundTag("trojan-tls", trojanPort), tlsTemplates[bootstrapOnlySelfSignedTLSName("tls-trojan")].id, map[string]interface{}{
			"listen":      "0.0.0.0",
			"listen_port": trojanPort,
			"transport":   map[string]interface{}{},
		}, []map[string]interface{}{{"server": seedHost, "server_port": trojanPort, "remark": inboundTag("trojan-tls", trojanPort), "tls": true, "insecure": true}}),
		inboundJSON("vmess", inboundTag("vmess-ws-tls", vmessPort), tlsTemplates[bootstrapOnlySelfSignedTLSName("tls-vmess-ws")].id, map[string]interface{}{
			"listen":      "0.0.0.0",
			"listen_port": vmessPort,
			"transport": map[string]interface{}{
				"type": "ws",
				"path": "/vmess",
				"headers": map[string]interface{}{
					"Host": seedHost,
				},
			},
		}, []map[string]interface{}{{"server": seedHost, "server_port": vmessPort, "remark": inboundTag("vmess-ws-tls", vmessPort), "tls": true, "insecure": true}}),
		inboundJSON("shadowsocks", inboundTag("ss-2022", shadowsocksPort), 0, map[string]interface{}{
			"listen":      "0.0.0.0",
			"listen_port": shadowsocksPort,
			"method":      "2022-blake3-aes-128-gcm",
			"password":    shadowsocksServerPassword,
		}, []map[string]interface{}{{"server": seedHost, "server_port": shadowsocksPort, "remark": inboundTag("ss-2022", shadowsocksPort)}}),
		inboundJSON("naive", inboundTag("naive-tls", naivePort), tlsTemplates[bootstrapOnlySelfSignedTLSName("tls-naive")].id, map[string]interface{}{
			"listen":      "0.0.0.0",
			"listen_port": naivePort,
		}, []map[string]interface{}{{"server": seedHost, "server_port": naivePort, "remark": inboundTag("naive-tls", naivePort), "tls": true, "insecure": true}}),
		inboundJSON("anytls", inboundTag("anytls", anyTLSPort), tlsTemplates[bootstrapOnlySelfSignedTLSName("tls-anytls")].id, map[string]interface{}{
			"listen":      "0.0.0.0",
			"listen_port": anyTLSPort,
			"padding_scheme": []string{
				"stop=8",
				"0=30-30",
				"1=100-400",
				"2=400-500,c,500-1000,c,500-1000,c,500-1000,c,500-1000",
				"3=9-9,500-1000",
				"4=500-1000",
				"5=500-1000",
				"6=500-1000",
				"7=500-1000",
			},
		}, []map[string]interface{}{{"server": seedHost, "server_port": anyTLSPort, "remark": inboundTag("anytls", anyTLSPort), "tls": true, "insecure": true}}),
		inboundJSON("hysteria", inboundTag("hysteria-tls", hysteriaPort), tlsTemplates[bootstrapOnlySelfSignedTLSName("tls-hysteria")].id, map[string]interface{}{
			"listen":       "0.0.0.0",
			"listen_port":  hysteriaPort,
			"up_mbps":      50,
			"down_mbps":    100,
			"udp_fragment": true,
			"udp_timeout":  "5m",
		}, []map[string]interface{}{{"server": seedHost, "server_port": hysteriaPort, "remark": inboundTag("hysteria-tls", hysteriaPort), "tls": true, "insecure": true}}),
	}

	seededInbounds := make([]model.Inbound, 0, len(inbounds))
	for _, raw := range inbounds {
		var inbound model.Inbound
		if err := inbound.UnmarshalJSON(raw); err != nil {
			return nil, err
		}
		if inbound.TlsId > 0 {
			var tlsModel model.Tls
			if err := tx.Model(&model.Tls{}).Where("id = ?", inbound.TlsId).First(&tlsModel).Error; err != nil {
				return nil, err
			}
			inbound.Tls = &tlsModel
			var clientTLS map[string]interface{}
			if err := json.Unmarshal(tlsModel.Client, &clientTLS); err != nil {
				return nil, err
			}
			if clientTLS["insecure"] == false {
				var addrs []map[string]interface{}
				if err := json.Unmarshal(inbound.Addrs, &addrs); err != nil {
					return nil, err
				}
				for _, addr := range addrs {
					addr["insecure"] = false
				}
				inbound.Addrs, err = json.Marshal(addrs)
				if err != nil {
					return nil, err
				}
			}
		}
		if err := util.FillOutJson(&inbound, seedHost); err != nil {
			return nil, err
		}
		inboundTLS := inbound.Tls
		inbound.Tls = nil
		if err := tx.Create(&inbound).Error; err != nil {
			return nil, err
		}
		inbound.Tls = inboundTLS
		seededInbounds = append(seededInbounds, inbound)
	}
	return seededInbounds, nil
}

func inboundTag(prefix string, port int) string {
	return prefix + "-" + strconv.Itoa(port)
}

func availableListenPort(network string, preferred int) int {
	for port := preferred; port < preferred+100; port++ {
		if canListen(network, port) {
			return port
		}
	}
	return preferred
}

func canListen(network string, port int) bool {
	address := net.JoinHostPort("0.0.0.0", strconv.Itoa(port))
	if network == "udp" {
		conn, err := net.ListenPacket("udp", address)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

func inboundJSON(inboundType string, tag string, tlsID uint, options map[string]interface{}, addrs []map[string]interface{}) json.RawMessage {
	for _, addr := range addrs {
		if _, exists := addr[util.AutoServerKey]; !exists {
			addr[util.AutoServerKey] = true
		}
	}
	data := map[string]interface{}{
		"type":     inboundType,
		"tag":      tag,
		"tls_id":   tlsID,
		"addrs":    addrs,
		"out_json": map[string]interface{}{},
	}
	for key, value := range options {
		data[key] = value
	}
	payload, _ := json.Marshal(data)
	return payload
}

func seedDefaultClient(tx *gorm.DB, inbounds []model.Inbound, seedHost string) error {
	var clientCount int64
	if err := tx.Model(&model.Client{}).Count(&clientCount).Error; err != nil {
		return err
	}
	if clientCount > 0 {
		return nil
	}

	clientUUID, err := uuid.NewV4()
	if err != nil {
		return err
	}
	password, err := common.RandomWithError(10)
	if err != nil {
		return err
	}
	shadowsocksPassword, err := randomBase64(32)
	if err != nil {
		return err
	}
	shadowsocks16Password, err := randomBase64(16)
	if err != nil {
		return err
	}
	config := map[string]interface{}{
		"vless": map[string]interface{}{
			"name": defaultTemplateClientName,
			"uuid": clientUUID.String(),
			"flow": "xtls-rprx-vision",
		},
		"vmess": map[string]interface{}{
			"name":    defaultTemplateClientName,
			"uuid":    clientUUID.String(),
			"alterId": 0,
		},
		"shadowsocks": map[string]interface{}{
			"name":     defaultTemplateClientName,
			"password": shadowsocksPassword,
		},
		"shadowsocks16": map[string]interface{}{
			"name":     defaultTemplateClientName,
			"password": shadowsocks16Password,
		},
		"trojan": map[string]interface{}{
			"name":     defaultTemplateClientName,
			"password": password,
		},
		"naive": map[string]interface{}{
			"username": defaultTemplateClientName,
			"password": password,
		},
		"hysteria": map[string]interface{}{
			"name":     defaultTemplateClientName,
			"auth_str": password,
		},
		"hysteria2": map[string]interface{}{
			"name":     defaultTemplateClientName,
			"password": password,
		},
		"anytls": map[string]interface{}{
			"name":     defaultTemplateClientName,
			"password": password,
		},
		"tuic": map[string]interface{}{
			"name":     defaultTemplateClientName,
			"uuid":     clientUUID.String(),
			"password": password,
		},
	}
	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	inboundIDs := make([]uint, 0, len(inbounds))
	for _, inbound := range inbounds {
		inboundIDs = append(inboundIDs, inbound.Id)
	}
	inboundsJSON, err := json.Marshal(inboundIDs)
	if err != nil {
		return err
	}

	links := make([]map[string]string, 0, len(inbounds))
	for _, inbound := range inbounds {
		for _, link := range util.LinkGenerator(configJSON, &inbound, seedHost) {
			links = append(links, map[string]string{
				"remark": inbound.Tag,
				"type":   "local",
				"uri":    link,
			})
		}
	}
	linksJSON, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}

	client := model.Client{
		Enable:   true,
		Name:     defaultTemplateClientName,
		Config:   configJSON,
		Inbounds: inboundsJSON,
		Links:    linksJSON,
		Volume:   0,
		Expiry:   0,
		Desc:     "Built-in default client for seeded protocol templates",
		Group:    "default",
	}
	return tx.Create(&client).Error
}

// seedDefaultXrayRealityTemplates creates embedded Xray templates during the same
// first-boot transaction that creates the sing-box templates. The packaged Linux
// installer explicitly opts in to both Reality presets only after it has verified
// their ports and target, so a fresh installation starts with usable Xray endpoints.
func seedDefaultXrayRealityTemplates(tx *gorm.DB, singBoxInbounds []model.Inbound) error {
	autoEnable := autoEnableDefaultXrayReality()
	var client model.Client
	if err := tx.Where("name = ?", defaultTemplateClientName).First(&client).Error; err != nil {
		// A partially populated legacy database can have no TLS/inbounds yet
		// already contain users. Do not manufacture a new Xray assignment in
		// that upgrade-shaped state; these drafts are first-boot defaults only.
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}

	for _, template := range []struct {
		transport string
		preferred int
		path      string
		mode      string
	}{
		{transport: "tcp", preferred: 10443},
		{transport: "xhttp", preferred: 2088, path: "/xhttp", mode: "auto"},
	} {
		port := availableSeedXrayTCPPort(singBoxInbounds, template.preferred)
		privateKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}
		publicKey := privateKey.PublicKey()
		xrayUUID, err := uuid.NewV4()
		if err != nil {
			return err
		}
		shortID, err := randomHex(8)
		if err != nil {
			return err
		}
		inbound := model.XrayXHTTPInbound{
			Enable:      autoEnable,
			Tag:         inboundTag("xray-vless-reality-"+template.transport, port),
			Listen:      "0.0.0.0",
			Port:        uint16(port),
			ServerName:  "go.microsoft.com",
			Dest:        "go.microsoft.com:443",
			PrivateKey:  base64.RawURLEncoding.EncodeToString(privateKey[:]),
			PublicKey:   base64.RawURLEncoding.EncodeToString(publicKey[:]),
			ShortID:     shortID,
			Fingerprint: "chrome",
			Transport:   template.transport,
			Path:        template.path,
			Mode:        template.mode,
		}
		if err := tx.Create(&inbound).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.XrayXHTTPClient{
			InboundID: inbound.Id,
			ClientID:  client.Id,
			Enable:    true,
			UUID:      xrayUUID.String(),
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

// autoEnableDefaultXrayReality is intentionally opt-in because Docker/custom
// deployments may have firewall policies that forbid opening public listeners.
// The Linux installer sets this only after checking the Reality target and all
// default ports; Xray-core itself is already compiled into the sui process.
func autoEnableDefaultXrayReality() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("SUI_AUTOCONFIGURE_XRAY_REALITY")))
	return value == "1" || value == "true" || value == "yes"
}

// availableSeedXrayTCPPort accounts for TCP ports already selected by the
// sing-box first-boot templates. Those templates are not listening yet while
// this transaction runs, so an OS bind probe alone cannot see their claims.
func availableSeedXrayTCPPort(singBoxInbounds []model.Inbound, preferred int) int {
	for port := preferred; port < preferred+100; port++ {
		if seedSingBoxTCPPortReserved(singBoxInbounds, port) {
			continue
		}
		if canListen("tcp", port) {
			return port
		}
	}
	// The template is disabled, and enabling it later performs the authoritative
	// cross-core listener conflict check. Keep a stable candidate even when the
	// host has no free port in the small first-boot probe range.
	return preferred
}

func seedSingBoxTCPPortReserved(inbounds []model.Inbound, port int) bool {
	for _, inbound := range inbounds {
		if inbound.Type == "hysteria" || inbound.Type == "hysteria2" || inbound.Type == "tuic" {
			continue
		}
		var options struct {
			ListenPort int `json:"listen_port"`
		}
		if err := json.Unmarshal(inbound.Options, &options); err == nil && options.ListenPort == port {
			return true
		}
	}
	return false
}
func detectDefaultSeedHost() string {
	if ip := detectPublicIP(); ip != "" {
		return ip
	}
	if ip := detectLocalIPv4(); ip != "" {
		return ip
	}
	return "localhost"
}

func detectPublicIP() string {
	apis := []string{
		"https://api64.ipify.org",
		"https://api.ipify.org",
		"https://ip.sb",
		"https://icanhazip.com",
		"https://ipinfo.io/ip",
		"https://checkip.amazonaws.com",
	}
	client := &http.Client{Timeout: time.Second}
	for _, api := range apis {
		resp, err := client.Get(api)
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
			continue
		}
		ip := strings.TrimSpace(string(body))
		if parsed := net.ParseIP(ip); parsed != nil && !parsed.IsLoopback() && !parsed.IsPrivate() && !parsed.IsUnspecified() {
			return ip
		}
	}
	return ""
}

func detectLocalIPv4() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() {
				continue
			}
			if ipv4 := ip.To4(); ipv4 != nil {
				return ipv4.String()
			}
		}
	}
	return ""
}

func randomHex(byteLen int) (string, error) {
	buf, err := common.RandomBytes(byteLen)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomBase64(byteLen int) (string, error) {
	buf, err := common.RandomBytes(byteLen)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

func pemLines(pem []byte) []string {
	lines := strings.Split(strings.TrimSpace(string(pem)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}
	}
	return lines
}

// defaultProductionTLS is supplied by the operator. Validation does not assert
// public-CA trust: no CA identity or chain is manufactured here.
type defaultProductionTLS struct {
	serverName               string
	certificate, key         []byte
	certificatePath, keyPath string
}

func loadDefaultProductionTLS() (*defaultProductionTLS, error) {
	names := []string{"S_UI_DEFAULT_TLS_SERVER_NAME", "S_UI_DEFAULT_TLS_CERTIFICATE_PATH", "S_UI_DEFAULT_TLS_KEY_PATH"}
	values := make([]string, len(names))
	configured := false
	for i, name := range names {
		value, present := os.LookupEnv(name)
		configured = configured || present
		values[i] = strings.TrimSpace(value)
	}
	if !configured {
		return nil, nil
	}
	for i, value := range values {
		if value == "" {
			return nil, fmt.Errorf("%s is required when default TLS is configured", names[i])
		}
	}
	cert, err := os.ReadFile(values[1])
	if err != nil {
		return nil, fmt.Errorf("read default TLS certificate: %w", err)
	}
	key, err := os.ReadFile(values[2])
	if err != nil {
		return nil, fmt.Errorf("read default TLS key: %w", err)
	}
	pair, err := stdtls.X509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("invalid default TLS certificate/key pair: %w", err)
	}
	now := time.Now()
	for i, der := range pair.Certificate {
		certificate, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("parse default TLS certificate %d: %w", i, err)
		}
		if now.Before(certificate.NotBefore) || !now.Before(certificate.NotAfter) {
			return nil, fmt.Errorf("default TLS certificate %d is outside its validity period", i)
		}
		if i == 0 {
			if err := certificate.VerifyHostname(values[0]); err != nil {
				return nil, fmt.Errorf("default TLS hostname: %w", err)
			}
		}
	}
	certificatePath, err := filepath.Abs(values[1])
	if err != nil {
		return nil, err
	}
	keyPath, err := filepath.Abs(values[2])
	if err != nil {
		return nil, err
	}
	return &defaultProductionTLS{serverName: values[0], certificate: cert, key: key, certificatePath: certificatePath, keyPath: keyPath}, nil
}

func buildProductionTLS(name string, alpn []string, config *defaultProductionTLS) (model.Tls, error) {
	template, err := buildCertificateTLS(name, config.serverName, alpn, config.certificate, config.key, false)
	if err != nil || config.certificatePath == "" || config.keyPath == "" {
		return template, err
	}
	var server map[string]interface{}
	if err := json.Unmarshal(template.Server, &server); err != nil {
		return model.Tls{}, err
	}
	// Keep renewed material outside the database; sing-box watches these files.
	delete(server, "certificate")
	delete(server, "key")
	server["certificate_path"] = config.certificatePath
	server["key_path"] = config.keyPath
	template.Server, err = json.MarshalIndent(server, "", "  ")
	return template, err
}

// Exact JSON shape plus the self-signed bootstrap certificate profile excludes
// custom TLS, extra settings and public-CA certificates from automatic updates.
func matchesUnmodifiedBootstrapTLS(candidate model.Tls, spec legacyBootstrapOnlyTLSSpec) bool {
	if candidate.Name != spec.name && candidate.Name != bootstrapOnlySelfSignedTLSName(spec.name) {
		return false
	}
	if !matchesLegacyBootstrapOnlyTLS(candidate, spec) {
		return false
	}
	var server, client map[string]interface{}
	if json.Unmarshal(candidate.Server, &server) != nil || json.Unmarshal(candidate.Client, &client) != nil {
		return false
	}
	pemBytes := func(value interface{}) []byte {
		values := value.([]interface{})
		lines := make([]string, len(values))
		for i, v := range values {
			lines[i] = v.(string)
		}
		return []byte(strings.Join(lines, "\n"))
	}
	pair, err := stdtls.X509KeyPair(pemBytes(server["certificate"]), pemBytes(server["key"]))
	if err != nil || len(pair.Certificate) != 1 {
		return false
	}
	cert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil || cert.IsCA || cert.PublicKeyAlgorithm != x509.RSA ||
		!bytes.Equal(cert.RawIssuer, cert.RawSubject) ||
		cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature) != nil ||
		cert.Subject.CommonName != spec.serverName ||
		!reflect.DeepEqual(cert.DNSNames, []string{spec.serverName}) ||
		len(cert.IPAddresses) != 0 || len(cert.URIs) != 0 || len(cert.EmailAddresses) != 0 ||
		cert.KeyUsage != x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature ||
		!reflect.DeepEqual(cert.ExtKeyUsage, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}) {
		return false
	}
	expected, err := buildCertificateTLS(candidate.Name, spec.serverName, spec.alpn, pemBytes(server["certificate"]), pemBytes(server["key"]), true)
	if err != nil {
		return false
	}
	var expectedServer, expectedClient map[string]interface{}
	_ = json.Unmarshal(expected.Server, &expectedServer)
	_ = json.Unmarshal(expected.Client, &expectedClient)
	return reflect.DeepEqual(server, expectedServer) && reflect.DeepEqual(client, expectedClient)
}

// Only S_UI_DEFAULT_TLS_MIGRATE_BOOTSTRAP=true opts in. TLS and dependent
// outbounds are updated atomically; ports and user assignments are untouched.
// Recognize both historical defaults and the reserved bootstrap identity.
func bootstrapTLSMigrationSpecs() []legacyBootstrapOnlyTLSSpec {
	specs := append([]legacyBootstrapOnlyTLSSpec(nil), legacyBootstrapOnlyTLSSpecs...)
	for _, spec := range legacyBootstrapOnlyTLSSpecs {
		spec.serverName = defaultBootstrapTLSName
		specs = append(specs, spec)
	}
	return specs
}

func migrateDefaultProductionTLS(database *gorm.DB, config *defaultProductionTLS) error {
	return database.Transaction(func(tx *gorm.DB) error {
		for _, spec := range bootstrapTLSMigrationSpecs() {
			var candidates []model.Tls
			if err := tx.Where("name IN ?", []string{spec.name, bootstrapOnlySelfSignedTLSName(spec.name)}).Find(&candidates).Error; err != nil {
				return err
			}
			for _, candidate := range candidates {
				if !matchesUnmodifiedBootstrapTLS(candidate, spec) {
					continue
				}
				replacement, err := buildProductionTLS(spec.name, spec.alpn, config)
				if err != nil {
					return err
				}
				replacement.Id = candidate.Id
				if err := tx.Model(&model.Tls{}).Where("id = ?", candidate.Id).Updates(map[string]interface{}{
					"name": replacement.Name, "server": replacement.Server, "client": replacement.Client,
				}).Error; err != nil {
					return err
				}
				var inbounds []model.Inbound
				if err := tx.Where("tls_id = ?", candidate.Id).Find(&inbounds).Error; err != nil {
					return err
				}
				for _, inbound := range inbounds {
					var outbound map[string]interface{}
					if err := decodeTLSMigrationJSON(inbound.OutJson, &outbound); err != nil {
						return err
					}
					if outbound == nil {
						return fmt.Errorf("inbound %d has null out_json", inbound.Id)
					}
					// Do not regenerate the outbound: ports, transports and extra
					// TLS options may have been customized independently of the template.
					var oldTLS, newTLS map[string]interface{}
					if err := json.Unmarshal(candidate.Client, &oldTLS); err != nil {
						return err
					}
					if err := json.Unmarshal(replacement.Client, &newTLS); err != nil {
						return err
					}
					var oldServer map[string]interface{}
					if err := json.Unmarshal(candidate.Server, &oldServer); err != nil {
						return err
					}
					// FillOutJson includes the bootstrap certificate in client TLS.
					oldTLS["certificate"] = oldServer["certificate"]
					if currentTLS, ok := outbound["tls"].(map[string]interface{}); ok {
						migrateBootstrapTLSFields(currentTLS, oldTLS, newTLS)
					}
					// Address-level overrides participate in subscription exports.
					// Keep hosts/ports, but do not let stale bootstrap TLS override
					// the newly verified template (including nested TLS overrides).
					var addrs []map[string]interface{}
					if len(inbound.Addrs) > 0 {
						if err := decodeTLSMigrationJSON(inbound.Addrs, &addrs); err != nil {
							return err
						}
					}
					for _, addr := range addrs {
						if addr == nil {
							return fmt.Errorf("inbound %d has null address", inbound.Id)
						}
						// Flat and nested options compose one TLS configuration. A
						// custom effective SNI protects both layers from migration.
						effectiveTLS := util.MergeSubscriptionTLS(oldTLS, addr)
						if effectiveTLS["server_name"] != oldTLS["server_name"] {
							continue
						}
						migrateBootstrapTLSFields(addr, oldTLS, newTLS)
						if nested, ok := addr["tls"].(map[string]interface{}); ok {
							migrateBootstrapTLSFields(nested, oldTLS, newTLS)
						}
					}
					if len(inbound.Addrs) > 0 {
						inbound.Addrs, err = json.Marshal(addrs)
						if err != nil {
							return err
						}
					}
					inbound.OutJson, err = json.Marshal(outbound)
					if err != nil {
						return err
					}
					if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Updates(map[string]interface{}{"out_json": inbound.OutJson, "addrs": inbound.Addrs}).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

// Unknown outbound/address extensions may contain integers larger than a
// float64 can represent. Preserve their JSON numbers during targeted updates.
func decodeTLSMigrationJSON(data []byte, value interface{}) error {
	if !json.Valid(data) {
		return fmt.Errorf("invalid TLS migration JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(value)
}

// Update only inherited values. A custom SNI denotes an independently managed
// TLS override; leave it (including its verification policy) entirely untouched.
func migrateBootstrapTLSFields(current, oldTLS, newTLS map[string]interface{}) {
	if name, present := current["server_name"]; present && name != oldTLS["server_name"] {
		return
	}
	for key, oldValue := range oldTLS {
		value, present := current[key]
		if !present || !reflect.DeepEqual(value, oldValue) {
			continue
		}
		if replacement, present := newTLS[key]; present {
			current[key] = replacement
		} else {
			delete(current, key)
		}
	}
}
