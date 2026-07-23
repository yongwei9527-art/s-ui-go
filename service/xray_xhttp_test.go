package service

import (
	"encoding/json"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/op/go-logging"
	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/internal/xrayruntime"
	"github.com/yongwei9527-art/s-ui-go/logger"
)

const (
	testXrayRealityPrivateKey = "aGSYystUbf59_9_6LKRxD27rmSW_-2_nyd9YG_Gwbks"
	testXrayRealityPublicKey  = "E59WjnvZcQMu7tR7_BgyhycuEdBS-CtKxfImRCdAvFM"
)

// TestFreshInstallAutoEnabledXrayRealityTemplates exercises the same database
// initialization path used by install-linux.sh after it exports
// SUI_AUTOCONFIGURE_XRAY_REALITY. It protects the ready-to-use package promise:
// both Xray Reality transports must be enabled, renderable, and subscribable.
func TestFreshInstallAutoEnabledXrayRealityTemplates(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_AUTOCONFIGURE_XRAY_REALITY", "1")
	t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", "test-only-initial-password")
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("initialize fresh database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := database.GetDB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	var inbounds []model.XrayXHTTPInbound
	if err := database.GetDB().Order("id").Find(&inbounds).Error; err != nil {
		t.Fatalf("load seeded Xray inbounds: %v", err)
	}
	if len(inbounds) != 2 {
		t.Fatalf("seeded Xray inbound count = %d, want 2", len(inbounds))
	}
	seen := map[string]bool{}
	for _, inbound := range inbounds {
		if !inbound.Enable {
			t.Fatalf("seeded %s inbound %q is disabled", inbound.Transport, inbound.Tag)
		}
		seen[inbound.Transport] = true
	}
	if !seen["tcp"] || !seen["xhttp"] {
		t.Fatalf("seeded transports = %#v, want tcp and xhttp", seen)
	}

	sidecar := XrayXHTTPService{}
	generated, enabled, err := sidecar.BuildConfig()
	if err != nil {
		t.Fatalf("build seeded Xray config: %v", err)
	}
	if !enabled {
		t.Fatal("seeded Xray config is not enabled")
	}
	text := string(generated)
	for _, want := range []string{`"network": "tcp"`, `"network": "xhttp"`, `"flow": "xtls-rprx-vision"`, `"path": "/xhttp"`, `"mode": "auto"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("seeded Xray config is missing %s: %s", want, text)
		}
	}
	embedded, err := xrayruntime.New(generated)
	if err != nil {
		t.Fatalf("embedded Xray source rejected the generated default config: %v", err)
	}
	if err := embedded.Close(); err != nil {
		t.Fatalf("close prepared embedded Xray instance: %v", err)
	}

	var client model.Client
	if err := database.GetDB().Where("name = ?", "default-user").First(&client).Error; err != nil {
		t.Fatalf("load default client: %v", err)
	}
	links, err := sidecar.LinksForClient(client.Id, "203.0.113.10")
	if err != nil {
		t.Fatalf("create default client links: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("default client Xray link count = %d, want 2", len(links))
	}
	joined := strings.Join(links, "\n")
	for _, want := range []string{"type=tcp", "flow=xtls-rprx-vision", "type=xhttp", "path=%2Fxhttp", "mode=auto"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("default client links are missing %q: %s", want, joined)
		}
	}

	outbounds, err := sidecar.SubscriptionOutboundsForClient(client.Id, "203.0.113.10")
	if err != nil {
		t.Fatalf("create default client structured outbounds: %v", err)
	}
	if len(outbounds) != 2 {
		t.Fatalf("default client structured outbound count = %d, want 2", len(outbounds))
	}
	byTransport := map[string]map[string]interface{}{}
	for _, outbound := range outbounds {
		if transport, ok := outbound["transport"].(map[string]interface{}); ok {
			byTransport[transport["type"].(string)] = outbound
		} else {
			byTransport["tcp"] = outbound
		}
	}
	if byTransport["tcp"]["flow"] != "xtls-rprx-vision" {
		t.Fatalf("TCP structured outbound is missing Vision flow: %#v", byTransport["tcp"])
	}
	xhttpTransport := byTransport["xhttp"]["transport"].(map[string]interface{})
	if xhttpTransport["path"] != "/xhttp" || xhttpTransport["mode"] != "auto" {
		t.Fatalf("XHTTP structured transport = %#v", xhttpTransport)
	}
}

func TestXraySaveFailureRestoresDatabaseAndPreviousRuntime(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", "test-only-initial-password")
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("initialize database: %v", err)
	}
	sidecar := &XrayXHTTPService{}
	t.Cleanup(func() {
		_ = sidecar.Stop()
		if sqlDB, err := database.GetDB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	var client model.Client
	if err := database.GetDB().Where("name = ?", "default-user").First(&client).Error; err != nil {
		t.Fatalf("load default client: %v", err)
	}
	oldPort := reserveAndReleaseTCPPort(t)
	newPort := reserveAndReleaseTCPPort(t)
	for newPort == oldPort {
		newPort = reserveAndReleaseTCPPort(t)
	}
	inbound := model.XrayXHTTPInbound{
		Enable: true, Tag: "rollback-reality-tcp", Listen: "127.0.0.1", Port: uint16(oldPort),
		ServerName: "www.example.com", Dest: "www.example.com:443",
		PrivateKey: testXrayRealityPrivateKey, PublicKey: testXrayRealityPublicKey,
		ShortID: "0123456789abcdef", Fingerprint: "chrome", Transport: "tcp",
	}
	if err := sidecar.Save(XrayXHTTPSaveRequest{Inbound: inbound, ClientIDs: []uint{client.Id}}); err != nil {
		t.Fatalf("start initial embedded Xray runtime: %v", err)
	}
	var saved model.XrayXHTTPInbound
	if err := database.GetDB().Where("tag = ?", inbound.Tag).First(&saved).Error; err != nil {
		t.Fatalf("load saved inbound: %v", err)
	}
	assertTCPPortOpen(t, oldPort)

	blocker, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(newPort)))
	if err != nil {
		t.Fatalf("occupy replacement port: %v", err)
	}
	defer blocker.Close()

	update := saved
	update.Port = uint16(newPort)
	update.PrivateKey = "" // Exercise private-key preservation on an edit.
	err = sidecar.Save(XrayXHTTPSaveRequest{Inbound: update, ClientIDs: []uint{client.Id}})
	if err == nil {
		t.Fatal("save unexpectedly succeeded on an occupied replacement port")
	}
	if status := sidecar.Status(); status["running"] != true {
		t.Fatalf("previous runtime was not restored: %#v", status)
	}
	var restored model.XrayXHTTPInbound
	if err := database.GetDB().First(&restored, saved.Id).Error; err != nil {
		t.Fatalf("load restored inbound: %v", err)
	}
	if restored.Port != uint16(oldPort) || restored.PrivateKey != testXrayRealityPrivateKey {
		t.Fatalf("database was not restored: %#v", restored)
	}
	assertTCPPortOpen(t, oldPort)
}

func TestXrayRestartRemovesPrivateConfigAfterDisablingLastInbound(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", "test-only-initial-password")
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("initialize database: %v", err)
	}
	sidecar := &XrayXHTTPService{}
	t.Cleanup(func() {
		_ = sidecar.Stop()
		if sqlDB, err := database.GetDB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	var client model.Client
	if err := database.GetDB().Where("name = ?", "default-user").First(&client).Error; err != nil {
		t.Fatalf("load default client: %v", err)
	}
	inbound := model.XrayXHTTPInbound{
		Enable: true, Tag: "private-config-cleanup", Listen: "127.0.0.1", Port: uint16(reserveAndReleaseTCPPort(t)),
		ServerName: "www.example.com", Dest: "www.example.com:443",
		PrivateKey: testXrayRealityPrivateKey, PublicKey: testXrayRealityPublicKey,
		ShortID: "0123456789abcdef", Fingerprint: "chrome", Transport: "tcp",
	}
	if err := sidecar.Save(XrayXHTTPSaveRequest{Inbound: inbound, ClientIDs: []uint{client.Id}}); err != nil {
		t.Fatalf("start embedded Xray runtime: %v", err)
	}
	if _, err := os.Stat(sidecar.configPath()); err != nil {
		t.Fatalf("private Xray config was not written: %v", err)
	}
	if err := database.GetDB().Model(&model.XrayXHTTPInbound{}).Where("tag = ?", inbound.Tag).Update("enable", false).Error; err != nil {
		t.Fatalf("disable Xray inbound: %v", err)
	}
	if err := sidecar.Restart(); err != nil {
		t.Fatalf("restart after disabling last inbound: %v", err)
	}
	if _, err := os.Stat(sidecar.configPath()); !os.IsNotExist(err) {
		t.Fatalf("private Xray config should be removed after disable, stat err = %v", err)
	}
}
func reserveAndReleaseTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve TCP port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release TCP port: %v", err)
	}
	return port
}

func assertTCPPortOpen(t *testing.T, port int) {
	t.Helper()
	conn, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	if err != nil {
		t.Fatalf("TCP port %d is not open: %v", port, err)
	}
	_ = conn.Close()
}

func TestXrayXHTTPConfigShape(t *testing.T) {
	inbound := model.XrayXHTTPInbound{
		Enable: true, Tag: "vless-reality-xhttp-2088", Listen: "0.0.0.0", Port: 2088,
		ServerName: "www.example.com", Dest: "www.example.com:443", PrivateKey: testXrayRealityPrivateKey,
		PublicKey: testXrayRealityPublicKey, ShortID: "0123456789abcdef", Fingerprint: "chrome", Path: "/xhttp", Mode: "auto",
	}
	if err := validateXrayXHTTPInbound(inbound); err != nil {
		t.Fatalf("valid inbound rejected: %v", err)
	}
	config := xrayInboundConfig(inbound, []map[string]string{{"id": "test-uuid", "email": "default-user"}})
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{`"network":"xhttp"`, `"security":"reality"`, `"path":"/xhttp"`, `"id":"test-uuid"`} {
		if !strings.Contains(text, want) {
			t.Errorf("generated config missing %s: %s", want, text)
		}
	}
}

func TestXrayTCPVisionConfigShape(t *testing.T) {
	inbound := model.XrayXHTTPInbound{
		Enable: true, Tag: "vless-reality-tcp-10443", Listen: "0.0.0.0", Port: 10443,
		ServerName: "www.example.com", Dest: "www.example.com:443", PrivateKey: testXrayRealityPrivateKey,
		PublicKey: testXrayRealityPublicKey, ShortID: "0123456789abcdef", Fingerprint: "chrome", Transport: "tcp",
	}
	if err := validateXrayXHTTPInbound(inbound); err != nil {
		t.Fatalf("valid TCP Vision inbound rejected: %v", err)
	}
	config := xrayInboundConfig(inbound, []map[string]string{xrayClientConfig(inbound.Transport, "test-uuid", "default-user")})
	raw, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{`"network":"tcp"`, `"flow":"xtls-rprx-vision"`, `"security":"reality"`} {
		if !strings.Contains(text, want) {
			t.Errorf("generated TCP Vision config missing %s: %s", want, text)
		}
	}
	if strings.Contains(text, `"xhttpSettings"`) || strings.Contains(text, `"path"`) || strings.Contains(text, `"mode"`) {
		t.Errorf("TCP Vision config unexpectedly contains XHTTP fields: %s", text)
	}
}
func TestXrayTCPVisionLinkShape(t *testing.T) {
	inbound := model.XrayXHTTPInbound{
		Tag: "tcp-vision", Port: 10443, ServerName: "www.example.com", PublicKey: "public-key",
		ShortID: "0123456789abcdef", Fingerprint: "chrome", Transport: "tcp", Path: "/must-not-appear", Mode: "auto",
	}
	link := xrayVLESSLink(inbound, "test-uuid", "203.0.113.10")
	for _, want := range []string{"type=tcp", "flow=xtls-rprx-vision", "vless://test-uuid@203.0.113.10:10443"} {
		if !strings.Contains(link, want) {
			t.Errorf("TCP Vision link missing %q: %s", want, link)
		}
	}
	for _, unwanted := range []string{"path=", "mode="} {
		if strings.Contains(link, unwanted) {
			t.Errorf("TCP Vision link unexpectedly contains %q: %s", unwanted, link)
		}
	}
}

func TestXrayXHTTPLinkShape(t *testing.T) {
	inbound := model.XrayXHTTPInbound{
		Tag: "xhttp", Port: 2088, ServerName: "www.example.com", PublicKey: "public-key",
		ShortID: "0123456789abcdef", Fingerprint: "chrome", Transport: "xhttp", Path: "/xhttp", Mode: "auto",
	}
	link := xrayVLESSLink(inbound, "test-uuid", "")
	for _, want := range []string{"type=xhttp", "path=%2Fxhttp", "mode=auto", "vless://test-uuid@www.example.com:2088"} {
		if !strings.Contains(link, want) {
			t.Errorf("XHTTP link missing %q: %s", want, link)
		}
	}
	if strings.Contains(link, "flow=xtls-rprx-vision") {
		t.Errorf("XHTTP link unexpectedly contains Vision flow: %s", link)
	}
}

func TestXraySubscriptionOutboundShape(t *testing.T) {
	inbound := model.XrayXHTTPInbound{
		Tag: "xhttp", Port: 2088, ServerName: "www.example.com", PublicKey: "public-key",
		ShortID: "0123456789abcdef", Fingerprint: "chrome", Transport: "xhttp", Path: "/xhttp", Mode: "stream-up",
	}
	outbound := xraySubscriptionOutbound(inbound, "test-uuid", "2001:db8::1")
	if outbound["type"] != "vless" || outbound["server"] != "2001:db8::1" || outbound["uuid"] != "test-uuid" || outbound["udp"] != true {
		t.Fatalf("unexpected structured Xray outbound or missing UDP capability: %#v", outbound)
	}
	transport, ok := outbound["transport"].(map[string]interface{})
	if !ok || transport["type"] != "xhttp" || transport["path"] != "/xhttp" || transport["mode"] != "stream-up" {
		t.Fatalf("unexpected XHTTP transport: %#v", outbound["transport"])
	}
	tls, ok := outbound["tls"].(map[string]interface{})
	if !ok || tls["server_name"] != "www.example.com" {
		t.Fatalf("unexpected Reality TLS settings: %#v", outbound["tls"])
	}
	reality, ok := tls["reality"].(map[string]interface{})
	if !ok || reality["public_key"] != "public-key" || reality["short_id"] != "0123456789abcdef" {
		t.Fatalf("unexpected Reality options: %#v", tls["reality"])
	}
}

func TestXrayStatusReportsEmbeddedSourceRuntime(t *testing.T) {
	status := (&XrayXHTTPService{}).Status()
	if status["engine"] != "embedded" || status["sourceIntegrated"] != true {
		t.Fatalf("Xray status does not report source-integrated runtime: %#v", status)
	}
	version, ok := status["version"].(string)
	if !ok || strings.TrimSpace(version) == "" {
		t.Fatalf("Xray status has no embedded source version: %#v", status)
	}
	if status["configured"] != true {
		t.Fatalf("embedded Xray runtime unexpectedly requires an external binary: %#v", status)
	}
}

func TestValidateXrayXHTTPInboundRejectsUnsafeValues(t *testing.T) {
	base := model.XrayXHTTPInbound{Tag: "xhttp", Listen: "0.0.0.0", Port: 2088, ServerName: "example.com", Dest: "example.com:443", PrivateKey: testXrayRealityPrivateKey, PublicKey: testXrayRealityPublicKey, ShortID: "abcd", Fingerprint: "chrome", Path: "/xhttp", Mode: "auto"}
	badPath := base
	badPath.Path = "xhttp"
	if err := validateXrayXHTTPInbound(badPath); err == nil {
		t.Fatal("path without leading slash was accepted")
	}
	badDest := base
	badDest.Dest = "example.com"
	if err := validateXrayXHTTPInbound(badDest); err == nil {
		t.Fatal("destination without port was accepted")
	}
	blankDestHost := base
	blankDestHost.Dest = ":443"
	if err := validateXrayXHTTPInbound(blankDestHost); err == nil {
		t.Fatal("destination without a host was accepted")
	}
	badMode := base
	badMode.Mode = "unsupported"
	if err := validateXrayXHTTPInbound(badMode); err == nil {
		t.Fatal("unsupported XHTTP mode was accepted")
	}
	badShortID := base
	badShortID.ShortID = "not-hex"
	if err := validateXrayXHTTPInbound(badShortID); err == nil {
		t.Fatal("non-hex Reality short ID was accepted")
	}
	badPort := base
	badPort.Dest = "example.com:0"
	if err := validateXrayXHTTPInbound(badPort); err == nil {
		t.Fatal("zero Reality destination port was accepted")
	}
	badTransport := base
	badTransport.Transport = "grpc"
	if err := validateXrayXHTTPInbound(badTransport); err == nil {
		t.Fatal("unsupported Xray Reality transport was accepted")
	}
	badFingerprint := base
	badFingerprint.Fingerprint = "not-a-real-fingerprint"
	if err := validateXrayXHTTPInbound(badFingerprint); err == nil || !strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("unsupported Reality fingerprint error = %v", err)
	}
	badPublicKey := base
	badPublicKey.PublicKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	if err := validateXrayXHTTPInbound(badPublicKey); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched Reality key pair error = %v", err)
	}
}

func TestFormatXrayXHTTPLinkHost(t *testing.T) {
	if got := formatLinkHost("2001:db8::1"); got != "[2001:db8::1]" {
		t.Fatalf("IPv6 host = %q", got)
	}
	if got := formatLinkHost("example.com"); got != "example.com" {
		t.Fatalf("hostname = %q", got)
	}
}

func TestXrayXHTTPListenerOverlap(t *testing.T) {
	cases := []struct {
		left, right string
		want        bool
	}{
		{"0.0.0.0", "127.0.0.1", true},
		{"::", "::1", true},
		{"127.0.0.1", "127.0.0.1", true},
		{"127.0.0.1", "127.0.0.2", false},
	}
	for _, tc := range cases {
		if got := listenerAddressesOverlap(tc.left, tc.right); got != tc.want {
			t.Errorf("listenerAddressesOverlap(%q, %q) = %v, want %v", tc.left, tc.right, got, tc.want)
		}
	}
}

func TestSingBoxInboundUDPOnly(t *testing.T) {
	if !singBoxInboundUDPOnly("hysteria2") || !singBoxInboundUDPOnly("tuic") {
		t.Fatal("UDP-only inbound types must be recognized")
	}
	if singBoxInboundUDPOnly("vless") {
		t.Fatal("VLESS must be treated as TCP-capable")
	}
}
