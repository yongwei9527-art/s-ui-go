package sub

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestClashCompatibilityWarningsSkipUnsupportedOutboundsOnceInSourceOrder(t *testing.T) {
	outbounds := []map[string]interface{}{
		{
			"type":        "vless",
			"tag":         "supported-vless",
			"server":      "example.com",
			"server_port": 443,
			"uuid":        "00000000-0000-0000-0000-000000000000",
		},
		{
			"type":        "naive",
			"tag":         "naive-primary",
			"server":      "naive.example.com",
			"server_port": 443,
		},
		{
			"type":        "shadowtls",
			"tag":         "shadowtls-fallback",
			"server":      "shadowtls.example.com",
			"server_port": 443,
		},
		{
			"type":        "naive",
			"tag":         "naive-primary",
			"server":      "duplicate.example.com",
			"server_port": 443,
		},
	}

	report := NewSubscriptionCheckReport()
	converted, err := (&ClashService{}).convertToClashMeta(&outbounds, "{}", report)
	if err != nil {
		t.Fatalf("convert Clash config: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(converted), &config); err != nil {
		t.Fatalf("parse converted Clash config: %v", err)
	}
	proxies, ok := config["proxies"].([]interface{})
	if !ok || len(proxies) != 1 {
		t.Fatalf("expected the supported vless proxy only, got %#v", config["proxies"])
	}
	proxy, ok := proxies[0].(map[string]interface{})
	if !ok || proxy["name"] != "supported-vless" || proxy["type"] != "vless" {
		t.Fatalf("supported proxy output changed: %#v", proxies[0])
	}

	warnings := report.Warnings()
	want := []string{
		"Clash/Mihomo subscription skipped outbound \"naive-primary\" (protocol \"naive\"): the target format does not support this protocol; use the sing-box JSON subscription for this node.",
		"Clash/Mihomo subscription skipped outbound \"shadowtls-fallback\" (protocol \"shadowtls\"): the target format does not support this protocol; use the sing-box JSON subscription for this node.",
	}
	if len(warnings) != len(want) {
		t.Fatalf("expected %d warnings, got %#v", len(want), warnings)
	}
	for i := range want {
		if warnings[i] != want[i] {
			t.Fatalf("warning %d = %q, want %q", i, warnings[i], want[i])
		}
	}
}

func TestClashCompatibilityWarningsDoNotReportInternalOutbounds(t *testing.T) {
	outbounds := []map[string]interface{}{
		{"type": "selector", "tag": "proxy"},
		{"type": "urltest", "tag": "auto"},
		{"type": "direct", "tag": "direct"},
	}

	report := NewSubscriptionCheckReport()
	_, err := (&ClashService{}).convertToClashMeta(&outbounds, "{}", report)
	if err != nil {
		t.Fatalf("convert Clash config: %v", err)
	}
	if warnings := report.Warnings(); len(warnings) != 0 {
		t.Fatalf("internal outbounds should not be treated as unsupported nodes: %#v", warnings)
	}
}

func TestClashCompatibilityWarningRecommendsSingBoxJSON(t *testing.T) {
	report := NewSubscriptionCheckReport()
	outbounds := []map[string]interface{}{{"type": "naive", "tag": "naive"}}
	_, err := (&ClashService{}).convertToClashMeta(&outbounds, "{}", report)
	if err != nil {
		t.Fatalf("convert Clash config: %v", err)
	}
	warnings := report.Warnings()
	if len(warnings) != 1 || !strings.Contains(warnings[0], "sing-box JSON subscription") {
		t.Fatalf("warning should recommend sing-box JSON: %#v", warnings)
	}
}

func TestConvertToClashMetaSupportsXrayRealityVisionAndXHTTP(t *testing.T) {
	const (
		uuid      = "11111111-2222-3333-4444-555555555555"
		publicKey = "YOsLhQJv2eVg8QyP5JdHcMru9o7ft2l-z1k4bV6x0WA"
		shortID   = "6ba85179e30d4fc2"
	)
	realityTLS := func() map[string]interface{} {
		return map[string]interface{}{
			"enabled":     true,
			"server_name": "www.microsoft.com",
			"reality": map[string]interface{}{
				"enabled":    true,
				"public_key": publicKey,
				"short_id":   shortID,
			},
			"utls": map[string]interface{}{
				"enabled":     true,
				"fingerprint": "chrome",
			},
		}
	}
	outbounds := []map[string]interface{}{
		{
			"type":        "vless",
			"tag":         "xray-reality-vision",
			"server":      "203.0.113.10",
			"server_port": 10443,
			"uuid":        uuid,
			"udp":         true,
			"flow":        "xtls-rprx-vision",
			"tls":         realityTLS(),
		},
		{
			"type":        "vless",
			"tag":         "xray-reality-xhttp",
			"server":      "203.0.113.10",
			"server_port": 2088,
			"uuid":        uuid,
			"udp":         true,
			"tls":         realityTLS(),
			"transport": map[string]interface{}{
				"type": "xhttp",
				"path": "/s-ui-xhttp",
				"mode": "auto",
			},
		},
	}

	converted, err := (&ClashService{}).convertToClashMeta(&outbounds, "{}", NewSubscriptionCheckReport())
	if err != nil {
		t.Fatalf("convert Xray outbounds to Clash/Mihomo: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(converted), &config); err != nil {
		t.Fatalf("parse converted Clash config: %v", err)
	}
	proxies, ok := config["proxies"].([]interface{})
	if !ok || len(proxies) != 2 {
		t.Fatalf("expected two Xray proxies, got %#v", config["proxies"])
	}

	vision, ok := proxies[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Vision proxy has unexpected shape: %#v", proxies[0])
	}
	if vision["type"] != "vless" || vision["flow"] != "xtls-rprx-vision" || vision["udp"] != true {
		t.Fatalf("Vision protocol, flow, or UDP capability was not preserved: %#v", vision)
	}
	assertClashRealityOptions(t, vision, publicKey, shortID)

	xhttp, ok := proxies[1].(map[string]interface{})
	if !ok {
		t.Fatalf("XHTTP proxy has unexpected shape: %#v", proxies[1])
	}
	if xhttp["type"] != "vless" || xhttp["network"] != "xhttp" || xhttp["udp"] != true {
		t.Fatalf("XHTTP protocol, network, or UDP capability was not preserved: %#v", xhttp)
	}
	xhttpOpts, ok := xhttp["xhttp-opts"].(map[string]interface{})
	if !ok || xhttpOpts["path"] != "/s-ui-xhttp" || xhttpOpts["mode"] != "auto" {
		t.Fatalf("XHTTP options were not preserved: %#v", xhttp["xhttp-opts"])
	}
	assertClashRealityOptions(t, xhttp, publicKey, shortID)

	groups, ok := config["proxy-groups"].([]interface{})
	if !ok || len(groups) != 2 {
		t.Fatalf("expected generated proxy groups, got %#v", config["proxy-groups"])
	}
	for _, groupValue := range groups {
		group, ok := groupValue.(map[string]interface{})
		if !ok {
			t.Fatalf("proxy group has unexpected shape: %#v", groupValue)
		}
		groupProxies, ok := group["proxies"].([]interface{})
		if !ok {
			t.Fatalf("proxy group has no proxy list: %#v", group)
		}
		joined := make([]string, 0, len(groupProxies))
		for _, value := range groupProxies {
			if name, ok := value.(string); ok {
				joined = append(joined, name)
			}
		}
		if !containsString(joined, "xray-reality-vision") || !containsString(joined, "xray-reality-xhttp") {
			t.Fatalf("Xray proxies were not added to group %q: %#v", group["name"], groupProxies)
		}
	}
}

func assertClashRealityOptions(t *testing.T, proxy map[string]interface{}, publicKey, shortID string) {
	t.Helper()
	if proxy["tls"] != true || proxy["servername"] != "www.microsoft.com" || proxy["client-fingerprint"] != "chrome" {
		t.Fatalf("Reality TLS options were not preserved: %#v", proxy)
	}
	reality, ok := proxy["reality-opts"].(map[string]interface{})
	if !ok || reality["public-key"] != publicKey || reality["short-id"] != shortID {
		t.Fatalf("Reality key options were not preserved: %#v", proxy["reality-opts"])
	}
}

func TestConvertToClashMetaKeepsIPv6ServerAsPlainAddress(t *testing.T) {
	outbounds := []map[string]interface{}{{
		"type":        "vless",
		"tag":         "ipv6-reality",
		"server":      "[2001:db8::10]",
		"server_port": 443,
		"uuid":        "11111111-2222-3333-4444-555555555555",
	}}

	converted, err := (&ClashService{}).convertToClashMeta(&outbounds, "{}", NewSubscriptionCheckReport())
	if err != nil {
		t.Fatalf("convert IPv6 Clash config: %v", err)
	}
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(converted), &config); err != nil {
		t.Fatalf("parse IPv6 Clash YAML: %v", err)
	}
	proxies := config["proxies"].([]interface{})
	proxy := proxies[0].(map[string]interface{})
	if got := proxy["server"]; got != "2001:db8::10" {
		t.Fatalf("IPv6 server = %#v, want plain address", got)
	}
}
func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
