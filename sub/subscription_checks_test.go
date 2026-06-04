package sub

import "testing"

func TestSubscriptionDNSHijackRuleRequiresTCPAndUDPForPort53(t *testing.T) {
	udpOnly := []interface{}{
		map[string]interface{}{
			"port":    53,
			"network": []interface{}{"udp"},
			"action":  "hijack-dns",
		},
	}
	if hasDNSHijackRule(udpOnly) {
		t.Fatal("udp-only port 53 hijack should not pass subscription DNS hijack check")
	}

	tcpUDP := []interface{}{
		map[string]interface{}{
			"port":    53,
			"network": []interface{}{"tcp", "udp"},
			"action":  "hijack-dns",
		},
	}
	if !hasDNSHijackRule(tcpUDP) {
		t.Fatal("tcp+udp port 53 hijack should pass subscription DNS hijack check")
	}

	splitTCPUDP := []interface{}{
		map[string]interface{}{
			"port":    53,
			"network": []interface{}{"tcp"},
			"action":  "hijack-dns",
		},
		map[string]interface{}{
			"port":    53,
			"network": []interface{}{"udp"},
			"action":  "hijack-dns",
		},
	}
	if !hasDNSHijackRule(splitTCPUDP) {
		t.Fatal("split tcp+udp port 53 hijack should pass subscription DNS hijack check")
	}

	fractionalPort := []interface{}{
		map[string]interface{}{
			"port":    53.5,
			"network": []interface{}{"tcp", "udp"},
			"action":  "hijack-dns",
		},
	}
	if hasDNSHijackRule(fractionalPort) {
		t.Fatal("fractional port should not pass subscription DNS hijack check")
	}
}

func TestSubscriptionDNSHijackRuleAllowsProtocolDNS(t *testing.T) {
	rules := []interface{}{
		map[string]interface{}{
			"protocol": []interface{}{"dns"},
			"action":   "hijack-dns",
		},
	}
	if !hasDNSHijackRule(rules) {
		t.Fatal("protocol dns hijack should pass subscription DNS hijack check")
	}
}

func TestSubscriptionWarningsExposeCopyAndCatchECH(t *testing.T) {
	report := NewSubscriptionCheckReport()
	report.CheckTransportDNSLinkage("vless", nil, map[string]interface{}{
		"tag": "ech-empty",
		"tls": map[string]interface{}{
			"enabled": true,
			"ech": map[string]interface{}{
				"enabled": true,
			},
		},
	})

	warnings := report.Warnings()
	if len(warnings) < 2 {
		t.Fatalf("expected TLS/SNI and ECH warnings, got %#v", warnings)
	}
	warnings[0] = "mutated"
	if report.Warnings()[0] == "mutated" {
		t.Fatal("Warnings should return a copy")
	}
}

func TestSubscriptionWarningsCatchWebSocketEarlyDataGap(t *testing.T) {
	report := NewSubscriptionCheckReport()
	report.CheckTransportDNSLinkage("vmess", nil, map[string]interface{}{
		"tag": "ws-early-data",
		"transport": map[string]interface{}{
			"type":           "ws",
			"host":           "example.com",
			"max_early_data": 2048,
		},
	})

	warnings := report.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("expected one warning, got %#v", warnings)
	}
	if warnings[0] != "vmess outbound \"ws-early-data\" sets WebSocket max_early_data without early_data_header_name" {
		t.Fatalf("unexpected warning: %#v", warnings[0])
	}
}
