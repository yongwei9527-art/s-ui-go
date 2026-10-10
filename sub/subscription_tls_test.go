package sub

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/util"
	"gopkg.in/yaml.v3"
)

func TestSubscriptionTLSPropagation(t *testing.T) {
	for _, protocol := range []string{"trojan", "vmess", "hysteria2", "tuic"} {
		t.Run(protocol, func(t *testing.T) {
			for _, mode := range []string{"flat", "nested", "none"} {
				t.Run(mode, func(t *testing.T) {
					tls := &model.Tls{Server: json.RawMessage(`{"enabled":true,"server_name":"edge.example.test","alpn":["h2","http/1.1"]}`), Client: json.RawMessage(`{"insecure":true}`)}
					addresses := `[{"server":"example.test","server_port":443,"remark":"a","tls":true,"insecure":true},{"server":"example.test","server_port":444,"remark":"b","tls":true,"insecure":false},{"server":"example.test","server_port":445,"remark":"c","tls":true}]`
					if mode == "nested" {
						addresses = `[{"server":"example.test","server_port":443,"remark":"a","tls":{"insecure":false,"server_name":"override.test","alpn":["h3"]}},{"server":"example.test","server_port":444,"remark":"b","tls":true}]`
					}
					if mode == "none" {
						addresses = `[]`
					}
					cached, _ := json.Marshal(map[string]interface{}{"type": protocol, "tag": protocol, "server": "example.test", "server_port": 443, "tls": map[string]interface{}{"enabled": true, "insecure": false}})
					in := &model.Inbound{Type: protocol, Tag: protocol, TlsId: 1, Tls: tls, Options: json.RawMessage(`{"listen_port":443}`), OutJson: cached, Addrs: json.RawMessage(addresses)}
					credentials, _ := json.Marshal(map[string]interface{}{protocol: map[string]interface{}{"password": "secret", "uuid": "11111111-1111-1111-1111-111111111111"}})
					outs, _, err := (&JsonService{}).getOutbounds(credentials, []*model.Inbound{in}, NewSubscriptionCheckReport(), "example.test")
					if err != nil {
						t.Fatal(err)
					}
					links := util.LinkGenerator(credentials, in, "example.test")
					if len(links) != len(*outs) {
						t.Fatalf("URI count %d JSON count %d", len(links), len(*outs))
					}
					clash, err := (&ClashService{}).ConvertToClashMeta(outs, basicClashConfig)
					if err != nil {
						t.Fatal(err)
					}
					var parsed struct {
						Proxies []map[string]interface{} `yaml:"proxies"`
					}
					if err := yaml.Unmarshal([]byte(clash), &parsed); err != nil {
						t.Fatal(err)
					}
					for i, ob := range *outs {
						wantInsecure, wantSNI, wantALPN := true, "edge.example.test", []interface{}{"h2", "http/1.1"}
						if mode == "flat" && i == 1 {
							wantInsecure = false
						}
						if mode == "nested" && i == 0 {
							wantInsecure = false
							wantSNI = "override.test"
							wantALPN = []interface{}{"h3"}
						}
						got := ob["tls"].(map[string]interface{})
						if got["insecure"] != wantInsecure || got["server_name"] != wantSNI || !reflect.DeepEqual(got["alpn"], wantALPN) {
							t.Fatalf("JSON TLS %d: %#v", i, got)
						}
						uriOut, _, err := util.GetOutbound(links[i], 0)
						if err != nil {
							t.Fatalf("parse %s: %v", links[i], err)
						}
						uriTLS := (*uriOut)["tls"].(map[string]interface{})
						if (uriTLS["insecure"] == true) != wantInsecure || uriTLS["server_name"] != wantSNI {
							t.Fatalf("URI TLS: %#v (%s)", uriTLS, links[i])
						}
						raw, _ := json.Marshal(uriTLS["alpn"])
						expected, _ := json.Marshal(wantALPN)
						if string(raw) != string(expected) {
							t.Fatalf("URI ALPN %s != %s", raw, expected)
						}
						proxy := parsed.Proxies[i]
						sniKey := "sni"
						if protocol == "vmess" {
							sniKey = "servername"
						}
						if proxy["skip-cert-verify"] != wantInsecure || proxy[sniKey] != wantSNI || !reflect.DeepEqual(proxy["alpn"], wantALPN) {
							t.Fatalf("Clash TLS: %#v", proxy)
						}
					}
				})
			}
		})
	}
}

func TestLegacyFlatTLSOverridesAndTypedALPN(t *testing.T) {
	base := map[string]interface{}{"enabled": true, "insecure": false, "server_name": "old.test", "alpn": []string{"h2"}}
	addr := map[string]interface{}{"tls": true, "insecure": true, "server_name": "new.test", "alpn": []string{"h3"}}
	merged := util.MergeSubscriptionTLS(base, addr)
	if merged["insecure"] != true || merged["server_name"] != "new.test" || !reflect.DeepEqual(merged["alpn"], []string{"h3"}) {
		t.Fatal(merged)
	}
	if base["insecure"] != false || base["server_name"] != "old.test" {
		t.Fatal("base TLS mutated", base)
	}
	outs := []map[string]interface{}{{"type": "trojan", "tag": "typed-alpn", "server": "example.test", "server_port": 443, "password": "secret", "tls": merged}}
	result, err := (&ClashService{}).ConvertToClashMeta(&outs, basicClashConfig)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Proxies []map[string]interface{} `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsed.Proxies[0]["alpn"], []interface{}{"h3"}) {
		t.Fatal(parsed.Proxies)
	}
}
