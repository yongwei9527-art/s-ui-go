package sub

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/anytls"
	"github.com/sagernet/sing-box/protocol/hysteria"
	"github.com/sagernet/sing-box/protocol/hysteria2"
	"github.com/sagernet/sing-box/protocol/shadowsocks"
	"github.com/sagernet/sing-box/protocol/trojan"
	"github.com/sagernet/sing-box/protocol/tuic"
	"github.com/sagernet/sing-box/protocol/vless"
	"github.com/sagernet/sing-box/protocol/vmess"
	"github.com/sagernet/sing/service"
	"github.com/yongwei9527-art/s-ui-go/database/model"
)

// TestJsonServiceGeneratedOutboundsParseAsSingBoxConfig exercises the database-
// independent portion of JSON subscription generation. It intentionally only
// parses the generated config: no Box is started, so the test never opens a
// listener or makes a network request.
func TestJsonServiceGeneratedOutboundsParseAsSingBoxConfig(t *testing.T) {
	const (
		serverHost   = "subscription.example.test"
		serverKey    = "AAAAAAAAAAAAAAAAAAAAAA"
		clientKey    = "BBBBBBBBBBBBBBBBBBBBBB"
		realityKey   = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		shortID      = "0123456789abcdef"
		clientConfig = `{
			"vless": {"uuid": "11111111-1111-1111-1111-111111111111", "flow": "xtls-rprx-vision"},
			"hysteria2": {"password": "hy2-client-password"},
			"tuic": {"uuid": "11111111-1111-1111-1111-111111111111", "password": "tuic-client-password"},
			"trojan": {"password": "trojan-client-password"},
			"vmess": {"uuid": "11111111-1111-1111-1111-111111111111", "security": "auto"},
			"shadowsocks16": {"password": "BBBBBBBBBBBBBBBBBBBBBB"},
			"naive": {"username": "naive-user", "password": "naive-client-password"},
			"anytls": {"password": "anytls-client-password"},
			"hysteria": {"auth_str": "hysteria-client-password"}
		}`
	)

	tls := map[string]interface{}{
		"enabled":     true,
		"server_name": "edge.example.test",
		"insecure":    true,
		"utls": map[string]interface{}{
			"enabled":     true,
			"fingerprint": "chrome",
		},
	}
	realityTLS := copyJSONMap(t, tls)
	realityTLS["reality"] = map[string]interface{}{
		"enabled":    true,
		"public_key": realityKey,
		"short_id":   shortID,
	}

	inbounds := []*model.Inbound{
		integrationInbound(t, "vless", "vless-reality-443", 1, map[string]interface{}{
			"type": "vless", "tag": "vless-reality", "server": "192.0.2.10", "server_port": 443,
			"tls": realityTLS,
		}),
		integrationInbound(t, "hysteria2", "hy2-443", 1, map[string]interface{}{
			"type": "hysteria2", "tag": "hy2", "server": "192.0.2.11", "server_port": 443,
			"tls": tls,
		}),
		integrationInbound(t, "tuic", "tuic-8443", 1, map[string]interface{}{
			"type": "tuic", "tag": "tuic", "server": "192.0.2.12", "server_port": 8443,
			"congestion_control": "cubic", "udp_relay_mode": "native", "tls": tls,
		}),
		integrationInbound(t, "trojan", "trojan-tls-8443", 1, map[string]interface{}{
			"type": "trojan", "tag": "trojan", "server": "192.0.2.13", "server_port": 8443, "tls": tls,
		}),
		integrationInbound(t, "vmess", "vmess-ws-tls-2053", 1, map[string]interface{}{
			"type": "vmess", "tag": "vmess", "server": "192.0.2.14", "server_port": 2053, "tls": tls,
			"transport": map[string]interface{}{"type": "ws", "path": "/vmess", "headers": map[string]interface{}{"Host": "edge.example.test"}},
		}),
		integrationInbound(t, "shadowsocks", "ss-2022-8388", 0, map[string]interface{}{
			"type": "shadowsocks", "tag": "ss-2022", "server": "192.0.2.15", "server_port": 8388,
			"method": "2022-blake3-aes-128-gcm",
		}, map[string]interface{}{"method": "2022-blake3-aes-128-gcm", "password": serverKey}),
		integrationInbound(t, "naive", "naive-tls-2087", 1, map[string]interface{}{
			"type": "naive", "tag": "naive", "server": "192.0.2.16", "server_port": 2087, "tls": tls,
		}),
		integrationInbound(t, "anytls", "anytls-2097", 1, map[string]interface{}{
			"type": "anytls", "tag": "anytls", "server": "192.0.2.17", "server_port": 2097, "tls": tls,
		}),
		integrationInbound(t, "hysteria", "hysteria-tls-2083", 1, map[string]interface{}{
			"type": "hysteria", "tag": "hysteria", "server": "192.0.2.18", "server_port": 2083,
			"up_mbps": 50, "down_mbps": 100, "tls": tls,
		}),
	}

	service := &JsonService{}
	outbounds, tags, err := service.getOutbounds(json.RawMessage(clientConfig), inbounds, NewSubscriptionCheckReport(), serverHost)
	if err != nil {
		t.Fatalf("getOutbounds returned error: %v", err)
	}
	if len(*outbounds) != len(inbounds) || len(*tags) != len(inbounds) {
		t.Fatalf("generated %d outbounds and %d tags, want %d for each", len(*outbounds), len(*tags), len(inbounds))
	}

	byType := make(map[string]map[string]interface{}, len(*outbounds))
	for _, outbound := range *outbounds {
		protocol, _ := outbound["type"].(string)
		byType[protocol] = outbound
		if server, _ := outbound["server"].(string); server != serverHost {
			t.Errorf("%s server = %q, want seeded hostname replacement %q", protocol, server, serverHost)
		}
	}
	for _, protocol := range []string{"vless", "hysteria2", "tuic", "trojan", "vmess", "shadowsocks", "naive", "anytls", "hysteria"} {
		if _, ok := byType[protocol]; !ok {
			t.Fatalf("generated outbounds missing %s: %#v", protocol, byType)
		}
	}
	if got, _ := byType["shadowsocks"]["password"].(string); got != serverKey+":"+clientKey {
		t.Errorf("shadowsocks 2022 password = %q, want combined server and client keys", got)
	}
	if reality, _ := byType["vless"]["tls"].(map[string]interface{})["reality"].(map[string]interface{}); reality == nil || reality["public_key"] != realityKey {
		t.Errorf("vless Reality settings were not preserved: %#v", byType["vless"]["tls"])
	}

	config, err := json.Marshal(map[string]interface{}{"outbounds": outbounds})
	if err != nil {
		t.Fatalf("marshal generated minimal sing-box config: %v", err)
	}
	var options option.Options
	if err := options.UnmarshalJSONContext(jsonSubscriptionOptionContext(), config); err != nil {
		t.Fatalf("generated JSON subscription outbounds are not parseable as a sing-box config: %v\nconfig:\n%s", err, config)
	}
}

