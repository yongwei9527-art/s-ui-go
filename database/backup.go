package database

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/yongwei9527-art/s-ui-go/cmd/migration"
	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/util/common"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// backupMu prevents imports from replacing the database during an export and
// serializes imports. It does not quiesce service handlers holding GetDB().
var backupMu sync.RWMutex

func GetDb(exclude string) ([]byte, error) {
	backupMu.RLock()
	defer backupMu.RUnlock()
	if db == nil {
		return nil, errors.New("database is not open")
	}
	return exportDB(db, exclude)
}

func exportDB(source *gorm.DB, exclude string) ([]byte, error) {
	// A private, unique file avoids same-second collisions and does not require
	// write access to the executable's directory. CreateTemp uses mode 0600.
	file, err := os.CreateTemp("", "s-ui-export-*.db")
	if err != nil {
		return nil, err
	}
	dbPath := file.Name()
	defer os.Remove(dbPath)
	if err := file.Close(); err != nil {
		return nil, err
	}
	backupDb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := backupDb.DB()
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()
	// The destination is private and uses a rollback journal, so the exported
	// main file never depends on a separate WAL file.
	if err := backupDb.Exec("PRAGMA journal_mode=DELETE").Error; err != nil {
		return nil, err
	}
	if err := backupDb.AutoMigrate(
		&model.Setting{}, &model.Tls{}, &model.Inbound{}, &model.Outbound{},
		&model.Service{}, &model.Endpoint{}, &model.User{}, &model.Tokens{},
		&model.Stats{}, &model.Client{}, &model.Changes{},
		&model.XrayXHTTPInbound{}, &model.XrayXHTTPClient{},
	); err != nil {
		return nil, err
	}
	excluded := make(map[string]bool)
	for _, table := range strings.Split(exclude, ",") {
		excluded[strings.TrimSpace(table)] = true
	}
	// All source reads share one SQLite snapshot, including committed WAL data.
	// The destination is committed only if every table has been copied.
	err = source.Transaction(func(readTx *gorm.DB) error {
		return backupDb.Transaction(func(writeTx *gorm.DB) error {
			rows := []interface{}{
				&[]model.Setting{}, &[]model.Tls{}, &[]model.Inbound{}, &[]model.Outbound{},
				&[]model.Service{}, &[]model.Endpoint{}, &[]model.User{}, &[]model.Tokens{},
				&[]model.Client{}, &[]model.XrayXHTTPInbound{}, &[]model.XrayXHTTPClient{},
			}
			if !excluded["stats"] {
				rows = append(rows, &[]model.Stats{})
			}
			if !excluded["changes"] {
				rows = append(rows, &[]model.Changes{})
			}
			for _, tableRows := range rows {
				// Maps retain explicit zero/false values instead of applying
				// GORM model defaults again (e.g. disabled Xray assignments).
				var values []map[string]interface{}
				result := readTx.Model(tableRows).Find(&values)
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected > 0 {
					if err := writeTx.Table(result.Statement.Table).Create(&values).Error; err != nil {
						return err
					}
				}
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Close(); err != nil {
		return nil, err
	}
	return os.ReadFile(dbPath)
}
func closeCurrentDB() error {
	if db == nil {
		return nil
	}
	// SQLite reports blocked checkpoints as a result row, not an SQL error.
	// Never discard a WAL that still contains committed pages.
	var busy, logPages, checkpointed int
	if err := db.Raw("PRAGMA wal_checkpoint(TRUNCATE)").Row().Scan(&busy, &logPages, &checkpointed); err != nil {
		return fmt.Errorf("checkpoint current db: %w", err)
	}
	if busy != 0 || (logPages >= 0 && checkpointed < logPages) {
		return fmt.Errorf("checkpoint current db incomplete: busy=%d pages=%d checkpointed=%d", busy, logPages, checkpointed)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close current db: %w", err)
	}
	db = nil
	return nil
}

func removeDBSidecars(dbPath string) error {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove db sidecar %s: %w", suffix, err)
		}
	}
	return nil
}

func restoreFallbackDB(fallbackPath string) error {
	// A consumed/missing fallback must not close or delete a database that has
	// already been restored. Also reject aliases of the active database itself.
	fallbackInfo, err := os.Lstat(fallbackPath)
	if err != nil {
		return fmt.Errorf("check fallback db: %w", err)
	}
	if !fallbackInfo.Mode().IsRegular() {
		return errors.New("fallback db is not a regular file")
	}
	dbPath := config.GetDBPath()
	if currentInfo, err := os.Stat(dbPath); err == nil && os.SameFile(fallbackInfo, currentInfo) {
		return errors.New("fallback db is the current database")
	}
	if err := closeCurrentDB(); err != nil {
		return err
	}
	if err := removeDBSidecars(dbPath); err != nil {
		return err
	}
	if err := os.Remove(dbPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(fallbackPath, dbPath)
}

func ImportDB(file multipart.File) error {
	return ImportDBContext(context.Background(), file)
}

// Import is recovery, not first boot. Require the core schema shared by legacy
// S-UI releases and an existing account, without requiring new schema markers,
// Xray tables, or the pre-1.2 externally stored inbound configuration.
func validateImportedSUIDB(candidate *gorm.DB) error {
	for _, required := range []struct {
		model   interface{}
		name    string
		columns []string
	}{
		{&model.Setting{}, "settings", []string{"id", "key", "value"}},
		{&model.User{}, "users", []string{"id", "username", "password"}},
		{&model.Client{}, "clients", []string{"id", "name", "config", "inbounds", "links"}},
		{&model.Tls{}, "tls", []string{"id", "name", "server", "client"}},
	} {
		if !candidate.Migrator().HasTable(required.model) {
			return fmt.Errorf("invalid S-UI database: missing %s table", required.name)
		}
		for _, column := range required.columns {
			if !candidate.Migrator().HasColumn(required.model, column) {
				return fmt.Errorf("invalid S-UI database: missing %s.%s column", required.name, column)
			}
		}
	}
	var accounts int64
	if err := candidate.Model(&model.User{}).Where("trim(username) <> '' AND trim(password) <> ''").Count(&accounts).Error; err != nil {
		return fmt.Errorf("validate imported S-UI accounts: %w", err)
	}
	if accounts == 0 {
		return errors.New("invalid S-UI database: no existing administrator credentials")
	}
	return nil
}

const maxSQLiteChangeID uint64 = 1<<63 - 1

func maxChangeID(database *gorm.DB) (uint64, error) {
	// Legacy databases and exports excluding history may have no changes table.
	// HasTable only returns bool and would hide a schema read failure as absence.
	var hasHistory bool
	if err := database.Raw("SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'changes')").Row().Scan(&hasHistory); err != nil {
		return 0, err
	}
	if !hasHistory {
		return 0, nil
	}
	var revision uint64
	err := database.Raw("SELECT COALESCE(MAX(id), 0) FROM changes").Row().Scan(&revision)
	return revision, err
}

func closedDBChangeID(dbPath string) (uint64, error) {
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return 0, err
	}
	uriPath := filepath.ToSlash(absPath)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath // file:///C:/... on Windows
	}
	// go-sqlite strips mode=ro from non-URI paths. Use an escaped file URI so
	// this probe cannot silently create a missing original database.
	dsn := (&url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro"}).String()
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return 0, err
	}
	sqlDB, err := database.DB()
	if err != nil {
		return 0, err
	}
	revision, readErr := maxChangeID(database)
	closeErr := sqlDB.Close()
	if readErr != nil {
		return 0, readErr
	}
	return revision, closeErr
}

func recordDBImportChange(originalRevision, uploadedRevision uint64) error {
	floor := max(originalRevision, uploadedRevision)
	secret, err := common.RandomWithError(32)
	if err != nil {
		return fmt.Errorf("rotate session signing key after database import: %w", err)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		// A backup can contain the same session key but different administrators.
		// Rebuilding the HTTP server alone would keep removed users' cookies valid.
		// Rotate every matching row, including legacy duplicate keys, atomically
		// with the successful import marker; do not rotate on a failed marker.
		settings := tx.Model(&model.Setting{}).Where("key = ?", "secret").Update("value", secret)
		if settings.Error != nil {
			return settings.Error
		}
		if settings.RowsAffected == 0 {
			if err := tx.Create(&model.Setting{Key: "secret", Value: secret}).Error; err != nil {
				return err
			}
		}
		// One atomic statement also includes changes committed to the new database
		// since restart, without a SELECT/INSERT race or a deferred read transaction.
		result := tx.Exec(`INSERT INTO changes (id, date_time, actor, key, action, obj)
		SELECT max(?, COALESCE(MAX(id), 0)) + 1, ?, 'system', 'database', 'import', ?
		FROM changes HAVING max(?, COALESCE(MAX(id), 0)) < ?`,
			floor, time.Now().Unix(), []byte(`{"operation":"database-import"}`), floor, maxSQLiteChangeID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("database import change revision exhausted")
		}
		return nil
	})
}

func importDB(file multipart.File, restart func() error) (resultErr error) {
	return importDBBeforeClose(file, restart, nil)
}

func importDBBeforeClose(file multipart.File, restart func() error, beforeClose func() error) (resultErr error) {
	backupMu.Lock()
	defer backupMu.Unlock()
	// Check if the file is a SQLite database
	isValidDb, err := IsSQLiteDB(file)
	if err != nil {
		return common.NewErrorf("Error checking db file format: %v", err)
	}
	if !isValidDb {
		return common.NewError("Invalid db file format")
	}

	// Reset the file reader to the beginning
	_, err = file.Seek(0, 0)
	if err != nil {
		return common.NewErrorf("Error resetting file reader: %v", err)
	}

	// Keep private unique upload/fallback files on the same filesystem so
	// renames are atomic, without overwriting another import's recovery data.
	tempFile, err := os.CreateTemp(filepath.Dir(config.GetDBPath()), ".s-ui-import-*.db")
	if err != nil {
		return common.NewErrorf("Error creating temporary db file: %v", err)
	}
	tempPath := tempFile.Name()

	// Remove temp file before returning
	defer os.Remove(tempPath)
	defer removeDBSidecars(tempPath)

	// Save uploaded file to temporary file
	_, err = io.Copy(tempFile, file)
	if err != nil {
		_ = tempFile.Close()
		return common.NewErrorf("Error saving db: %v", err)
	}
	if err = tempFile.Close(); err != nil {
		return common.NewErrorf("Error closing temporary db file: %v", err)
	}

	// Check if we can init db or not
	newDb, err := gorm.Open(sqlite.Open(tempPath), &gorm.Config{})
	if err != nil {
		return common.NewErrorf("Error checking db: %v", err)
	}
	newDbSQL, err := newDb.DB()
	if err != nil {
		return err
	}
	var integrity string
	checkErr := newDb.Raw("PRAGMA integrity_check").Row().Scan(&integrity)
	var validationErr error
	var uploadedRevision uint64
	if checkErr == nil && integrity == "ok" {
		validationErr = validateImportedSUIDB(newDb)
		if validationErr == nil {
			uploadedRevision, validationErr = maxChangeID(newDb)
			if validationErr == nil && uploadedRevision >= maxSQLiteChangeID {
				validationErr = errors.New("uploaded database change revision exhausted")
			}
		}
	}
	closeErr := newDbSQL.Close()
	if checkErr != nil || integrity != "ok" {
		return fmt.Errorf("invalid database integrity: %s (check error: %v)", integrity, checkErr)
	}
	if validationErr != nil {
		return validationErr
	}
	if closeErr != nil {
		return closeErr
	}

	// Close old DB only after the uploaded database has been written and validated.
	// If validation fails before this point, the running panel keeps its active DB connection.
	fallbackFile, err := os.CreateTemp(filepath.Dir(config.GetDBPath()), ".s-ui-fallback-*.db")
	if err != nil {
		return err
	}
	fallbackPath := fallbackFile.Name()
	if err := fallbackFile.Close(); err != nil {
		_ = os.Remove(fallbackPath)
		return err
	}
	if err := os.Remove(fallbackPath); err != nil {
		return err
	}
	if beforeClose != nil {
		if err := beforeClose(); err != nil {
			return err
		}
	}
	if err := closeCurrentDB(); err != nil {
		return err
	}
	// If preparation fails after closing, reopen the unchanged database.
	oldAtPath := true
	defer func() {
		if resultErr != nil && db == nil && oldAtPath {
			if reopenErr := InitDB(config.GetDBPath()); reopenErr != nil {
				resultErr = fmt.Errorf("%w; failed to reopen database: %v", resultErr, reopenErr)
			}
		}
	}()
	// Read after closing the old pool to include commits completed between the
	// checkpoint and Close. Outstanding transactions/external connections still
	// require cross-layer quiescing; Close alone does not provide that guarantee.
	originalRevision, err := closedDBChangeID(config.GetDBPath())
	if err != nil {
		return fmt.Errorf("read original database change revision: %w", err)
	}
	if originalRevision >= maxSQLiteChangeID {
		return errors.New("original database change revision exhausted")
	}
	if err := removeDBSidecars(config.GetDBPath()); err != nil {
		return err
	}
	if err := removeDBSidecars(tempPath); err != nil {
		return err
	}
	// Move the current database to the fallback location
	err = os.Rename(config.GetDBPath(), fallbackPath)
	if err != nil {
		if reopenErr := InitDB(config.GetDBPath()); reopenErr != nil {
			return common.NewErrorf("Error backing up temporary db file: %v; failed to reopen current db: %v", err, reopenErr)
		}
		return common.NewErrorf("Error backing up temporary db file: %v", err)
	}
	oldAtPath = false

	// No unconditional defer: a failed rollback must retain the only fallback.

	// Move temp to DB path
	err = os.Rename(tempPath, config.GetDBPath())
	if err != nil {
		errRename := restoreFallbackDB(fallbackPath)
		if errRename != nil {
			return common.NewErrorf("Error moving db file: %v; restoring fallback: %v; recovery file retained at %s", err, errRename, fallbackPath)
		}
		if reopenErr := InitDB(config.GetDBPath()); reopenErr != nil {
			return common.NewErrorf("Error moving db file: %v; failed to reopen restored db: %v", err, reopenErr)
		}
		return common.NewErrorf("Error moving db file: %v", err)
	}

	// Migrate DB
	err = migration.MigrateDb()
	if err != nil {
		errRename := restoreFallbackDB(fallbackPath)
		if errRename != nil {
			return common.NewErrorf("Error migrating db: %v; restoring fallback: %v; recovery file retained at %s", err, errRename, fallbackPath)
		}
		if reopenErr := InitDB(config.GetDBPath()); reopenErr != nil {
			return common.NewErrorf("Error migrating db: %v; failed to reopen restored db: %v", err, reopenErr)
		}
		return common.NewErrorf("Error migrating db: %v", err)
	}
	err = InitDB(config.GetDBPath())
	if err != nil {
		errRename := restoreFallbackDB(fallbackPath)
		if errRename != nil {
			return common.NewErrorf("Error initializing db: %v; restoring fallback: %v; recovery file retained at %s", err, errRename, fallbackPath)
		}
		if reopenErr := InitDB(config.GetDBPath()); reopenErr != nil {
			return common.NewErrorf("Error initializing db: %v; failed to reopen restored db: %v", err, reopenErr)
		}
		return common.NewErrorf("Error initializing db: %v", err)
	}

	// Restart app
	err = restart()
	if err != nil {
		return common.NewErrorf("Error restarting app: %v; recovery file retained at %s", err, fallbackPath)
	}
	if err := recordDBImportChange(originalRevision, uploadedRevision); err != nil {
		return fmt.Errorf("record database import change: %w; recovery file retained at %s", err, fallbackPath)
	}
	if err := os.Remove(fallbackPath); err != nil {
		return fmt.Errorf("import succeeded but fallback cleanup failed (%s): %w", fallbackPath, err)
	}

	return nil
}

func IsSQLiteDB(file io.Reader) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := io.ReadFull(file, buf)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

func SendSighup() error {
	decision := make(chan bool, 1)
	decision <- true
	return scheduleImportRestart(decision)
}

// Register the signal before recording success, but wait for the final import
// outcome so a slow marker/cleanup cannot race a three-second reload timer.
func scheduleImportRestart(decision <-chan bool) error {
	// Get the current process
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

	// Send SIGHUP to the current process
	go func() {
		defer process.Release()
		if !<-decision {
			return
		}
		time.Sleep(3 * time.Second)
		var err error
		if runtime.GOOS == "windows" {
			err = process.Kill()
		} else {
			err = process.Signal(syscall.SIGHUP)
		}
		if err != nil {
			logger.Error("send signal SIGHUP failed:", err)
		}
	}()
	return nil
}
