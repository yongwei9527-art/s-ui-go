package database

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"gorm.io/gorm"
)

func TestSeedDefaultInboundsStructure(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := testDB.AutoMigrate(&model.Tls{}, &model.Inbound{}); err != nil {
		t.Fatalf("migrate TLS and inbound models: %v", err)
	}

	tlsTemplates, err := seedDefaultTlsTemplates(testDB)
	if err != nil {
		t.Fatalf("seedDefaultTlsTemplates() error = %v", err)
	}
	seededInbounds, err := seedDefaultInbounds(testDB, tlsTemplates, "127.0.0.1")
	if err != nil {
		t.Fatalf("seedDefaultInbounds() error = %v", err)
	}

	var inbounds []model.Inbound
	if err := testDB.Preload("Tls").Order("id").Find(&inbounds).Error; err != nil {
		t.Fatalf("load seeded inbounds: %v", err)
	}
	if len(inbounds) != len(seededInbounds) {
		t.Fatalf("persisted inbound count = %d, seeded count = %d", len(inbounds), len(seededInbounds))
	}

	expectedTLSByType := map[string]string{
		"vless":     "reality-vless",
		"hysteria2": bootstrapOnlySelfSignedTLSName("tls-hy2"),
		"tuic":      bootstrapOnlySelfSignedTLSName("tls-tuic"),
		"trojan":    bootstrapOnlySelfSignedTLSName("tls-trojan"),
		"vmess":     bootstrapOnlySelfSignedTLSName("tls-vmess-ws"),
		"naive":     bootstrapOnlySelfSignedTLSName("tls-naive"),
		"anytls":    bootstrapOnlySelfSignedTLSName("tls-anytls"),
		"hysteria":  bootstrapOnlySelfSignedTLSName("tls-hysteria"),
	}
	expectedTypes := map[string]struct{}{
		"vless": {}, "hysteria2": {}, "tuic": {}, "trojan": {}, "vmess": {},
		"shadowsocks": {}, "naive": {}, "anytls": {}, "hysteria": {},
	}
	if len(inbounds) != len(expectedTypes) {
		t.Fatalf("seeded inbound count = %d, want %d", len(inbounds), len(expectedTypes))
	}

	seenTypes := make(map[string]bool, len(expectedTypes))
	seenTags := make(map[string]bool, len(inbounds))
	for _, inbound := range inbounds {
		if _, ok := expectedTypes[inbound.Type]; !ok {
			t.Fatalf("unexpected default inbound type %q", inbound.Type)
		}
		if seenTypes[inbound.Type] {
			t.Fatalf("default inbound type %q was seeded more than once", inbound.Type)
		}
		seenTypes[inbound.Type] = true

		if inbound.Tag == "" {
			t.Fatalf("default %s inbound has an empty tag", inbound.Type)
		}
		if seenTags[inbound.Tag] {
			t.Fatalf("default inbound tag %q is not unique", inbound.Tag)
		}
		seenTags[inbound.Tag] = true

		options := defaultSeedJSONMap(t, inbound.Options, inbound.Type+" Options")
		if port, ok := options["listen_port"].(float64); !ok || port <= 0 {
			t.Fatalf("default %s inbound has invalid listen_port in Options: %#v", inbound.Type, options["listen_port"])
		}

		outbound := defaultSeedJSONMap(t, inbound.OutJson, inbound.Type+" OutJson")
		if outbound["type"] != inbound.Type {
			t.Fatalf("default %s OutJson type = %#v, want %q", inbound.Type, outbound["type"], inbound.Type)
		}
		if outbound["tag"] != inbound.Tag {
			t.Fatalf("default %s OutJson tag = %#v, want %q", inbound.Type, outbound["tag"], inbound.Tag)
		}
		if outbound["server"] != "127.0.0.1" {
			t.Fatalf("default %s OutJson server = %#v, want 127.0.0.1", inbound.Type, outbound["server"])
		}

		expectedTLSName, needsTLS := expectedTLSByType[inbound.Type]
		if !needsTLS {
			if inbound.Type != "shadowsocks" || inbound.TlsId != 0 || inbound.Tls != nil {
				t.Fatalf("default non-TLS inbound %#v unexpectedly has TLS association", inbound)
			}
			if _, hasTLS := outbound["tls"]; hasTLS {
				t.Fatalf("default shadowsocks OutJson unexpectedly contains TLS: %#v", outbound["tls"])
			}
			continue
		}

		expectedTLS, ok := tlsTemplates[expectedTLSName]
		if !ok {
			t.Fatalf("expected TLS template %q is missing", expectedTLSName)
		}
		if inbound.TlsId != expectedTLS.id {
			t.Fatalf("default %s TLS ID = %d, want %d (%s)", inbound.Type, inbound.TlsId, expectedTLS.id, expectedTLSName)
		}
		if inbound.Tls == nil || inbound.Tls.Name != expectedTLSName {
			t.Fatalf("default %s TLS association = %#v, want %q", inbound.Type, inbound.Tls, expectedTLSName)
		}

		serverTLS := defaultSeedJSONMap(t, inbound.Tls.Server, inbound.Type+" TLS server")
		clientTLS := defaultSeedJSONMap(t, inbound.Tls.Client, inbound.Type+" TLS client")
		outboundTLS, ok := outbound["tls"].(map[string]interface{})
		if !ok {
			t.Fatalf("default %s OutJson is missing parseable TLS config: %#v", inbound.Type, outbound["tls"])
		}

		if inbound.Type == "vless" {
			defaultSeedAssertRealityAssociation(t, serverTLS, clientTLS, outboundTLS)
			continue
		}
		defaultSeedAssertBootstrapSelfSignedAssociation(t, inbound.Type, expectedTLSName, clientTLS, outboundTLS)
	}

	if len(seenTypes) != len(expectedTypes) {
		t.Fatalf("default inbound protocol collection = %#v, want %#v", seenTypes, expectedTypes)
	}
}

