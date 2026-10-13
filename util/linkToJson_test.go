package util

import (
	"encoding/json"
	"reflect"
	"strings"
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

func TestGetOutboundSupportedLinksTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		uri   string
		index int
		want  map[string]interface{}
	}{
		{
			name:  "vless Reality keeps TLS and gRPC fields",
			uri:   "vless://11111111-1111-1111-1111-111111111111@reality.example.com:8443?security=reality&pbk=public-key&sid=abcd&sni=www.example.com&fp=chrome&type=grpc&serviceName=grpc-service#reality",
			index: 2,
			want: map[string]interface{}{
				"type": "vless", "tag": "2.reality", "server": "reality.example.com", "server_port": 8443,
				"tls": map[string]interface{}{
					"enabled": true, "server_name": "www.example.com",
					"reality": map[string]interface{}{"enabled": true, "public_key": "public-key", "short_id": "abcd"},
					"utls":    map[string]interface{}{"enabled": true, "fingerprint": "chrome"},
				},
				"transport": map[string]interface{}{"type": "grpc", "service_name": "grpc-service"},
			},
		},
		{
			name:  "hysteria2 keeps obfuscation hopping and TLS fields",
			uri:   "hysteria2://hy2-password@hy2.example.com:8443?obfs=salamander&obfs-password=obfs-secret&mport=443-445,8443&hopInterval=30s&downmbps=100&upmbps=20&allowInsecure=true#hy2",
			index: 0,
			want: map[string]interface{}{
				"type": "hysteria2", "tag": "hy2", "server": "hy2.example.com", "server_port": 8443, "password": "hy2-password",
				"down_mbps": 100, "up_mbps": 20, "server_ports": []string{"443:445", "8443"}, "hop_interval": "30s",
				"obfs": map[string]interface{}{"type": "salamander", "password": "obfs-secret"},
				"tls":  map[string]interface{}{"enabled": true, "insecure": true},
			},
		},
		{
			name:  "tuic accepts alias query names",
			uri:   "tuic://22222222-2222-2222-2222-222222222222:tuic-password@tuic.example.com:10443?congestionControl=bbr&udpRelayMode=native&allow_insecure=true&disableSNI=true#tuic",
			index: 0,
			want: map[string]interface{}{
				"type": "tuic", "tag": "tuic", "server": "tuic.example.com", "server_port": 10443,
				"uuid": "22222222-2222-2222-2222-222222222222", "password": "tuic-password",
				"congestion_control": "bbr", "udp_relay_mode": "native",
				"tls": map[string]interface{}{"enabled": true, "insecure": true, "disable_sni": true},
			},
		},
		{
			name:  "shadowsocks parses credentials and plugin options",
			uri:   "ss://aes-128-gcm:shadow-password@ss.example.com:8388?plugin=v2ray-plugin%3Bmode%3Dwebsocket%3Bhost%3Dcdn.example.com#ss",
			index: 0,
			want: map[string]interface{}{
				"type": "shadowsocks", "tag": "ss", "server": "ss.example.com", "server_port": 8388,
				"method": "aes-128-gcm", "password": "shadow-password", "plugin": "v2ray-plugin", "plugin_opts": "mode=websocket;host=cdn.example.com",
			},
		},
		{
			name:  "naive quic keeps TLS options and fallback tag",
			uri:   "naive+quic://naive-user:naive-password@naive.example.com:9443?peer=cdn.example.com&insecure=true&alpn=h3,h2",
			index: 0,
			want: map[string]interface{}{
				"type": "naive", "tag": "naive-0", "server": "naive.example.com", "server_port": 9443, "quic": true,
				"username": "naive-user", "password": "naive-password",
				"tls": map[string]interface{}{"enabled": true, "server_name": "cdn.example.com", "insecure": true, "alpn": []string{"h3", "h2"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outbound, tag, err := GetOutbound(tt.uri, tt.index)
			if err != nil {
				t.Fatalf("GetOutbound(%q) returned error: %v", tt.uri, err)
			}
			if outbound == nil {
				t.Fatal("GetOutbound returned a nil outbound")
			}
			if tag != tt.want["tag"] {
				t.Fatalf("tag = %q, want %q", tag, tt.want["tag"])
			}
			assertMapContains(t, *outbound, tt.want)
		})
	}
}

func TestGetOutboundUnsupportedLinksReturnNoOutbound(t *testing.T) {
	t.Parallel()

	for _, uri := range []string{
		"juicity://token@example.com:443#unsupported",
		"shadowtls://password@example.com:443#unsupported",
		"%invalid-url",
	} {
		t.Run(uri, func(t *testing.T) {
			outbound, tag, err := GetOutbound(uri, 0)
			if err == nil || !strings.Contains(err.Error(), "Unsupported link format") {
				t.Fatalf("GetOutbound(%q) error = %v, want unsupported link format", uri, err)
			}
			if outbound != nil || tag != "" {
				t.Fatalf("GetOutbound(%q) = (%#v, %q), want (nil, empty tag)", uri, outbound, tag)
			}
		})
	}
}

func assertMapContains(t *testing.T, actual, want map[string]interface{}) {
	t.Helper()
	for key, wantValue := range want {
		actualValue, ok := actual[key]
		if !ok {
			t.Fatalf("missing %q in %#v", key, actual)
		}
		if wantMap, ok := wantValue.(map[string]interface{}); ok {
			actualMap, ok := actualValue.(map[string]interface{})
			if !ok {
				t.Fatalf("%q = %#v, want map %#v", key, actualValue, wantMap)
			}
			assertMapContains(t, actualMap, wantMap)
			continue
		}
		if !reflect.DeepEqual(actualValue, wantValue) {
			t.Fatalf("%q = %#v, want %#v", key, actualValue, wantValue)
		}
	}
}

func TestNaiveLinkWithoutPortKeepsHost(t *testing.T) {
	for _, host := range []string{"example.test", "[2001:db8::1]"} {
		out, _, err := GetOutbound("naive+https://user:pass@"+host+"#node", 0)
		if err != nil {
			t.Fatal(err)
		}
		if (*out)["server"] != strings.Trim(host, "[]") || (*out)["server_port"] != 443 {
			t.Fatalf("invalid naive target: %#v", *out)
		}
	}
}
