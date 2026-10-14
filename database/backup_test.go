package database

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"gorm.io/gorm"
)

func backupTestDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open(path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(50)"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.AutoMigrate(&model.Setting{}, &model.Tls{}, &model.Inbound{}, &model.Outbound{},
		&model.Service{}, &model.Endpoint{}, &model.User{}, &model.Tokens{}, &model.Client{},
		&model.Stats{}, &model.Changes{}, &model.XrayXHTTPInbound{}, &model.XrayXHTTPClient{}); err != nil {
		t.Fatal(err)
	}
	return database
}

func useBackupTestDB(t *testing.T, database *gorm.DB) {
	t.Helper()
	old := db
	db = database
	t.Cleanup(func() {
		if db != nil {
			if sqlDB, err := db.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		db = old
	})
}

func readBackup(t *testing.T, data []byte) *gorm.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "export.db")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	database, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB()
	t.Cleanup(func() { _ = sqlDB.Close() })
	var integrity string
	if err := database.Raw("PRAGMA integrity_check").Row().Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("export integrity=%q error=%v", integrity, err)
	}
	return database
}

// Unlike exportDB, this preserves triggers and missing legacy tables.
func closedBackupBytes(t *testing.T, database *gorm.DB) []byte {
	t.Helper()
	var seq int
	var name, path string
	if err := database.Raw("PRAGMA database_list").Row().Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestGetDbReadsConsistentWALSnapshot(t *testing.T) {
	source := backupTestDB(t, filepath.Join(t.TempDir(), "source.db"))
	useBackupTestDB(t, source)
	if err := source.Create(&model.Setting{Key: "revision", Value: "old"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := source.Create(&model.Client{Name: "old"}).Error; err != nil {
		t.Fatal(err)
	}
	var changed atomic.Bool
	// Commit a related two-table change after the first export SELECT. Without
	// a source transaction the result contains old settings but new clients.
	if err := source.Callback().Query().After("gorm:query").Register("test:concurrent_write", func(query *gorm.DB) {
		if query.Statement.Table != "settings" || !changed.CompareAndSwap(false, true) {
			return
		}
		err := source.Transaction(func(writer *gorm.DB) error {
			if err := writer.Exec("UPDATE settings SET value = 'new' WHERE key = 'revision'").Error; err != nil {
				return err
			}
			return writer.Exec("UPDATE clients SET name = 'new'").Error
		})
		if err != nil {
			query.AddError(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	data, err := GetDb("")
	if err != nil {
		t.Fatal(err)
	}
	if !changed.Load() {
		t.Fatal("concurrent writer was not exercised")
	}
	backup := readBackup(t, data)
	var setting model.Setting
	var client model.Client
	if err := backup.First(&setting).Error; err != nil {
		t.Fatal(err)
	}
	if err := backup.First(&client).Error; err != nil {
		t.Fatal(err)
	}
	if setting.Value != "old" || client.Name != "old" {
		t.Fatalf("inconsistent snapshot: setting=%q client=%q", setting.Value, client.Name)
	}
	if err := source.First(&client).Error; err != nil || client.Name != "new" {
		t.Fatalf("concurrent WAL commit was not retained: client=%q err=%v", client.Name, err)
	}
}

func TestGetDbConcurrentExportsExclusionsAndDisabledAssignment(t *testing.T) {
	source := backupTestDB(t, filepath.Join(t.TempDir(), "source.db"))
	useBackupTestDB(t, source)
	for _, value := range []interface{}{
		&model.Stats{Traffic: 123}, &model.Changes{Key: "keep"},
		&model.XrayXHTTPInbound{Tag: "draft", PrivateKey: "private-key"},
		&model.XrayXHTTPClient{InboundID: 1, ClientID: 1, UUID: "independent-uuid"},
	} {
		if err := source.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := source.Model(&model.XrayXHTTPClient{}).Where("id = ?", 1).Update("enable", false).Error; err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan []byte, 6)
	errs := make(chan error, 6)
	for n := 0; n < 6; n++ {
		wg.Go(func() {
			data, err := GetDb(" changes , stats ")
			if err != nil {
				errs <- err
				return
			}
			results <- data
		})
	}
	wg.Wait()
	close(errs)
	close(results)
	for err := range errs {
		t.Error(err)
	}
	if len(results) != 6 {
		t.Fatalf("successful concurrent exports=%d want 6", len(results))
	}
	for data := range results {
		backup := readBackup(t, data)
		for _, table := range []interface{}{&model.Stats{}, &model.Changes{}} {
			var count int64
			if err := backup.Model(table).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("excluded table count=%d err=%v", count, err)
			}
		}
		var assignment model.XrayXHTTPClient
		if err := backup.First(&assignment).Error; err != nil || assignment.Enable || assignment.UUID != "independent-uuid" {
			t.Fatalf("assignment changed in export: %#v err=%v", assignment, err)
		}
		var inbound model.XrayXHTTPInbound
		if err := backup.First(&inbound).Error; err != nil || inbound.Enable || inbound.PrivateKey != "private-key" {
			t.Fatalf("inbound changed in export: %#v err=%v", inbound, err)
		}
	}
}

type backupUpload struct{ *bytes.Reader }

func (backupUpload) Close() error { return nil }

func importFixture(t *testing.T) (*gorm.DB, []byte) {
	t.Helper()
	clearProductionEnv(t)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	database := backupTestDB(t, config.GetDBPath())
	useBackupTestDB(t, database)
	for _, value := range []interface{}{
		&model.Setting{Key: "schemaVersion", Value: config.GetDatabaseSchemaVersion()},
		&model.Setting{Key: "identity", Value: "old"},
		&model.Tls{Name: "custom", Server: []byte(`{}`), Client: []byte(`{}`)},
		&model.User{Username: "existing-admin", Password: "keep-hash"},
	} {
		if err := database.Create(value).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := database.Model(&model.Setting{}).Where("key = ?", "identity").Update("value", "uploaded").Error; err != nil {
		t.Fatal(err)
	}
	data, err := GetDb("")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Model(&model.Setting{}).Where("key = ?", "identity").Update("value", "old").Error; err != nil {
		t.Fatal(err)
	}
	return database, data
}

func assertImportIdentity(t *testing.T, want string) {
	t.Helper()
	if db == nil {
		t.Fatal("active database connection missing")
	}
	var setting model.Setting
	if err := db.Where("key = ?", "identity").First(&setting).Error; err != nil || setting.Value != want {
		t.Fatalf("identity=%q want %q error=%v", setting.Value, want, err)
	}
}

func assertImportChangeCount(t *testing.T, database *gorm.DB, want int64) {
	t.Helper()
	var count int64
	if err := database.Model(&model.Changes{}).Where("key = ? AND action = ?", "database", "import").Count(&count).Error; err != nil || count != want {
		t.Fatalf("import marker count=%d want %d err=%v", count, want, err)
	}
}

func assertImportMarker(t *testing.T, database *gorm.DB, want uint64) model.Changes {
	t.Helper()
	assertImportChangeCount(t, database, 1)
	var marker model.Changes
	if err := database.Where("key = ? AND action = ?", "database", "import").First(&marker).Error; err != nil {
		t.Fatal(err)
	}
	if marker.Id != want || marker.Actor != "system" || marker.DateTime <= 0 {
		t.Fatalf("import revision marker=%#v want ID=%d", marker, want)
	}
	if !json.Valid(marker.Obj) || !bytes.Equal(marker.Obj, []byte(`{"operation":"database-import"}`)) {
		t.Fatalf("invalid import marker JSON: %q", marker.Obj)
	}
	var obj struct{ Operation string }
	if err := json.Unmarshal(marker.Obj, &obj); err != nil || obj.Operation != "database-import" {
		t.Fatalf("import marker operation=%q err=%v", obj.Operation, err)
	}
	// Obj is json.RawMessage: invalid escaped JSON also breaks the API response.
	if _, err := json.Marshal(marker); err != nil {
		t.Fatalf("import marker cannot be marshaled as an API change: %v", err)
	}
	return marker
}

func retainedImportFallback(t *testing.T) *gorm.DB {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(config.GetDBFolderPath(), ".s-ui-fallback-*.db"))
	if err != nil || len(files) != 1 {
		t.Fatalf("expected one retained fallback: %v err=%v", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	fallback := readBackup(t, data)
	var identity model.Setting
	if err := fallback.Where("key = ?", "identity").First(&identity).Error; err != nil || identity.Value != "old" {
		t.Fatalf("wrong fallback identity: %q err=%v", identity.Value, err)
	}
	assertImportChangeCount(t, fallback, 0)
	return fallback
}

func TestImportDBAdvancesRevisionAcrossHistoryReplacement(t *testing.T) {
	for _, tc := range []struct {
		name                                           string
		original, uploaded, duringRestart              uint64
		missingUploadedHistory, missingOriginalHistory bool
	}{
		{"older", 1000, 20, 0, false, false},
		{"equal", 1000, 1000, 0, false, false},
		{"newer", 20, 1000, 0, false, false},
		{"no-history", 1000, 0, 0, false, false},
		{"both-empty-history", 0, 0, 0, false, false},
		{"missing-uploaded-history-table", 1000, 0, 0, true, false},
		{"missing-original-history-table", 0, 20, 0, false, true},
		{"both-missing-history-tables", 0, 0, 0, true, true},
		{"large-exact-original-id", 9007199254740993, 20, 0, false, false},
		{"large-exact-uploaded-id", 20, 9007199254740993, 0, false, false},
		{"restart-commits-change", 1000, 20, 2000, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, data := importFixture(t)
			if tc.original > 0 {
				if err := source.Create(&model.Changes{Id: tc.original, Key: "original-history", Obj: []byte(`{}`)}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if tc.missingOriginalHistory {
				if err := source.Migrator().DropTable(&model.Changes{}); err != nil {
					t.Fatal(err)
				}
			}
			uploadDB := readBackup(t, data)
			// A legitimate legacy release marker, with no current schema marker.
			if err := uploadDB.Exec("DELETE FROM settings WHERE key = 'schemaVersion'").Error; err != nil {
				t.Fatal(err)
			}
			if err := uploadDB.Create(&model.Setting{Key: "version", Value: "1.4.9"}).Error; err != nil {
				t.Fatal(err)
			}
			if tc.uploaded > 0 {
				if err := uploadDB.Create(&model.Changes{Id: tc.uploaded, Key: "uploaded-history", Obj: []byte(`{}`)}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if tc.missingUploadedHistory {
				if err := uploadDB.Migrator().DropTable(&model.Changes{}); err != nil {
					t.Fatal(err)
				}
			}
			data = closedBackupBytes(t, uploadDB)
			err := importDB(backupUpload{bytes.NewReader(data)}, func() error {
				assertImportChangeCount(t, db, 0)
				if tc.duringRestart > 0 {
					return db.Create(&model.Changes{Id: tc.duringRestart, Key: "restart-history", Obj: []byte(`{}`)}).Error
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			assertImportIdentity(t, "uploaded")
			want := max(tc.original, tc.uploaded, tc.duringRestart) + 1
			marker := assertImportMarker(t, db, want)
			if err := closeCurrentDB(); err != nil {
				t.Fatal(err)
			}
			if err := InitDB(config.GetDBPath()); err != nil {
				t.Fatal(err)
			}
			persisted := assertImportMarker(t, db, want)
			if persisted.DateTime != marker.DateTime {
				t.Fatal("reopen regenerated the import marker")
			}
			exported, err := GetDb("")
			if err != nil {
				t.Fatal(err)
			}
			assertImportMarker(t, readBackup(t, exported), want)
			var count int64
			if err := db.Model(&model.Changes{}).Where("key = ?", "uploaded-history").Count(&count).Error; err != nil || (tc.uploaded > 0 && count != 1) || (tc.uploaded == 0 && count != 0) {
				t.Fatalf("uploaded history lost: count=%d err=%v", count, err)
			}
			next := model.Changes{Key: "after-import", Obj: []byte(`{}`)}
			if err := db.Create(&next).Error; err != nil || next.Id <= marker.Id {
				t.Fatalf("subsequent auto ID did not advance: id=%d marker=%d err=%v", next.Id, marker.Id, err)
			}
		})
	}
}

func TestImportDBRejectsExhaustedChangeRevision(t *testing.T) {
	for _, location := range []string{"original", "uploaded"} {
		t.Run(location, func(t *testing.T) {
			source, data := importFixture(t)
			candidate := source
			if location == "uploaded" {
				candidate = readBackup(t, data)
			}
			if err := candidate.Create(&model.Changes{Id: maxSQLiteChangeID, Key: "history", Obj: []byte(`{}`)}).Error; err != nil {
				t.Fatal(err)
			}
			if location == "uploaded" {
				var err error
				data, err = exportDB(candidate, "")
				if err != nil {
					t.Fatal(err)
				}
			}
			err := importDB(backupUpload{bytes.NewReader(data)}, func() error { t.Fatal("restart with exhausted revision"); return nil })
			if err == nil || !strings.Contains(err.Error(), "revision exhausted") {
				t.Fatalf("exhausted revision not rejected: %v", err)
			}
			assertImportIdentity(t, "old")
			assertImportChangeCount(t, db, 0)
			if location == "uploaded" && db != source {
				t.Fatal("exhausted upload closed the original connection before validation finished")
			}
			if err := db.Create(&model.Setting{Key: "after-exhaustion-rejection", Value: "writable"}).Error; err != nil {
				t.Fatalf("original database not writable after exhausted revision rejection: %v", err)
			}
		})
	}
}

func TestMaxChangeIDPropagatesSchemaReadFailure(t *testing.T) {
	database := backupTestDB(t, filepath.Join(t.TempDir(), "source.db"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if revision, err := maxChangeID(database.WithContext(ctx)); !errors.Is(err, context.Canceled) || revision != 0 {
		t.Fatalf("schema read failure was mistaken for absent history: revision=%d err=%v", revision, err)
	}
}

func TestClosedDBChangeIDReadOnlyAndHistoryEdges(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.db")
	if _, err := closedDBChangeID(missing); err == nil {
		t.Fatal("missing original database accepted")
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only revision probe created a database: %v", err)
	}
	for _, kind := range []string{"empty", "missing-table", "large-id", "malformed-id"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "原库 #50% +.db")
			database := backupTestDB(t, path)
			var want uint64
			switch kind {
			case "missing-table":
				if err := database.Migrator().DropTable(&model.Changes{}); err != nil {
					t.Fatal(err)
				}
			case "large-id":
				want = 9007199254740993
				if err := database.Create(&model.Changes{Id: want, Obj: []byte(`{}`)}).Error; err != nil {
					t.Fatal(err)
				}
			case "malformed-id":
				for _, query := range []string{"DROP TABLE changes", "CREATE TABLE changes (id TEXT)", "INSERT INTO changes VALUES ('not-an-integer')"} {
					if err := database.Exec(query).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			_ = closedBackupBytes(t, database)
			revision, err := closedDBChangeID(path)
			if kind == "malformed-id" {
				if err == nil {
					t.Fatal("unreadable history revision accepted")
				}
			} else if err != nil || revision != want {
				t.Fatalf("closed revision=%d want %d err=%v", revision, want, err)
			}
		})
	}
}

// GORM asks GetDBConn for the pool immediately before closeCurrentDB closes it.
// Commit here to deterministically cover the gap after the checkpoint, without
// sleeping, racing the global db pointer, or using an external connection.
type finalCommitPool struct {
	*sql.DB
	commit func() error
}

func (p *finalCommitPool) GetDBConn() (*sql.DB, error) {
	if p.commit != nil {
		commit := p.commit
		p.commit = nil
		if err := commit(); err != nil {
			return nil, err
		}
	}
	return p.DB, nil
}

func TestImportDBRevisionIncludesOldPoolFinalCommit(t *testing.T) {
	source, data := importFixture(t)
	if err := source.Create(&model.Changes{Id: 1000, Key: "earlier-history", Obj: []byte(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := source.DB()
	if err != nil {
		t.Fatal(err)
	}
	committed := false
	pool := &finalCommitPool{DB: sqlDB, commit: func() error {
		_, err := sqlDB.Exec("INSERT INTO changes (id, key, obj) VALUES (3000, 'final-old-commit', ?)", []byte(`{}`))
		committed = err == nil
		return err
	}}
	source.ConnPool, source.Statement.ConnPool = pool, pool
	err = importDB(backupUpload{bytes.NewReader(data)}, func() error {
		if !committed {
			t.Fatal("old pool's final commit was not exercised")
		}
		var final model.Changes
		if err := retainedImportFallback(t).Where("key = ?", "final-old-commit").First(&final).Error; err != nil || final.Id != 3000 {
			t.Fatalf("fallback lost the final committed old revision: %#v err=%v", final, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	assertImportMarker(t, db, 3001)
}

func TestImportDBRevisionIncludesInitializationChanges(t *testing.T) {
	for _, duringInit := range []uint64{3000, maxSQLiteChangeID} {
		t.Run(fmt.Sprint(duringInit), func(t *testing.T) {
			source, data := importFixture(t)
			if err := source.Create(&model.Changes{Id: 1000, Key: "original-history", Obj: []byte(`{}`)}).Error; err != nil {
				t.Fatal(err)
			}
			uploadDB := readBackup(t, data)
			if err := uploadDB.Create(&model.Changes{Id: 20, Key: "uploaded-history", Obj: []byte(`{}`)}).Error; err != nil {
				t.Fatal(err)
			}
			spec := legacyBootstrapOnlyTLSSpecs[0]
			legacy, err := buildBootstrapOnlySelfSignedTLS(spec.name, spec.serverName, spec.alpn)
			if err != nil {
				t.Fatal(err)
			}
			legacy.Name = spec.name
			if err := uploadDB.Create(&legacy).Error; err != nil {
				t.Fatal(err)
			}
			// This trigger runs during the real InitDB TLS rename, before the
			// mock restart. A pre-initialization MAX(id) cannot see its record.
			query := fmt.Sprintf(`CREATE TRIGGER record_init_history AFTER UPDATE OF name ON tls
				WHEN OLD.id = %d BEGIN INSERT INTO changes (id, key, obj) VALUES (%d, 'init-history', CAST('{}' AS BLOB)); END`, legacy.Id, duringInit)
			if err := uploadDB.Exec(query).Error; err != nil {
				t.Fatal(err)
			}
			data = closedBackupBytes(t, uploadDB)
			restarts := 0
			err = importDB(backupUpload{bytes.NewReader(data)}, func() error {
				restarts++
				var change model.Changes
				if err := db.Where("key = ?", "init-history").First(&change).Error; err != nil || change.Id != duringInit {
					t.Fatalf("initialization change was not exercised: %#v err=%v", change, err)
				}
				assertImportChangeCount(t, db, 0)
				return nil
			})
			if restarts != 1 {
				t.Fatalf("restart callback count=%d want 1", restarts)
			}
			if duringInit == maxSQLiteChangeID {
				if err == nil || !strings.Contains(err.Error(), "revision exhausted") {
					t.Fatalf("initialization revision exhaustion reported success: %v", err)
				}
				assertImportChangeCount(t, db, 0)
				retainedImportFallback(t)
			} else {
				if err != nil {
					t.Fatal(err)
				}
				assertImportMarker(t, db, duringInit+1)
			}
		})
	}
}

func TestImportDBRestartRevisionExhaustionRetainsFallback(t *testing.T) {
	_, data := importFixture(t)
	err := importDB(backupUpload{bytes.NewReader(data)}, func() error {
		return db.Create(&model.Changes{Id: maxSQLiteChangeID, Key: "restart-history", Obj: []byte(`{}`)}).Error
	})
	if err == nil || !strings.Contains(err.Error(), "revision exhausted") {
		t.Fatalf("restart revision exhaustion reported success: %v", err)
	}
	assertImportChangeCount(t, db, 0)
	retainedImportFallback(t)
}

func TestImportDBAllocatesLastAvailableChangeID(t *testing.T) {
	source, data := importFixture(t)
	if err := source.Create(&model.Changes{Id: maxSQLiteChangeID - 1, Obj: []byte(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := importDB(backupUpload{bytes.NewReader(data)}, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	assertImportMarker(t, db, maxSQLiteChangeID)
	if err := closeCurrentDB(); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(config.GetDBPath()); err != nil {
		t.Fatal(err)
	}
	assertImportMarker(t, db, maxSQLiteChangeID)
}

func TestImportDBMarkerFailureRetainsFallbackWithoutPartialChange(t *testing.T) {
	_, data := importFixture(t)
	err := importDB(backupUpload{bytes.NewReader(data)}, func() error {
		return db.Exec("CREATE TRIGGER reject_import_marker BEFORE INSERT ON changes BEGIN SELECT RAISE(ABORT, 'fixture marker failure'); END").Error
	})
	if err == nil || !strings.Contains(err.Error(), "record database import change") {
		t.Fatalf("import marker failure not reported: %v", err)
	}
	assertImportIdentity(t, "uploaded")
	assertImportChangeCount(t, db, 0)
	files, err := filepath.Glob(filepath.Join(config.GetDBFolderPath(), ".s-ui-fallback-*.db"))
	if err != nil || len(files) != 1 {
		t.Fatalf("marker failure lost fallback: %v err=%v", files, err)
	}
	data, err = os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	fallback := readBackup(t, data)
	assertImportChangeCount(t, fallback, 0)
	var identity model.Setting
	if err := fallback.Where("key = ?", "identity").First(&identity).Error; err != nil || identity.Value != "old" {
		t.Fatalf("wrong fallback identity: %q err=%v", identity.Value, err)
	}
}

func TestImportDBRejectsCorruptUploadWithoutClosingActiveDB(t *testing.T) {
	source, _ := importFixture(t)
	data := append([]byte("SQLite format 3\x00"), make([]byte, 100)...)
	if err := importDB(backupUpload{bytes.NewReader(data)}, func() error { t.Fatal("restart on corrupt upload"); return nil }); err == nil {
		t.Fatal("corrupt upload accepted")
	}
	if db != source {
		t.Fatal("validation failure replaced active connection")
	}
	assertImportIdentity(t, "old")
}

func TestImportDBRejectsForeignOrAccountlessSQLiteBeforeClosingLiveDB(t *testing.T) {
	for _, kind := range []string{"empty-schema", "foreign", "settings-only", "accountless", "blank-account"} {
		t.Run(kind, func(t *testing.T) {
			source, _ := importFixture(t)
			path := filepath.Join(t.TempDir(), "upload.db")
			uploadDB, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := uploadDB.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			if err := uploadDB.Exec("CREATE TABLE books (title TEXT)").Error; err != nil {
				t.Fatal(err)
			}
			if kind == "empty-schema" {
				if err := uploadDB.Exec("DROP TABLE books").Error; err != nil {
					t.Fatal(err)
				}
			}
			if kind == "settings-only" || kind == "accountless" || kind == "blank-account" {
				if err := uploadDB.AutoMigrate(&model.Setting{}); err != nil {
					t.Fatal(err)
				}
				if err := uploadDB.Create(&model.Setting{Key: "schemaVersion", Value: config.GetDatabaseSchemaVersion()}).Error; err != nil {
					t.Fatal(err)
				}
				if kind == "accountless" || kind == "blank-account" {
					if err := uploadDB.AutoMigrate(&model.User{}, &model.Client{}, &model.Tls{}); err != nil {
						t.Fatal(err)
					}
					if kind == "blank-account" {
						if err := uploadDB.Create(&model.User{Username: "admin", Password: "   "}).Error; err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			if err := sqlDB.Close(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// A local production fixture prevents public-IP probes if the bug
			// accidentally treats a settings-only database as a fresh install.
			cert, key := productionCertificate(t, "vpn.example.test", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
			configureProductionTLS(t, "vpn.example.test", cert, key)
			unsetEnvForTest(t, "SUI_INITIAL_ADMIN_USERNAME")
			unsetEnvForTest(t, "SUI_INITIAL_ADMIN_PASSWORD")
			oldPassword := generatedInitialAdminPassword
			generatedInitialAdminPassword = ""
			t.Cleanup(func() { generatedInitialAdminPassword = oldPassword })
			restarts := 0
			err = importDB(backupUpload{bytes.NewReader(data)}, func() error { restarts++; return nil })
			if err == nil || restarts != 0 || db != source || generatedInitialAdminPassword != "" {
				t.Fatalf("invalid upload was not rejected before replacement: err=%v restarts=%d sameDB=%v generatedAdmin=%v", err, restarts, db == source, generatedInitialAdminPassword != "")
			}
			assertImportIdentity(t, "old")
		})
	}
}

func TestValidateImportedSUIDBRejectsIncompleteCoreSchema(t *testing.T) {
	for _, missing := range []string{"settings", "users", "clients", "tls", "clients.config"} {
		t.Run(missing, func(t *testing.T) {
			candidate := backupTestDB(t, filepath.Join(t.TempDir(), "candidate.db"))
			if err := candidate.Create(&model.User{Username: "admin", Password: "keep-hash"}).Error; err != nil {
				t.Fatal(err)
			}
			if err := validateImportedSUIDB(candidate); err != nil {
				t.Fatalf("complete core fixture rejected: %v", err)
			}
			query := "DROP TABLE " + missing
			if missing == "clients.config" {
				query = "ALTER TABLE clients DROP COLUMN config"
			}
			if err := candidate.Exec(query).Error; err != nil {
				t.Fatal(err)
			}
			if err := validateImportedSUIDB(candidate); err == nil || !strings.Contains(err.Error(), missing) {
				t.Fatalf("missing core schema %s not rejected: %v", missing, err)
			}
		})
	}
}

func TestImportDBAcceptsLegacyReleaseWithoutNewTablesOrSchemaMarker(t *testing.T) {
	for _, version := range []string{"1.1.9", "1.2.8", "1.4.9"} {
		t.Run(version, func(t *testing.T) {
			_, data := importFixture(t)
			// Older migrations may look next to the executable for config.json;
			// pin that lookup to an isolated directory, never an installed config.
			oldArgs := os.Args
			os.Args = append([]string{filepath.Join(t.TempDir(), "sui-test.exe")}, oldArgs[1:]...)
			t.Cleanup(func() { os.Args = oldArgs })
			t.Setenv("SUI_BIN_FOLDER", "isolated-legacy-bin")
			uploadDB := readBackup(t, data)
			if err := uploadDB.Exec("DELETE FROM settings WHERE key = 'schemaVersion'").Error; err != nil {
				t.Fatal(err)
			}
			for _, setting := range []model.Setting{{Key: "version", Value: version}, {Key: "config", Value: `{}`}} {
				if err := uploadDB.Create(&setting).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := uploadDB.Migrator().DropTable(&model.XrayXHTTPClient{}, &model.XrayXHTTPInbound{}); err != nil {
				t.Fatal(err)
			}
			if version == "1.1.9" {
				if err := uploadDB.Exec("ALTER TABLE changes ADD COLUMN `index` INTEGER").Error; err != nil {
					t.Fatal(err)
				}
				client := model.Client{Name: "legacy-client", Config: []byte(`{"trojan":{"password":"legacy-password"}}`), Inbounds: []byte(`[]`), Links: []byte(`[]`)}
				if err := uploadDB.Create(&client).Error; err != nil {
					t.Fatal(err)
				}
			}
			var seq int
			var name, path string
			if err := uploadDB.Raw("PRAGMA database_list").Row().Scan(&seq, &name, &path); err != nil {
				t.Fatal(err)
			}
			sqlDB, err := uploadDB.DB()
			if err != nil {
				t.Fatal(err)
			}
			if err := sqlDB.Close(); err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			restarts := 0
			if err := importDB(backupUpload{bytes.NewReader(data)}, func() error { restarts++; return nil }); err != nil {
				t.Fatalf("valid legacy import rejected: %v", err)
			}
			if restarts != 1 {
				t.Fatalf("restart count=%d", restarts)
			}
			assertImportIdentity(t, "uploaded")
			var user model.User
			if err := db.First(&user).Error; err != nil || user.Username != "existing-admin" || user.Password != "keep-hash" {
				t.Fatalf("legacy credentials changed: %#v err=%v", user, err)
			}
			var marker model.Setting
			if err := db.Where("key = ?", "schemaVersion").First(&marker).Error; err != nil || marker.Value != config.GetDatabaseSchemaVersion() {
				t.Fatalf("legacy migration did not record schema: %#v err=%v", marker, err)
			}
		})
	}
}

func TestImportDBRefusesBusyCheckpointAndRetainsWAL(t *testing.T) {
	source, upload := importFixture(t)
	sqlDB, _ := source.DB()
	reader, err := sqlDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Rollback()
	var count int
	if err := reader.QueryRow("SELECT count(*) FROM settings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	// Add committed WAL frames beyond the reader's snapshot.
	if err := source.Create(&model.Setting{Key: "wal-only", Value: "must-survive"}).Error; err != nil {
		t.Fatal(err)
	}
	walBefore, err := os.ReadFile(config.GetDBPath() + "-wal")
	if err != nil || len(walBefore) == 0 {
		t.Fatalf("missing WAL fixture: %v", err)
	}
	err = importDB(backupUpload{bytes.NewReader(upload)}, func() error { t.Fatal("restart on busy checkpoint"); return nil })
	if err == nil || !strings.Contains(err.Error(), "checkpoint") {
		t.Fatalf("busy checkpoint was not rejected: %v", err)
	}
	walAfter, err := os.ReadFile(config.GetDBPath() + "-wal")
	if err != nil || !bytes.Equal(walBefore, walAfter) || db != source {
		t.Fatalf("busy import modified live DB/WAL: %v", err)
	}
	var setting model.Setting
	if err := source.Where("key = ?", "wal-only").First(&setting).Error; err != nil || setting.Value != "must-survive" {
		t.Fatalf("committed WAL data lost: %v", err)
	}
}

func TestImportDBSuccessAndMigrationRollback(t *testing.T) {
	for _, failMigration := range []bool{false, true} {
		t.Run(fmt.Sprint(failMigration), func(t *testing.T) {
			_, data := importFixture(t)
			if failMigration {
				uploadDB := readBackup(t, data)
				if err := uploadDB.Model(&model.Setting{}).Where("key = ?", "schemaVersion").Update("value", "invalid").Error; err != nil {
					t.Fatal(err)
				}
				var err error
				data, err = exportDB(uploadDB, "")
				if err != nil {
					t.Fatal(err)
				}
			}
			restarts := 0
			err := importDB(backupUpload{bytes.NewReader(data)}, func() error { restarts++; return nil })
			if failMigration {
				if err == nil || restarts != 0 {
					t.Fatalf("migration failure err=%v restarts=%d", err, restarts)
				}
				assertImportIdentity(t, "old")
				assertImportChangeCount(t, db, 0)
			} else {
				if err != nil || restarts != 1 {
					t.Fatalf("import err=%v restarts=%d", err, restarts)
				}
				assertImportIdentity(t, "uploaded")
			}
			files, _ := filepath.Glob(filepath.Join(config.GetDBFolderPath(), ".s-ui-*.db*"))
			if len(files) != 0 {
				t.Fatalf("temporary import files leaked: %v", files)
			}
		})
	}
}

func TestImportDBRestartFailureRetainsFallback(t *testing.T) {
	source, data := importFixture(t)
	if err := source.Create(&model.Changes{Id: 1000, Key: "original-history", Obj: []byte(`{}`)}).Error; err != nil {
		t.Fatal(err)
	}
	restarts := 0
	err := importDB(backupUpload{bytes.NewReader(data)}, func() error {
		restarts++
		assertImportChangeCount(t, db, 0)
		if err := db.Create(&model.Changes{Id: 2000, Key: "failed-restart-history", Obj: []byte(`{}`)}).Error; err != nil {
			return err
		}
		return errors.New("fixture restart failed")
	})
	if err == nil || !strings.Contains(err.Error(), "Error restarting app: fixture restart failed") || restarts != 1 {
		t.Fatalf("restart failure not reported: err=%v calls=%d", err, restarts)
	}
	importErr := err
	assertImportIdentity(t, "uploaded")
	assertImportChangeCount(t, db, 0)
	if err := closeCurrentDB(); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(config.GetDBPath()); err != nil {
		t.Fatal(err)
	}
	assertImportChangeCount(t, db, 0)
	var failedRestartChange model.Changes
	if err := db.Where("key = ?", "failed-restart-history").First(&failedRestartChange).Error; err != nil || failedRestartChange.Id != 2000 {
		t.Fatalf("failed restart fixture did not persist: %#v err=%v", failedRestartChange, err)
	}
	fallback := retainedImportFallback(t)
	if revision, err := maxChangeID(fallback); err != nil || revision != 1000 {
		t.Fatalf("fallback lost committed original revision: %d err=%v", revision, err)
	}
	files, err := filepath.Glob(filepath.Join(config.GetDBFolderPath(), ".s-ui-fallback-*.db"))
	if err != nil || len(files) != 1 {
		t.Fatalf("fallback not retained: %v err=%v", files, err)
	}
	if !strings.Contains(importErr.Error(), files[0]) {
		t.Fatal("restart failure did not identify the retained recovery file")
	}
}

func TestImportDBInitializationFailureRestoresUsableDB(t *testing.T) {
	source, data := importFixture(t)
	uploadDB := readBackup(t, data)
	spec := legacyBootstrapOnlyTLSSpecs[0]
	legacy, err := buildBootstrapOnlySelfSignedTLS(spec.name, spec.serverName, spec.alpn)
	if err != nil {
		t.Fatal(err)
	}
	legacy.Name = spec.name
	if err := uploadDB.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	// Fail only the uploaded DB's legacy-name initialization, after it opens.
	// Keep the existing user: accountless uploads are rejected before replacement.
	if err := uploadDB.Exec("CREATE TRIGGER reject_tls_rename BEFORE UPDATE OF name ON tls BEGIN SELECT RAISE(ABORT, 'fixture init failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	var seq int
	var name, path string
	if err := uploadDB.Raw("PRAGMA database_list").Row().Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := uploadDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	err = importDB(backupUpload{bytes.NewReader(data)}, func() error {
		t.Fatal("restart after failed initialization")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "Error initializing db") {
		t.Fatalf("initialization failure not reported: %v", err)
	}
	assertImportIdentity(t, "old")
	if db == source {
		t.Fatal("rollback reused the closed original connection")
	}
	assertImportChangeCount(t, db, 0)
	if err := db.Create(&model.Setting{Key: "after-rollback", Value: "writable"}).Error; err != nil {
		t.Fatalf("restored database was closed or removed: %v", err)
	}
	if _, err := GetDb(""); err != nil {
		t.Fatalf("cannot export restored database: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(config.GetDBFolderPath(), ".s-ui-*.db*"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary import files leaked: %v err=%v", files, err)
	}
}

func TestRestoreFallbackRejectsInvalidSourceWithoutClosingLiveDB(t *testing.T) {
	for _, kind := range []string{"missing", "current", "directory"} {
		t.Run(kind, func(t *testing.T) {
			source, _ := importFixture(t)
			fallback := filepath.Join(config.GetDBFolderPath(), "missing-fallback.db")
			if kind == "current" {
				fallback = config.GetDBPath()
			} else if kind == "directory" {
				if err := os.Mkdir(fallback, 0700); err != nil {
					t.Fatal(err)
				}
			}
			walBefore, err := os.ReadFile(config.GetDBPath() + "-wal")
			if err != nil {
				t.Fatal(err)
			}
			if err := restoreFallbackDB(fallback); err == nil {
				t.Fatal("invalid fallback accepted")
			}
			walAfter, err := os.ReadFile(config.GetDBPath() + "-wal")
			if err != nil || !bytes.Equal(walBefore, walAfter) || db != source {
				t.Fatalf("invalid fallback modified live DB/WAL: %v", err)
			}
			assertImportIdentity(t, "old")
			if err := db.Create(&model.Setting{Key: "still-live", Value: kind}).Error; err != nil {
				t.Fatalf("active database not writable: %v", err)
			}
		})
	}
}

func TestRestoreFallbackRepeatedRollbackKeepsRestoredDB(t *testing.T) {
	_, data := importFixture(t)
	if err := importDB(backupUpload{bytes.NewReader(data)}, func() error { return errors.New("restart failed") }); err == nil {
		t.Fatal("restart failure ignored")
	}
	files, err := filepath.Glob(filepath.Join(config.GetDBFolderPath(), ".s-ui-fallback-*.db"))
	if err != nil || len(files) != 1 {
		t.Fatalf("missing recovery file: %v err=%v", files, err)
	}
	if err := restoreFallbackDB(files[0]); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(config.GetDBPath()); err != nil {
		t.Fatal(err)
	}
	restored := db
	if err := restoreFallbackDB(files[0]); err == nil {
		t.Fatal("consumed fallback accepted a second time")
	}
	if db != restored {
		t.Fatal("repeated rollback closed the restored database")
	}
	assertImportIdentity(t, "old")
	if err := db.Create(&model.Setting{Key: "after-second-rollback", Value: "writable"}).Error; err != nil {
		t.Fatalf("restored database not writable: %v", err)
	}
}

func TestRestoreFallbackFailurePreservesRecoveryFile(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	useBackupTestDB(t, nil)
	if err := os.MkdirAll(filepath.Join(config.GetDBPath(), "nonempty"), 0700); err != nil {
		t.Fatal(err)
	}
	fallback := filepath.Join(config.GetDBFolderPath(), "recovery.db")
	if err := os.WriteFile(fallback, []byte("recovery-content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := restoreFallbackDB(fallback); err == nil {
		t.Fatal("rollback unexpectedly replaced a nonempty directory")
	}
	data, err := os.ReadFile(fallback)
	if err != nil || string(data) != "recovery-content" {
		t.Fatalf("failed rollback destroyed fallback: %v", err)
	}
}

func TestRestoreFallbackBusyCheckpointPreservesBothDatabases(t *testing.T) {
	source, data := importFixture(t)
	fallback := filepath.Join(config.GetDBFolderPath(), "recovery.db")
	if err := os.WriteFile(fallback, data, 0600); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := source.DB()
	if err != nil {
		t.Fatal(err)
	}
	reader, err := sqlDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Rollback()
	var count int
	if err := reader.QueryRow("SELECT count(*) FROM settings").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := source.Create(&model.Setting{Key: "wal-only", Value: "must-survive"}).Error; err != nil {
		t.Fatal(err)
	}
	walBefore, err := os.ReadFile(config.GetDBPath() + "-wal")
	if err != nil {
		t.Fatal(err)
	}
	if err := restoreFallbackDB(fallback); err == nil || !strings.Contains(err.Error(), "checkpoint") {
		t.Fatalf("busy rollback not rejected: %v", err)
	}
	walAfter, err := os.ReadFile(config.GetDBPath() + "-wal")
	if err != nil || !bytes.Equal(walBefore, walAfter) || db != source {
		t.Fatalf("busy rollback modified live DB/WAL: %v", err)
	}
	fallbackAfter, err := os.ReadFile(fallback)
	if err != nil || !bytes.Equal(data, fallbackAfter) {
		t.Fatalf("busy rollback destroyed recovery database: %v", err)
	}
	assertImportIdentity(t, "old")
	var setting model.Setting
	if err := source.Where("key = ?", "wal-only").First(&setting).Error; err != nil || setting.Value != "must-survive" {
		t.Fatalf("committed WAL data lost: %v", err)
	}
}

type shortSQLiteReader struct{ io.Reader }

func (r shortSQLiteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.Reader.Read(p)
}

func TestIsSQLiteDBAcceptsShortReadsAndRejectsTruncation(t *testing.T) {
	ok, err := IsSQLiteDB(shortSQLiteReader{strings.NewReader("SQLite format 3\x00")})
	if err != nil || !ok {
		t.Fatalf("valid short reads rejected: ok=%v err=%v", ok, err)
	}
	if ok, err := IsSQLiteDB(strings.NewReader("SQLite")); ok || err == nil {
		t.Fatalf("truncated header accepted: ok=%v err=%v", ok, err)
	}
}
