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
