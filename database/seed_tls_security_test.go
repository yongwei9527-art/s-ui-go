package database

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"gorm.io/gorm"
)

func TestBootstrapOnlySelfSignedTLSIsExplicitlyMarked(t *testing.T) {
	tlsTemplate, err := buildBootstrapOnlySelfSignedTLS("tls-test", "example.test", []string{"h3"})
	if err != nil {
		t.Fatalf("buildBootstrapOnlySelfSignedTLS() error = %v", err)
	}

	if !strings.HasSuffix(tlsTemplate.Name, selfSignedTLSBootstrapOnlySuffix) {
		t.Fatalf("self-signed TLS template name %q is missing bootstrap-only marker %q", tlsTemplate.Name, selfSignedTLSBootstrapOnlySuffix)
	}

	assertBootstrapOnlySelfSignedTLS(t, tlsTemplate)
}

func TestSeedDefaultTlsTemplatesMarksEverySelfSignedDefaultAsBootstrapOnly(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := testDB.AutoMigrate(&model.Tls{}, &model.Inbound{}); err != nil {
		t.Fatalf("migrate TLS and inbound models: %v", err)
	}

	seeded, err := seedDefaultTlsTemplates(testDB)
	if err != nil {
		t.Fatalf("seedDefaultTlsTemplates() error = %v", err)
	}

	selfSignedBaseNames := []string{
		"tls-hy2",
		"tls-tuic",
		"tls-trojan",
		"tls-trojan-ws",
		"tls-vmess-ws",
		"tls-naive",
		"tls-anytls",
		"tls-hysteria",
	}
	if len(seeded) != len(selfSignedBaseNames)+1 {
		t.Fatalf("seeded TLS count = %d, want %d", len(seeded), len(selfSignedBaseNames)+1)
	}

	for _, baseName := range selfSignedBaseNames {
		name := bootstrapOnlySelfSignedTLSName(baseName)
		seededTemplate, ok := seeded[name]
		if !ok {
			t.Fatalf("default self-signed TLS template %q was not seeded", name)
		}

		var tlsTemplate model.Tls
		if err := testDB.First(&tlsTemplate, seededTemplate.id).Error; err != nil {
			t.Fatalf("load seeded TLS template %q: %v", name, err)
		}
		assertBootstrapOnlySelfSignedTLS(t, tlsTemplate)
	}

	if _, ok := seeded["reality-vless"]; !ok {
		t.Fatal("Reality template was not seeded")
	}

	// Verify that renaming the visible self-signed templates does not break the
	// default inbound-to-TLS associations used by generated client links.
	inbounds, err := seedDefaultInbounds(testDB, seeded, "127.0.0.1")
	if err != nil {
		t.Fatalf("seedDefaultInbounds() error = %v", err)
	}
	expectedTLSByInboundType := map[string]string{
		"vless":     "reality-vless",
		"hysteria2": bootstrapOnlySelfSignedTLSName("tls-hy2"),
		"tuic":      bootstrapOnlySelfSignedTLSName("tls-tuic"),
		"trojan":    bootstrapOnlySelfSignedTLSName("tls-trojan"),
		"vmess":     bootstrapOnlySelfSignedTLSName("tls-vmess-ws"),
		"naive":     bootstrapOnlySelfSignedTLSName("tls-naive"),
		"anytls":    bootstrapOnlySelfSignedTLSName("tls-anytls"),
		"hysteria":  bootstrapOnlySelfSignedTLSName("tls-hysteria"),
	}
	if len(inbounds) != len(expectedTLSByInboundType)+1 { // + Shadowsocks, which has no TLS.
		t.Fatalf("seeded inbound count = %d, want %d", len(inbounds), len(expectedTLSByInboundType)+1)
	}
	for _, inbound := range inbounds {
		expectedTLSName, needsTLS := expectedTLSByInboundType[inbound.Type]
		if !needsTLS {
			if inbound.Type != "shadowsocks" || inbound.TlsId != 0 {
				t.Fatalf("unexpected non-TLS seeded inbound %#v", inbound)
			}
			continue
		}
		if inbound.TlsId != seeded[expectedTLSName].id {
			t.Fatalf("seeded %s inbound TLS ID = %d, want %d (%s)", inbound.Type, inbound.TlsId, seeded[expectedTLSName].id, expectedTLSName)
		}
	}
}

