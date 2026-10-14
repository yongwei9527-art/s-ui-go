package database

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/yongwei9527-art/s-ui-go/database/model"
)

func migrationFixture(t *testing.T) (model.Tls, model.Inbound, *defaultProductionTLS) {
	t.Helper()
	clearProductionEnv(t)
	database := productionDB(t)
	template, err := buildBootstrapOnlySelfSignedTLS("tls-trojan", defaultBootstrapTLSName, []string{"http/1.1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	server := decodeProductionJSON(t, template.Server)
	clientTLS := decodeProductionJSON(t, template.Client)
	clientTLS["certificate"] = server["certificate"]
	clientTLS["custom_tls_option"] = "keep"
	outbound, err := json.Marshal(map[string]interface{}{
		"type": "trojan", "tag": "custom-tag", "server": "192.0.2.20", "server_port": 24443,
		"transport": map[string]interface{}{"type": "ws", "path": "/custom"},
		"tls":       clientTLS, "dialer": "custom-dialer",
	})
	if err != nil {
		t.Fatal(err)
	}
	inbound := model.Inbound{Type: "trojan", Tag: "custom-tag", TlsId: template.Id,
		Options: json.RawMessage(`{"listen_port":8443,"transport":{}}`), OutJson: outbound,
		Addrs: json.RawMessage(`[{"server":"192.0.2.20","server_port":24443,"insecure":true,"server_name":"bootstrap.s-ui.invalid","tls":{"server_name":"bootstrap.s-ui.invalid","insecure":true,"custom":"keep"}},{"server":"custom.test","server_name":"custom.test","insecure":true,"tls":{"server_name":"custom.test","insecure":true,"alpn":["custom"]}}]`)}
	if err := database.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	cert, key := productionCertificate(t, "vpn.example.test", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	return template, inbound, &defaultProductionTLS{serverName: "vpn.example.test", certificate: cert, key: key}
}

func TestProductionTLSMigrationPreservesCustomOutboundAndAddressOverrides(t *testing.T) {
	_, before, config := migrationFixture(t)
	if err := migrateDefaultProductionTLS(db, config); err != nil {
		t.Fatal(err)
	}
	var after model.Inbound
	if err := db.First(&after, before.Id).Error; err != nil {
		t.Fatal(err)
	}
	oldOut := decodeProductionJSON(t, before.OutJson)
	newOut := decodeProductionJSON(t, after.OutJson)
	newTLS := newOut["tls"].(map[string]interface{})
	if newTLS["server_name"] != config.serverName || newTLS["insecure"] != false || newTLS["custom_tls_option"] != "keep" {
		t.Fatalf("incorrect targeted TLS migration: %v", newTLS)
	}
	if _, found := newTLS["certificate"]; found {
		t.Fatal("bootstrap certificate still pins the old self-signed identity")
	}
	delete(oldOut, "tls")
	delete(newOut, "tls")
	if !reflect.DeepEqual(oldOut, newOut) || !bytes.Equal(before.Options, after.Options) {
		t.Fatal("migration overwrote custom outbound/listener configuration")
	}
	var oldAddrs, newAddrs []map[string]interface{}
	if err := json.Unmarshal(before.Addrs, &oldAddrs); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after.Addrs, &newAddrs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(oldAddrs[1], newAddrs[1]) {
		t.Fatal("independent address TLS override changed")
	}
	if newAddrs[0]["insecure"] != false || newAddrs[0]["server_name"] != config.serverName || newAddrs[0]["server_port"] != float64(24443) {
		t.Fatalf("bad inherited address migration: %v", newAddrs[0])
	}
	nested := newAddrs[0]["tls"].(map[string]interface{})
	if nested["insecure"] != false || nested["server_name"] != config.serverName || nested["custom"] != "keep" {
		t.Fatalf("bad nested TLS migration: %v", nested)
	}
}

func TestProductionTLSMigrationPreservesIndependentOutboundTLS(t *testing.T) {
	_, before, config := migrationFixture(t)
	out := decodeProductionJSON(t, before.OutJson)
	tls := out["tls"].(map[string]interface{})
	tls["server_name"] = "custom.test"
	before.OutJson, _ = json.Marshal(out)
	if err := db.Model(&before).Update("out_json", before.OutJson).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateDefaultProductionTLS(db, config); err != nil {
		t.Fatal(err)
	}
	var after model.Inbound
	if err := db.First(&after, before.Id).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, decodeProductionJSON(t, after.OutJson)) {
		t.Fatal("independent outbound TLS configuration changed")
	}
}

func TestProductionTLSMigrationPreservesCompositeAddressOverrides(t *testing.T) {
	_, before, config := migrationFixture(t)
	// In both cases the effective SNI is independent, although only one layer
	// carries it. Migrating the other layer would still alter its TLS policy.
	before.Addrs = json.RawMessage(`[
		{"server":"custom.test","server_name":"custom.test","insecure":true,"tls":{"insecure":true,"alpn":["custom"]}},
		{"server":"custom.test","server_name":"bootstrap.s-ui.invalid","insecure":true,"tls":{"server_name":"custom.test","insecure":true}}
	]`)
	if err := db.Model(&before).Update("addrs", before.Addrs).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateDefaultProductionTLS(db, config); err != nil {
		t.Fatal(err)
	}
	var after model.Inbound
	if err := db.First(&after, before.Id).Error; err != nil {
		t.Fatal(err)
	}
	var oldAddrs, newAddrs []map[string]interface{}
	if err := json.Unmarshal(before.Addrs, &oldAddrs); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(after.Addrs, &newAddrs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(oldAddrs, newAddrs) {
		t.Fatalf("composite independent TLS overrides changed: %s", after.Addrs)
	}
}

func TestProductionTLSMigrationPreservesExactCustomJSONNumbers(t *testing.T) {
	_, before, config := migrationFixture(t)
	out := decodeProductionJSON(t, before.OutJson)
	out["custom_counter"] = json.Number("9007199254740993")
	before.OutJson, _ = json.Marshal(out)
	before.Addrs = json.RawMessage(`[{"server":"custom.test","server_name":"custom.test","custom_counter":9007199254740993,"tls":{"insecure":true}}]`)
	if err := db.Model(&before).Updates(map[string]interface{}{"out_json": before.OutJson, "addrs": before.Addrs}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateDefaultProductionTLS(db, config); err != nil {
		t.Fatal(err)
	}
	var after model.Inbound
	if err := db.First(&after, before.Id).Error; err != nil {
		t.Fatal(err)
	}
	for _, raw := range []json.RawMessage{after.OutJson, after.Addrs} {
		if !bytes.Contains(raw, []byte(`"custom_counter":9007199254740993`)) {
			t.Fatalf("custom JSON integer rounded during migration: %s", raw)
		}
	}
}

func TestProductionTLSMigrationRejectsNullJSONAndRollsBack(t *testing.T) {
	for _, field := range []string{"out_json", "addrs"} {
		t.Run(field, func(t *testing.T) {
			template, inbound, config := migrationFixture(t)
			bad := json.RawMessage(`null`)
			if field == "addrs" {
				bad = json.RawMessage(`[null]`)
			}
			if err := db.Model(&inbound).Update(field, bad).Error; err != nil {
				t.Fatal(err)
			}
			if err := migrateDefaultProductionTLS(db, config); err == nil {
				t.Fatal("invalid null JSON accepted")
			}
			var after model.Tls
			if err := db.First(&after, template.Id).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(template, after) {
				t.Fatal("failed migration did not roll back TLS")
			}
		})
	}
}
