package service

import (
	"encoding/json"
	"testing"
)

func TestEnsureRouteDNSHijackStrictForcesRemoteResolver(t *testing.T) {
	route := map[string]interface{}{
		"default_domain_resolver": "local-dns",
		"rules": []interface{}{
			map[string]interface{}{"action": "sniff"},
		},
	}

	if err := EnsureRouteDNSHijack(route, DNSLeakGuardModeStrict); err != nil {
		t.Fatal(err)
	}
	if got := route["default_domain_resolver"]; got != remoteDNSTag {
		t.Fatalf("strict resolver = %v, want %s", got, remoteDNSTag)
	}
	if got := route["auto_detect_interface"]; got != true {
		t.Fatalf("auto_detect_interface = %v, want true", got)
	}
}

func TestEnsureRouteDNSHijackRecommendedPreservesExplicitResolver(t *testing.T) {
	route := map[string]interface{}{
		"default_domain_resolver": "local-dns",
		"rules":                   []interface{}{},
	}

	if err := EnsureRouteDNSHijack(route, DNSLeakGuardModeRecommended); err != nil {
		t.Fatal(err)
	}
	if got := route["default_domain_resolver"]; got != "local-dns" {
		t.Fatalf("recommended resolver = %v, want local-dns", got)
	}
}

func TestEnsureRouteDNSHijackRecommendedFillsMissingResolver(t *testing.T) {
	route := map[string]interface{}{
		"rules": []interface{}{},
	}

	if err := EnsureRouteDNSHijack(route, DNSLeakGuardModeRecommended); err != nil {
		t.Fatal(err)
	}
	if got := route["default_domain_resolver"]; got != remoteDNSTag {
		t.Fatalf("recommended missing resolver = %v, want %s", got, remoteDNSTag)
	}
}

func TestHasPort53HijackRequiresTCPAndUDP(t *testing.T) {
	udpOnly := []interface{}{
		map[string]interface{}{
			"port":    53,
			"network": []interface{}{"udp"},
			"action":  dnsHijackAction,
		},
	}
	if hasPort53Hijack(udpOnly) {
		t.Fatal("udp-only port 53 hijack should not pass")
	}

	tcpUDP := []interface{}{
		map[string]interface{}{
			"port":    53,
			"network": []interface{}{"tcp", "udp"},
			"action":  dnsHijackAction,
		},
	}
	if !hasPort53Hijack(tcpUDP) {
		t.Fatal("tcp+udp port 53 hijack should pass")
	}

	splitTCPUDP := []interface{}{
		map[string]interface{}{
			"port":    53,
			"network": []interface{}{"tcp"},
			"action":  dnsHijackAction,
		},
		map[string]interface{}{
			"port":    53,
			"network": []interface{}{"udp"},
			"action":  dnsHijackAction,
		},
	}
	if !hasPort53Hijack(splitTCPUDP) {
		t.Fatal("split tcp+udp port 53 hijack should pass")
	}

	fractionalPort := []interface{}{
		map[string]interface{}{
			"port":    53.5,
			"network": []interface{}{"tcp", "udp"},
			"action":  dnsHijackAction,
		},
	}
	if hasPort53Hijack(fractionalPort) {
		t.Fatal("fractional port should not pass as port 53")
	}
}

func TestValidateRouteDefaultDomainResolverRejectsUnknownTag(t *testing.T) {
	dns := map[string]interface{}{
		"servers": []interface{}{
			map[string]interface{}{"tag": remoteDNSTag, "type": "tls"},
		},
	}
	route := map[string]interface{}{
		"default_domain_resolver": "missing-dns",
	}
	if err := ValidateRouteDefaultDomainResolver(dns, route); err == nil {
		t.Fatal("expected unknown default_domain_resolver to fail")
	}
}

func TestEnsureDNSLeakGuardCompletesPartialLocalRule(t *testing.T) {
	dns := map[string]interface{}{
		"servers": []interface{}{
			map[string]interface{}{"tag": remoteDNSTag, "type": "tls", "server": "1.1.1.1", "server_port": float64(853)},
			map[string]interface{}{"tag": localDNSTag, "type": "udp", "server": "223.5.5.5", "server_port": float64(53)},
		},
		"rules": []interface{}{
			map[string]interface{}{"domain_suffix": []interface{}{".lan"}, "server": localDNSTag},
		},
	}

	if err := EnsureDNSLeakGuard(dns, DNSLeakGuardModeRecommended, "direct"); err != nil {
		t.Fatal(err)
	}
	rules := dns["rules"].([]interface{})
	if !hasLocalDNSRule(rules) {
		raw, _ := json.Marshal(rules)
		t.Fatalf("local rule was not completed: %s", raw)
	}
}

