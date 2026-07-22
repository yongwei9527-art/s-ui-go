package service

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yongwei9527-art/s-ui-go/database/model"
)

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