func integrationInbound(t *testing.T, protocol, tag string, tlsID uint, outbound map[string]interface{}, options ...map[string]interface{}) *model.Inbound {
	t.Helper()
	outJSON, err := json.Marshal(outbound)
	if err != nil {
		t.Fatalf("marshal %s outbound: %v", protocol, err)
	}
	inboundOptions := map[string]interface{}{}
	if len(options) > 0 {
		inboundOptions = options[0]
	}
	optionsJSON, err := json.Marshal(inboundOptions)
	if err != nil {
		t.Fatalf("marshal %s inbound options: %v", protocol, err)
	}
	addrsJSON, err := json.Marshal([]map[string]interface{}{{"server": "192.0.2.1", "server_port": 1}})
	if err != nil {
		t.Fatalf("marshal %s addresses: %v", protocol, err)
	}
	return &model.Inbound{
		Type:    protocol,
		Tag:     tag,
		TlsId:   tlsID,
		Addrs:   addrsJSON,
		OutJson: outJSON,
		Options: optionsJSON,
	}
}

func copyJSONMap(t *testing.T, source map[string]interface{}) map[string]interface{} {
	t.Helper()
	data, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("marshal map copy: %v", err)
	}
	var copied map[string]interface{}
	if err := json.Unmarshal(data, &copied); err != nil {
		t.Fatalf("unmarshal map copy: %v", err)
	}
	return copied
}

// jsonSubscriptionOptionContext registers precisely the outbound types covered
// by this test. Naive uses its option-only registry entry so the config can be
// parsed in default builds where the application gates the runtime implementation
// behind the with_naive_outbound build tag.
func jsonSubscriptionOptionContext() context.Context {
	registry := outbound.NewRegistry()
	vless.RegisterOutbound(registry)
	hysteria2.RegisterOutbound(registry)
	tuic.RegisterOutbound(registry)
	trojan.RegisterOutbound(registry)
	vmess.RegisterOutbound(registry)
	shadowsocks.RegisterOutbound(registry)
	outbound.Register[option.NaiveOutboundOptions](registry, "naive", func(context.Context, adapter.Router, log.ContextLogger, string, option.NaiveOutboundOptions) (adapter.Outbound, error) {
		return nil, nil
	})
	anytls.RegisterOutbound(registry)
	hysteria.RegisterOutbound(registry)
	return service.ContextWith[option.OutboundOptionsRegistry](context.Background(), registry)
}