func TestDefaultConfigReportPassesRecommended(t *testing.T) {
	var coreConfig map[string]json.RawMessage
	if err := json.Unmarshal([]byte(defaultConfig), &coreConfig); err != nil {
		t.Fatal(err)
	}
	if err := ensureCoreDNSLeakGuard(coreConfig, DNSLeakGuardModeRecommended); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(coreConfig)
	if err != nil {
		t.Fatal(err)
	}
	report, err := BuildDNSLeakGuardCheckReport(raw, DNSLeakGuardModeRecommended)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatalf("default recommended report should pass: %+v", report.Checks)
	}
}

func TestDefaultConfigOmitsDirectDNSDetour(t *testing.T) {
	var coreConfig map[string]json.RawMessage
	if err := json.Unmarshal([]byte(defaultConfig), &coreConfig); err != nil {
		t.Fatal(err)
	}
	dnsConfig, err := decodeObjectSection("dns", coreConfig["dns"])
	if err != nil {
		t.Fatal(err)
	}
	servers, err := getObjectSlice(dnsConfig, "servers")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range servers {
		server := item.(map[string]interface{})
		if detour, _ := server["detour"].(string); detour == "direct" {
			t.Fatalf("default DNS server %q must not use direct detour", server["tag"])
		}
	}
}

func TestDefaultDNSLeakGuardConfigDetourPolicy(t *testing.T) {
	directDNS := DefaultDNSLeakGuardConfig("direct")
	directServers := directDNS["servers"].([]interface{})
	for _, item := range directServers {
		server := item.(map[string]interface{})
		if _, ok := server["detour"]; ok {
			t.Fatalf("direct DNS server %q should omit detour", server["tag"])
		}
	}

	proxyDNS := DefaultDNSLeakGuardConfig("proxy")
	proxyServers := proxyDNS["servers"].([]interface{})
	remoteServer := findDNSServer(proxyServers, remoteDNSTag)
	if remoteServer == nil {
		t.Fatal("remote-dns server not found")
	}
	if got := remoteServer["detour"]; got != "proxy" {
		t.Fatalf("remote-dns detour = %v, want proxy", got)
	}
}

func TestEnsureDNSLeakGuardRemovesLegacyDirectDetour(t *testing.T) {
	dns := map[string]interface{}{
		"servers": []interface{}{
			map[string]interface{}{"tag": remoteDNSTag, "type": "tls", "server": "1.1.1.1", "server_port": float64(853), "detour": "direct"},
			map[string]interface{}{"tag": localDNSTag, "type": "udp", "server": "223.5.5.5", "server_port": float64(53)},
		},
	}

	if err := EnsureDNSLeakGuard(dns, DNSLeakGuardModeRecommended, "direct"); err != nil {
		t.Fatal(err)
	}
	remoteServer := findDNSServer(dns["servers"].([]interface{}), remoteDNSTag)
	if remoteServer == nil {
		t.Fatal("remote-dns server not found")
	}
	if _, ok := remoteServer["detour"]; ok {
		t.Fatalf("legacy direct detour should be removed: %+v", remoteServer)
	}
}

func TestEnsureDNSLeakGuardPreservesExplicitProxyDetour(t *testing.T) {
	dns := map[string]interface{}{
		"servers": []interface{}{
			map[string]interface{}{"tag": remoteDNSTag, "type": "tls", "server": "1.1.1.1", "server_port": float64(853), "detour": "proxy"},
			map[string]interface{}{"tag": localDNSTag, "type": "udp", "server": "223.5.5.5", "server_port": float64(53)},
		},
	}

	if err := EnsureDNSLeakGuard(dns, DNSLeakGuardModeRecommended, "direct"); err != nil {
		t.Fatal(err)
	}
	remoteServer := findDNSServer(dns["servers"].([]interface{}), remoteDNSTag)
	if remoteServer == nil {
		t.Fatal("remote-dns server not found")
	}
	if got := remoteServer["detour"]; got != "proxy" {
		t.Fatalf("remote-dns detour = %v, want proxy", got)
	}
}
