package util

import (
	"encoding/json"
	"testing"

	"github.com/yongwei9527-art/s-ui-go/database/model"
)

func TestVlessLinkParsesIPv6HostWithoutPort(t *testing.T) {
	out, tag, err := GetOutbound("vless://11111111-1111-1111-1111-111111111111@[2001:db8::1]?security=tls&type=ws&host=example.com&path=%2Fws&maxEarlyData=2048&ed=Sec-WebSocket-Protocol#ipv6", 0)
	if err != nil {
		t.Fatalf("GetOutbound returned error: %v", err)
	}
	if tag != "ipv6" {
		t.Fatalf("unexpected tag: %q", tag)
	}
	if (*out)["server"] != "2001:db8::1" {
		t.Fatalf("unexpected server: %#v", (*out)["server"])
	}
	if (*out)["server_port"] != 443 {
		t.Fatalf("unexpected server_port: %#v", (*out)["server_port"])
	}
	transport, ok := (*out)["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing or wrong type: %#v", (*out)["transport"])
	}
	if transport["max_early_data"] != 2048 {
		t.Fatalf("unexpected max_early_data: %#v", transport["max_early_data"])
	}
	if transport["early_data_header_name"] != "Sec-WebSocket-Protocol" {
		t.Fatalf("unexpected early_data_header_name: %#v", transport["early_data_header_name"])
	}
}

func TestHy2LinkParsesECHAndHopInterval(t *testing.T) {
	out, _, err := GetOutbound("hy2://pass@example.com?mport=443-445,8443&hop_interval=30s&ech=config-a,config-b&ech_pq_signature_schemes_enabled=true#hy2", 0)
	if err != nil {
		t.Fatalf("GetOutbound returned error: %v", err)
	}
	if (*out)["hop_interval"] != "30s" {
		t.Fatalf("unexpected hop_interval: %#v", (*out)["hop_interval"])
	}
	ports, ok := (*out)["server_ports"].([]string)
	if !ok || len(ports) != 2 || ports[0] != "443:445" || ports[1] != "8443" {
		t.Fatalf("unexpected server_ports: %#v", (*out)["server_ports"])
	}
	tls, ok := (*out)["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls missing or wrong type: %#v", (*out)["tls"])
	}
	ech, ok := tls["ech"].(map[string]interface{})
	if !ok {
		t.Fatalf("ech missing or wrong type: %#v", tls["ech"])
	}
	configs, ok := ech["config"].([]string)
	if !ok || len(configs) != 2 || configs[0] != "config-a" || configs[1] != "config-b" {
		t.Fatalf("unexpected ech config: %#v", ech["config"])
	}
	if ech["pq_signature_schemes_enabled"] != true {
		t.Fatalf("unexpected ech pq flag: %#v", ech["pq_signature_schemes_enabled"])
	}
}

func TestTuicLinkOmitsEmptyOptionalStringsAndAppliesTLSAliases(t *testing.T) {
	out, _, err := GetOutbound("tuic://uuid:pass@example.com?allow_insecure=true&disableSNI=true#tuic", 0)
	if err != nil {
		t.Fatalf("GetOutbound returned error: %v", err)
	}
	if _, ok := (*out)["congestion_control"]; ok {
		t.Fatalf("empty congestion_control should be omitted: %#v", (*out)["congestion_control"])
	}
	if _, ok := (*out)["udp_relay_mode"]; ok {
		t.Fatalf("empty udp_relay_mode should be omitted: %#v", (*out)["udp_relay_mode"])
	}
	tls, ok := (*out)["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls missing or wrong type: %#v", (*out)["tls"])
	}
	if tls["insecure"] != true {
		t.Fatalf("allow_insecure alias did not set insecure: %#v", tls["insecure"])
	}
	if tls["disable_sni"] != true {
		t.Fatalf("disableSNI alias did not set disable_sni: %#v", tls["disable_sni"])
	}
}

func TestLinkGeneratorUsesRequestHostForSeededDefaultInbound(t *testing.T) {
	clientConfig := json.RawMessage(`{"vless":{"uuid":"11111111-1111-1111-1111-111111111111"}}`)
	addrs := json.RawMessage(`[{"server":"192.0.2.10","server_port":443,"remark":"seeded"}]`)
	options := json.RawMessage(`{"listen":"0.0.0.0","listen_port":443,"transport":{}}`)
	inbound := &model.Inbound{
		Type:    "vless",
		Tag:     "vless-reality-443",
		Addrs:   addrs,
		Options: options,
	}

	links := LinkGenerator(clientConfig, inbound, "proxy.example.com")
	if len(links) != 1 {
		t.Fatalf("unexpected links: %#v", links)
	}
	out, _, err := GetOutbound(links[0], 0)
	if err != nil {
		t.Fatalf("generated link did not parse: %v", err)
	}
	if (*out)["server"] != "proxy.example.com" {
		t.Fatalf("generated link server = %#v, want proxy.example.com", (*out)["server"])
	}
}
