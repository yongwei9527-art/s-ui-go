package database

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
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

const defaultTemplateClientName = "default-user"

type seededTLS struct {
	id   uint
	name string
}

func seedDefaultTlsAndInbounds() error {
	var tlsCount int64
	if err := db.Model(&model.Tls{}).Count(&tlsCount).Error; err != nil {
		return err
	}

	var inboundCount int64
	if err := db.Model(&model.Inbound{}).Count(&inboundCount).Error; err != nil {
		return err
	}

	if tlsCount > 0 || inboundCount > 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		seedHost := detectDefaultSeedHost()
		seeded, err := seedDefaultTlsTemplates(tx)
		if err != nil {
			return err
		}

		inbounds, err := seedDefaultInbounds(tx, seeded, seedHost)
		if err != nil {
			return err
		}

		return seedDefaultClient(tx, inbounds, seedHost)
	})
}

func seedDefaultTlsTemplates(tx *gorm.DB) (map[string]seededTLS, error) {
	realityTLS, err := buildRealityTLS("reality-vless", "go.microsoft.com")
	if err != nil {
		return nil, err
	}
	hy2TLS, err := buildSelfSignedTLS("tls-hy2", "tls1.tcm.mmbing.net", []string{"h3"})
	if err != nil {
		return nil, err
	}
	tuicTLS, err := buildSelfSignedTLS("tls-tuic", "snap.licdn.com", []string{"h3"})
	if err != nil {
		return nil, err
	}
	trojanTLS, err := buildSelfSignedTLS("tls-trojan", "snap.licdn.com", []string{"http/1.1"})
	if err != nil {
		return nil, err
	}
	trojanWSTLS, err := buildSelfSignedTLS("tls-trojan-ws", "snap.licdn.com", []string{"http/1.1"})
	if err != nil {
		return nil, err
	}
	vmessWSTLS, err := buildSelfSignedTLS("tls-vmess-ws", "snap.licdn.com", []string{"http/1.1"})
	if err != nil {
		return nil, err
	}
	naiveTLS, err := buildSelfSignedTLS("tls-naive", "snap.licdn.com", []string{"h2", "http/1.1"})
	if err != nil {
		return nil, err
	}
	anyTLSTLS, err := buildSelfSignedTLS("tls-anytls", "snap.licdn.com", []string{"http/1.1"})
	if err != nil {
		return nil, err
	}
	hysteriaTLS, err := buildSelfSignedTLS("tls-hysteria", "tls1.tcm.mmbing.net", []string{"h3"})
	if err != nil {
		return nil, err
	}

	templates := []model.Tls{realityTLS, hy2TLS, tuicTLS, trojanTLS, trojanWSTLS, vmessWSTLS, naiveTLS, anyTLSTLS, hysteriaTLS}
	seeded := make(map[string]seededTLS, len(templates))

	for _, tpl := range templates {
		if err := tx.Create(&tpl).Error; err != nil {
			return nil, err
		}
		seeded[tpl.Name] = seededTLS{id: tpl.Id, name: tpl.Name}
	}

	return seeded, nil
}

func buildRealityTLS(name string, serverName string) (model.Tls, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return model.Tls{}, err
	}
	publicKey := privateKey.PublicKey()
	shortID := randomHex(8)

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