func assertBootstrapOnlySelfSignedTLS(t *testing.T, tlsTemplate model.Tls) {
	t.Helper()

	if !strings.HasSuffix(tlsTemplate.Name, selfSignedTLSBootstrapOnlySuffix) {
		t.Fatalf("self-signed TLS template name %q is missing bootstrap-only marker %q", tlsTemplate.Name, selfSignedTLSBootstrapOnlySuffix)
	}

	var client map[string]interface{}
	if err := json.Unmarshal(tlsTemplate.Client, &client); err != nil {
		t.Fatalf("unmarshal client TLS config: %v", err)
	}
	if insecure, ok := client["insecure"].(bool); !ok || !insecure {
		t.Fatalf("self-signed bootstrap template must explicitly retain insecure=true, got %#v", client["insecure"])
	}

	var server map[string]interface{}
	if err := json.Unmarshal(tlsTemplate.Server, &server); err != nil {
		t.Fatalf("unmarshal server TLS config: %v", err)
	}
	for _, key := range []string{"certificate", "key"} {
		value, ok := server[key].([]interface{})
		if !ok || len(value) == 0 {
			t.Fatalf("self-signed bootstrap template server config is missing %s", key)
		}
	}
}

func TestMigrateLegacyBootstrapOnlyTLSTemplateNamesIsStrictAndPreservesAssociations(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := testDB.AutoMigrate(&model.Tls{}, &model.Inbound{}); err != nil {
		t.Fatalf("migrate TLS and inbound models: %v", err)
	}

	type savedTemplate struct {
		id     uint
		server json.RawMessage
		client json.RawMessage
	}
	legacyTemplates := make(map[string]savedTemplate, len(legacyBootstrapOnlyTLSSpecs))
	for _, spec := range legacyBootstrapOnlyTLSSpecs {
		template, err := buildBootstrapOnlySelfSignedTLS(spec.name, spec.serverName, spec.alpn)
		if err != nil {
			t.Fatalf("build legacy %s template: %v", spec.name, err)
		}
		template.Name = spec.name // Simulate the pre-marker database version.
		if err := testDB.Create(&template).Error; err != nil {
			t.Fatalf("create legacy %s template: %v", spec.name, err)
		}
		legacyTemplates[spec.name] = savedTemplate{id: template.Id, server: append(json.RawMessage(nil), template.Server...), client: append(json.RawMessage(nil), template.Client...)}
	}

	// This intentionally resembles the historical name but differs from the
	// generated template. It represents user-maintained data and must not move.
	custom, err := buildBootstrapOnlySelfSignedTLS("tls-hy2", "custom.example.test", []string{"h3"})
	if err != nil {
		t.Fatalf("build user-maintained TLS template: %v", err)
	}
	custom.Name = "tls-hy2" // Reuse a historical name without matching its generated structure.
	if err := testDB.Create(&custom).Error; err != nil {
		t.Fatalf("create user-maintained TLS template: %v", err)
	}

	linkedID := legacyTemplates["tls-hy2"].id
	linkedInbound := model.Inbound{Type: "hysteria2", Tag: "legacy-hy2", TlsId: linkedID}
	if err := testDB.Create(&linkedInbound).Error; err != nil {
		t.Fatalf("create linked inbound: %v", err)
	}

	if err := migrateLegacyBootstrapOnlyTLSTemplateNames(testDB); err != nil {
		t.Fatalf("migrate legacy templates: %v", err)
	}
	// A second run must make no additional data changes.
	if err := migrateLegacyBootstrapOnlyTLSTemplateNames(testDB); err != nil {
		t.Fatalf("repeat legacy template migration: %v", err)
	}

	for _, spec := range legacyBootstrapOnlyTLSSpecs {
		saved := legacyTemplates[spec.name]
		var migrated model.Tls
		if err := testDB.First(&migrated, saved.id).Error; err != nil {
			t.Fatalf("load migrated %s template: %v", spec.name, err)
		}
		if migrated.Name != bootstrapOnlySelfSignedTLSName(spec.name) {
			t.Fatalf("migrated %s name = %q, want %q", spec.name, migrated.Name, bootstrapOnlySelfSignedTLSName(spec.name))
		}
		if string(migrated.Server) != string(saved.server) || string(migrated.Client) != string(saved.client) {
			t.Fatalf("migrated %s changed TLS configuration bytes", spec.name)
		}
	}

	var unchangedCustom model.Tls
	if err := testDB.First(&unchangedCustom, custom.Id).Error; err != nil {
		t.Fatalf("load user-maintained template: %v", err)
	}
	if unchangedCustom.Name != "tls-hy2" {
		t.Fatalf("user-maintained template was renamed to %q", unchangedCustom.Name)
	}

	var reloadedInbound model.Inbound
	if err := testDB.Preload("Tls").First(&reloadedInbound, linkedInbound.Id).Error; err != nil {
		t.Fatalf("reload linked inbound: %v", err)
	}
	if reloadedInbound.TlsId != linkedID || reloadedInbound.Tls == nil || reloadedInbound.Tls.Id != linkedID || reloadedInbound.Tls.Name != bootstrapOnlySelfSignedTLSName("tls-hy2") {
		t.Fatalf("legacy inbound TLS association changed after rename: %#v", reloadedInbound)
	}
}
