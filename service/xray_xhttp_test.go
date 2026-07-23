package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
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
}

func TestXrayXHTTPConfigShape(t *testing.T) {
	inbound := model.XrayXHTTPInbound{
		Enable: true, Tag: "vless-reality-xhttp-2088", Listen: "0.0.0.0", Port: 2088,
		ServerName: "www.example.com", Dest: "www.example.com:443", PrivateKey: "private-key",
		PublicKey: "public-key", ShortID: "0123456789abcdef", Fingerprint: "chrome", Path: "/xhttp", Mode: "auto",
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
		ServerName: "www.example.com", Dest: "www.example.com:443", PrivateKey: "private-key",
		PublicKey: "public-key", ShortID: "0123456789abcdef", Fingerprint: "chrome", Transport: "tcp",
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
func TestValidateXrayXHTTPInboundRejectsUnsafeValues(t *testing.T) {
	base := model.XrayXHTTPInbound{Tag: "xhttp", Listen: "0.0.0.0", Port: 2088, ServerName: "example.com", Dest: "example.com:443", PrivateKey: "private", PublicKey: "public", ShortID: "abcd", Fingerprint: "chrome", Path: "/xhttp", Mode: "auto"}
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

func TestWaitForXrayXHTTPStartup(t *testing.T) {
	exited := make(chan struct{})
	close(exited)
	if err := waitForXrayXHTTPStartup(exited, time.Millisecond); err == nil {
		t.Fatal("immediately exited sidecar was reported as started")
	}

	running := make(chan struct{})
	if err := waitForXrayXHTTPStartup(running, time.Millisecond); err != nil {
		t.Fatalf("running sidecar was rejected: %v", err)
	}
	close(running)
}