func buildSelfSignedTLS(name string, serverName string, alpn []string) (model.Tls, error) {
	privateKeyPEM, certPEM, err := tls.GenerateCertificate(nil, nil, time.Now, serverName, time.Now().AddDate(1, 0, 0))
	if err != nil {
		return model.Tls{}, err
	}

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
		"insecure":      true,
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
	shadowsocksServerPassword := randomBase64(16)

	inbounds := []json.RawMessage{
		inboundJSON("vless", inboundTag("vless-reality", vlessPort), tlsTemplates["reality-vless"].id, map[string]interface{}{
			"listen":      "0.0.0.0",
			"listen_port": vlessPort,
			"transport":   map[string]interface{}{},
		}, []map[string]interface{}{{"server": seedHost, "server_port": vlessPort, "remark": inboundTag("vless-reality", vlessPort)}}),
		inboundJSON("hysteria2", inboundTag("hy2", hy2Port), tlsTemplates["tls-hy2"].id, map[string]interface{}{
			"listen":                  "0.0.0.0",
			"listen_port":             hy2Port,
			"udp_fragment":            true,
			"udp_timeout":             "5m",
			"ignore_client_bandwidth": true,
		}, []map[string]interface{}{{"server": seedHost, "server_port": hy2Port, "remark": inboundTag("hy2", hy2Port), "tls": true, "insecure": true}}),
		inboundJSON("tuic", inboundTag("tuic", tuicPort), tlsTemplates["tls-tuic"].id, map[string]interface{}{
			"listen":             "0.0.0.0",
			"listen_port":        tuicPort,
			"congestion_control": "cubic",
			"auth_timeout":       "3s",
			"heartbeat":          "10s",
			"udp_fragment":       true,
			"udp_timeout":        "5m",
		}, []map[string]interface{}{{"server": seedHost, "server_port": tuicPort, "remark": inboundTag("tuic", tuicPort), "tls": true, "insecure": true}}),
		inboundJSON("trojan", inboundTag("trojan-tls", trojanPort), tlsTemplates["tls-trojan"].id, map[string]interface{}{
			"listen":      "0.0.0.0",
			"listen_port": trojanPort,
			"transport":   map[string]interface{}{},
		}, []map[string]interface{}{{"server": seedHost, "server_port": trojanPort, "remark": inboundTag("trojan-tls", trojanPort), "tls": true, "insecure": true}}),
		inboundJSON("vmess", inboundTag("vmess-ws-tls", vmessPort), tlsTemplates["tls-vmess-ws"].id, map[string]interface{}{
			"listen":      "0.0.0.0",
			"listen_port": vmessPort,
			"transport": map[string]interface{}{
				"type": "ws",
				"path": "/vmess",
				"headers": map[string]interface{}{
					"Host": "snap.licdn.com",
				},
			},
		}, []map[string]interface{}{{"server": seedHost, "server_port": vmessPort, "remark": inboundTag("vmess-ws-tls", vmessPort), "tls": true, "insecure": true}}),
		inboundJSON("shadowsocks", inboundTag("ss-2022", shadowsocksPort), 0, map[string]interface{}{
			"listen":      "0.0.0.0",
			"listen_port": shadowsocksPort,
			"method":      "2022-blake3-aes-128-gcm",
			"password":    shadowsocksServerPassword,
		}, []map[string]interface{}{{"server": seedHost, "server_port": shadowsocksPort, "remark": inboundTag("ss-2022", shadowsocksPort)}}),
		inboundJSON("naive", inboundTag("naive-tls", naivePort), tlsTemplates["tls-naive"].id, map[string]interface{}{
			"listen":      "0.0.0.0",
			"listen_port": naivePort,
		}, []map[string]interface{}{{"server": seedHost, "server_port": naivePort, "remark": inboundTag("naive-tls", naivePort), "tls": true, "insecure": true}}),
		inboundJSON("anytls", inboundTag("anytls", anyTLSPort), tlsTemplates["tls-anytls"].id, map[string]interface{}{
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
		inboundJSON("hysteria", inboundTag("hysteria-tls", hysteriaPort), tlsTemplates["tls-hysteria"].id, map[string]interface{}{
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
	password := common.Random(10)
	shadowsocksPassword := randomBase64(32)
	shadowsocks16Password := randomBase64(16)
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

func randomHex(byteLen int) string {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return common.Random(byteLen * 2)
	}
	return hex.EncodeToString(buf)
}

func randomBase64(byteLen int) string {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return common.Random(byteLen)
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func pemLines(pem []byte) []string {
	lines := strings.Split(strings.TrimSpace(string(pem)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []string{}
	}
	return lines
}
