package migration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/sagernet/sing-box/option"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"gorm.io/gorm"
)

// Regression tests use isolated databases; no installed database is touched.
func auditDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "witness.db")))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err = db.AutoMigrate(&model.Setting{}); err != nil {
		t.Fatal(err)
	}
	return db
}
func auditConfigFile(t *testing.T, text string) {
	t.Helper()
	dir := t.TempDir()
	savedArgs := os.Args
	os.Args = append([]string{filepath.Join(dir, "sui.exe")}, savedArgs[1:]...)
	t.Cleanup(func() { os.Args = savedArgs })
	t.Setenv("SUI_BIN_FOLDER", "bin")
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bin", "config.json"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestDNSMigrationPreservesHostPortAndPath(t *testing.T) {
	db := auditDB(t)
	text := `{"dns":{"servers":[{"address":"tcp://1.1.1.1:53"},{"address":"https://dns.example/custom-query"}]}}`
	if err := db.Create(&model.Setting{Key: "config", Value: text}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrate_dns(db); err != nil {
		t.Fatal(err)
	}
	var s model.Setting
	if err := db.Where("key=?", "config").First(&s).Error; err != nil {
		t.Fatal(err)
	}
	t.Logf("migrated config=%s", s.Value)
	var v struct {
		DNS struct {
			Servers []json.RawMessage `json:"servers"`
		} `json:"dns"`
	}
	if err := json.Unmarshal([]byte(s.Value), &v); err != nil {
		t.Fatal(err)
	}
	var first option.RemoteDNSServerOptions
	if err := json.Unmarshal(v.DNS.Servers[0], &first); err != nil {
		t.Fatal(err)
	}
	dest := first.DNSServerAddressOptions.Build()
	t.Logf("sing-box destination=%s; IsIP=%t; IsDomain=%t", dest.String(), dest.IsIP(), dest.IsDomain())
	if first.Server != "1.1.1.1" || first.ServerPort != 53 || !dest.IsIP() {
		t.Fatal("host or port was corrupted")
	}
	var second option.RemoteHTTPSDNSServerOptions
	if err := json.Unmarshal(v.DNS.Servers[1], &second); err != nil {
		t.Fatal(err)
	}
	if second.Path != "/custom-query" {
		t.Fatalf("path was corrupted: %q", second.Path)
	}
	t.Log("custom-query path preserved")
}
func TestMoveJSONUpdatesExistingConfig(t *testing.T) {
	db := auditDB(t)
	if err := db.Create(&model.Setting{Key: "config", Value: `{"old":true}`}).Error; err != nil {
		t.Fatal(err)
	}
	auditConfigFile(t, `{"inbounds":[],"outbounds":[],"experimental":{}}`)
	if err := moveJsonToDb(db); err != nil {
		t.Fatal(err)
	}
	var settings []model.Setting
	if err := db.Where("key=?", "config").Order("id").Find(&settings).Error; err != nil {
		t.Fatal(err)
	}
	t.Logf("config rows=%d values=%+v", len(settings), settings)
	if len(settings) != 1 || settings[0].Value == `{"old":true}` {
		t.Fatal("config was not updated in place")
	}
}
func TestMoveJSONAcceptsMissingExperimental(t *testing.T) {
	db := auditDB(t)
	auditConfigFile(t, `{"inbounds":[],"outbounds":[]}`)

	if err := moveJsonToDb(db); err != nil {
		t.Fatal(err)
	}
}
func TestFailedMigrationDoesNotMarkSchemaComplete(t *testing.T) {
	for _, table := range []string{"clients", "outbounds"} {
		t.Run(table, func(t *testing.T) {
			path := createMigrationTestDatabase(t, []model.Setting{{Key: "version", Value: "1.2.8"}, {Key: "config", Value: `{}`}}, []model.Client{legacyMigrationClient()})
			db := openMigrationTestDatabase(t, path)
			out := model.Outbound{Type: "direct", Tag: "test", Options: json.RawMessage(`{"domain_strategy":"prefer_ipv4"}`)}
			if err := db.Create(&out).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec("CREATE TRIGGER reject_" + table + " BEFORE UPDATE ON " + table + " BEGIN SELECT RAISE(ABORT, 'rejected update'); END").Error; err != nil {
				t.Fatal(err)
			}
			closeMigrationTestDatabase(t, db)
			if err := MigrateDb(); err == nil {
				t.Fatal("migration silently accepted a failed write")
			}
			assertMigrationSettingCount(t, path, schemaVersionSettingKey, 0)
			assertClientHasAnyTLS(t, path, false)
			db = openMigrationTestDatabase(t, path)
			defer closeMigrationTestDatabase(t, db)
			var after model.Outbound
			if err := db.First(&after, out.Id).Error; err != nil {
				t.Fatal(err)
			}
			if string(after.Options) != `{"domain_strategy":"prefer_ipv4"}` {
				t.Fatalf("failed migration changed outbound: %s", after.Options)
			}
		})
	}
}

func TestMoveJSONRejectsMalformedCollectionsBeforeDroppingTables(t *testing.T) {
	db := auditDB(t)
	if err := db.AutoMigrate(&model.Inbound{}); err != nil {
		t.Fatal(err)
	}
	existing := model.Inbound{Type: "vless", Tag: "keep", Options: json.RawMessage(`{}`)}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	auditConfigFile(t, `{"inbounds":[],"outbounds":[null]}`)
	if err := moveJsonToDb(db); err == nil {
		t.Fatal("accepted invalid legacy outbound")
	}
	var count int64
	if err := db.Model(&model.Inbound{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("validation destroyed existing inbounds: %d", count)
	}
}
