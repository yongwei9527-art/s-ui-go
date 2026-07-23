package database

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"gorm.io/gorm"
)

func TestSeedDefaultXrayRealityTemplatesCreatesDisabledDrafts(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := testDB.AutoMigrate(&model.Tls{}, &model.Inbound{}, &model.Client{}, &model.XrayXHTTPInbound{}, &model.XrayXHTTPClient{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	tlsTemplates, err := seedDefaultTlsTemplates(testDB)
	if err != nil {
		t.Fatalf("seed TLS templates: %v", err)
	}
	singBoxInbounds, err := seedDefaultInbounds(testDB, tlsTemplates, "127.0.0.1")
	if err != nil {
		t.Fatalf("seed sing-box inbounds: %v", err)
	}
	if err := seedDefaultClient(testDB, singBoxInbounds, "127.0.0.1"); err != nil {
		t.Fatalf("seed default client: %v", err)
	}
	if err := seedDefaultXrayRealityTemplates(testDB, singBoxInbounds); err != nil {
		t.Fatalf("seed Xray templates: %v", err)
	}

	var inbounds []model.XrayXHTTPInbound
	if err := testDB.Order("id").Find(&inbounds).Error; err != nil {
		t.Fatalf("load Xray templates: %v", err)
	}
	if len(inbounds) != 2 {
		t.Fatalf("Xray template count = %d, want 2", len(inbounds))
	}
	seen := map[string]model.XrayXHTTPInbound{}
	for _, inbound := range inbounds {
		if inbound.Enable {
			t.Fatalf("first-boot Xray template %q must remain disabled", inbound.Tag)
		}
		if inbound.Transport != "tcp" && inbound.Transport != "xhttp" {
			t.Fatalf("template %q transport = %q", inbound.Tag, inbound.Transport)
		}
		if !strings.HasPrefix(inbound.Tag, "xray-vless-reality-"+inbound.Transport+"-") {
			t.Fatalf("template tag %q does not match transport %q", inbound.Tag, inbound.Transport)
		}
		if inbound.Listen != "0.0.0.0" || inbound.Port == 0 || inbound.ServerName != "go.microsoft.com" || inbound.Dest != "go.microsoft.com:443" {
			t.Fatalf("template %q has unexpected endpoint settings: %#v", inbound.Tag, inbound)
		}
		if inbound.PrivateKey == "" || inbound.PublicKey == "" || len(inbound.ShortID) != 16 {
			t.Fatalf("template %q is missing generated Reality credentials", inbound.Tag)
		}
		if inbound.Transport == "xhttp" && (inbound.Path != "/xhttp" || inbound.Mode != "auto") {
			t.Fatalf("XHTTP template fields = path %q, mode %q", inbound.Path, inbound.Mode)
		}
		if inbound.Transport == "tcp" && (inbound.Path != "" || inbound.Mode != "") {
			t.Fatalf("TCP template unexpectedly has XHTTP fields: %#v", inbound)
		}
		seen[inbound.Transport] = inbound
	}
	if len(seen) != 2 {
		t.Fatalf("seeded transports = %#v, want tcp and xhttp", seen)
	}

	var client model.Client
	if err := testDB.Where("name = ?", defaultTemplateClientName).First(&client).Error; err != nil {
		t.Fatalf("load default client: %v", err)
	}
	for transport, inbound := range seen {
		var assignment model.XrayXHTTPClient
		if err := testDB.Where("inbound_id = ? AND client_id = ?", inbound.Id, client.Id).First(&assignment).Error; err != nil {
			t.Fatalf("load %s template client assignment: %v", transport, err)
		}
		if !assignment.Enable || assignment.UUID == "" {
			t.Fatalf("invalid %s client assignment: %#v", transport, assignment)
		}
	}
}
