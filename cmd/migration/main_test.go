package migration

import (
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"gorm.io/gorm"
)

func TestMigrateDbPreservesLegacy14ReleaseAndRecordsSchema(t *testing.T) {
	path := createMigrationTestDatabase(t,
		[]model.Setting{
			{Key: "version", Value: "1.4.9"},
			{Key: "config", Value: legacyDNSConfig},
		},
		[]model.Client{legacyMigrationClient()},
	)

	if err := MigrateDb(); err != nil {
		t.Fatalf("migrate 1.4.x database: %v", err)
	}

	assertMigrationSetting(t, path, "version", "1.4.9")
	assertMigrationSetting(t, path, schemaVersionSettingKey, config.GetDatabaseSchemaVersion())
	assertMigrationSettingCount(t, path, schemaVersionSettingKey, 1)

	// A 1.4.x release is already past the final explicit 1.3 migration. The
	// old migration must not run again merely because the display version is 0.1.
	assertClientHasAnyTLS(t, path, false)
	assertDNSAddressStillPresent(t, path, true)

	if err := MigrateDb(); err != nil {
		t.Fatalf("repeat migrate 1.4.x database: %v", err)
	}
	assertMigrationSetting(t, path, "version", "1.4.9")
	assertMigrationSettingCount(t, path, schemaVersionSettingKey, 1)
}

func TestMigrateDbMigratesLegacy12OnceAndUsesSchemaMarker(t *testing.T) {
	path := createMigrationTestDatabase(t,
		[]model.Setting{
			{Key: "version", Value: "1.2.8"},
			{Key: "config", Value: legacyDNSConfig},
		},
		[]model.Client{legacyMigrationClient()},
	)

	if err := MigrateDb(); err != nil {
		t.Fatalf("migrate 1.2 database: %v", err)
	}

	assertMigrationSetting(t, path, "version", "1.2.8")
	assertMigrationSetting(t, path, schemaVersionSettingKey, config.GetDatabaseSchemaVersion())
	assertMigrationSettingCount(t, path, schemaVersionSettingKey, 1)
	assertClientHasAnyTLS(t, path, true)
	assertDNSAddressStillPresent(t, path, false)

	configAfterFirstMigration := readMigrationSetting(t, path, "config")
	clientAfterFirstMigration := readMigrationClientConfig(t, path)
	if err := MigrateDb(); err != nil {
		t.Fatalf("repeat migrate 1.2 database: %v", err)
	}

	assertMigrationSetting(t, path, "version", "1.2.8")
	assertMigrationSetting(t, path, schemaVersionSettingKey, config.GetDatabaseSchemaVersion())
	assertMigrationSettingCount(t, path, schemaVersionSettingKey, 1)
	if got := readMigrationSetting(t, path, "config"); got != configAfterFirstMigration {
		t.Fatalf("second migration rewrote config: got %q, want %q", got, configAfterFirstMigration)
	}
	if got := readMigrationClientConfig(t, path); got != clientAfterFirstMigration {
		t.Fatalf("second migration rewrote client config: got %q, want %q", got, clientAfterFirstMigration)
	}
}

func TestMigrateDbDoesNotDowngradeNewerSchemaMarker(t *testing.T) {
	path := createMigrationTestDatabase(t,
		[]model.Setting{
			{Key: "version", Value: "1.4.9"},
			{Key: schemaVersionSettingKey, Value: "1.4"},
			{Key: "config", Value: legacyDNSConfig},
		},
		[]model.Client{legacyMigrationClient()},
	)

	if err := MigrateDb(); err != nil {
		t.Fatalf("migrate newer-schema database: %v", err)
	}

	assertMigrationSetting(t, path, "version", "1.4.9")
	assertMigrationSetting(t, path, schemaVersionSettingKey, "1.4")
	assertMigrationSettingCount(t, path, schemaVersionSettingKey, 1)
	assertClientHasAnyTLS(t, path, false)
	assertDNSAddressStillPresent(t, path, true)
}

const legacyDNSConfig = `{
  "dns": {
    "servers": [
      {
        "address": "tcp://1.1.1.1:53",
        "address_resolver": "bootstrap",
        "strategy": "prefer_ipv4"
      }
    ]
  }
}`