func defaultSeedJSONMap(t *testing.T, raw json.RawMessage, description string) map[string]interface{} {
	t.Helper()
	if len(raw) == 0 {
		t.Fatalf("%s is empty", description)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("parse %s JSON: %v", description, err)
	}
	if decoded == nil {
		t.Fatalf("%s decodes to null", description)
	}
	return decoded
}

func defaultSeedAssertRealityAssociation(t *testing.T, serverTLS, clientTLS, outboundTLS map[string]interface{}) {
	t.Helper()
	serverReality, ok := serverTLS["reality"].(map[string]interface{})
	if !ok || serverReality["enabled"] != true {
		t.Fatalf("Reality TLS server config is missing enabled reality settings: %#v", serverTLS["reality"])
	}
	clientReality, ok := clientTLS["reality"].(map[string]interface{})
	if !ok || clientReality["enabled"] != true {
		t.Fatalf("Reality TLS client config is missing enabled reality settings: %#v", clientTLS["reality"])
	}
	outboundReality, ok := outboundTLS["reality"].(map[string]interface{})
	if !ok || outboundReality["enabled"] != true {
		t.Fatalf("VLESS OutJson is missing enabled Reality settings: %#v", outboundTLS["reality"])
	}

	serverShortIDs, ok := serverReality["short_id"].([]interface{})
	if !ok || len(serverShortIDs) == 0 {
		t.Fatalf("Reality TLS server short_id = %#v, want non-empty array", serverReality["short_id"])
	}
	clientShortID, ok := clientReality["short_id"].(string)
	if !ok || clientShortID == "" {
		t.Fatalf("Reality TLS client short_id = %#v, want non-empty string", clientReality["short_id"])
	}
	if serverShortIDs[0] != clientShortID || outboundReality["short_id"] != clientShortID {
		t.Fatalf("Reality short ID association mismatch: server=%#v client=%#v outbound=%#v", serverShortIDs, clientShortID, outboundReality["short_id"])
	}
	if publicKey, ok := clientReality["public_key"].(string); !ok || publicKey == "" {
		t.Fatalf("Reality TLS client public_key = %#v, want non-empty string", clientReality["public_key"])
	}
	if _, insecure := clientTLS["insecure"]; insecure {
		t.Fatalf("Reality TLS client unexpectedly uses bootstrap insecure mode: %#v", clientTLS["insecure"])
	}
}

func defaultSeedAssertBootstrapSelfSignedAssociation(t *testing.T, inboundType, tlsName string, clientTLS, outboundTLS map[string]interface{}) {
	t.Helper()
	if !strings.HasSuffix(tlsName, selfSignedTLSBootstrapOnlySuffix) {
		t.Fatalf("default %s TLS template %q is not marked bootstrap-only", inboundType, tlsName)
	}
	if insecure, ok := clientTLS["insecure"].(bool); !ok || !insecure {
		t.Fatalf("default %s bootstrap TLS client insecure = %#v, want true", inboundType, clientTLS["insecure"])
	}
	if insecure, ok := outboundTLS["insecure"].(bool); !ok || !insecure {
		t.Fatalf("default %s OutJson TLS insecure = %#v, want true", inboundType, outboundTLS["insecure"])
	}
}