func legacyMigrationClient() model.Client {
	return model.Client{
		Enable:   true,
		Name:     "legacy-client",
		Config:   json.RawMessage(`{"trojan":{"password":"legacy-password"}}`),
		Inbounds: json.RawMessage(`[]`),
		Links:    json.RawMessage(`[]`),
	}
}

func createMigrationTestDatabase(t *testing.T, settings []model.Setting, clients []model.Client) string {
	t.Helper()
	dbFolder := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbFolder)
	path := config.GetDBPath()
	db := openMigrationTestDatabase(t, path)
	if err := db.AutoMigrate(&model.Setting{}, &model.Client{}, &model.Outbound{}); err != nil {
		closeMigrationTestDatabase(t, db)
		t.Fatalf("create test schema: %v", err)
	}
	if len(settings) > 0 {
		if err := db.Create(&settings).Error; err != nil {
			closeMigrationTestDatabase(t, db)
			t.Fatalf("seed settings: %v", err)
		}
	}
	if len(clients) > 0 {
		if err := db.Create(&clients).Error; err != nil {
			closeMigrationTestDatabase(t, db)
			t.Fatalf("seed clients: %v", err)
		}
	}
	closeMigrationTestDatabase(t, db)
	return path
}

func openMigrationTestDatabase(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	return db
}

func closeMigrationTestDatabase(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("access test SQL database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close test SQL database: %v", err)
	}
}

func readMigrationSetting(t *testing.T, path, key string) string {
	t.Helper()
	db := openMigrationTestDatabase(t, path)
	defer closeMigrationTestDatabase(t, db)
	var setting model.Setting
	if err := db.Where("key = ?", key).First(&setting).Error; err != nil {
		t.Fatalf("read setting %q: %v", key, err)
	}
	return setting.Value
}

func assertMigrationSetting(t *testing.T, path, key, want string) {
	t.Helper()
	if got := readMigrationSetting(t, path, key); got != want {
		t.Fatalf("setting %q = %q, want %q", key, got, want)
	}
}

func assertMigrationSettingCount(t *testing.T, path, key string, want int64) {
	t.Helper()
	db := openMigrationTestDatabase(t, path)
	defer closeMigrationTestDatabase(t, db)
	var count int64
	if err := db.Model(&model.Setting{}).Where("key = ?", key).Count(&count).Error; err != nil {
		t.Fatalf("count setting %q: %v", key, err)
	}
	if count != want {
		t.Fatalf("setting %q count = %d, want %d", key, count, want)
	}
}

func readMigrationClientConfig(t *testing.T, path string) string {
	t.Helper()
	db := openMigrationTestDatabase(t, path)
	defer closeMigrationTestDatabase(t, db)
	var client model.Client
	if err := db.Where("name = ?", "legacy-client").First(&client).Error; err != nil {
		t.Fatalf("read legacy client: %v", err)
	}
	return string(client.Config)
}

func assertClientHasAnyTLS(t *testing.T, path string, want bool) {
	t.Helper()
	var clientConfig map[string]json.RawMessage
	if err := json.Unmarshal([]byte(readMigrationClientConfig(t, path)), &clientConfig); err != nil {
		t.Fatalf("parse legacy client config: %v", err)
	}
	_, got := clientConfig["anytls"]
	if got != want {
		t.Fatalf("client anytls present = %t, want %t; config=%s", got, want, clientConfig)
	}
}

func assertDNSAddressStillPresent(t *testing.T, path string, want bool) {
	t.Helper()
	var coreConfig struct {
		DNS struct {
			Servers []map[string]interface{} `json:"servers"`
		} `json:"dns"`
	}
	if err := json.Unmarshal([]byte(readMigrationSetting(t, path, "config")), &coreConfig); err != nil {
		t.Fatalf("parse legacy DNS config: %v", err)
	}
	if len(coreConfig.DNS.Servers) != 1 {
		t.Fatalf("DNS server count = %d, want 1", len(coreConfig.DNS.Servers))
	}
	_, got := coreConfig.DNS.Servers[0]["address"]
	if got != want {
		t.Fatalf("DNS address present = %t, want %t; server=%#v", got, want, coreConfig.DNS.Servers[0])
	}
	if !want && coreConfig.DNS.Servers[0]["type"] != "tcp" {
		t.Fatalf("migrated DNS type = %#v, want tcp", coreConfig.DNS.Servers[0]["type"])
	}
}
