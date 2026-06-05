package database

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func GetDb(exclude string) ([]byte, error) {
	exclude_changes, exclude_stats := false, false
	for _, table := range strings.Split(exclude, ",") {
		if table == "changes" {
			exclude_changes = true
		} else if table == "stats" {
			exclude_stats = true
		}
	}

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, config.GetName()+"_"+time.Now().Format("20060102-200203")+".db")

	backupDb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	defer func() {
		if sqlDB, e := backupDb.DB(); e == nil {
			_ = sqlDB.Close()
		}
	}()
	defer os.Remove(dbPath)

	err = backupDb.AutoMigrate(
		&model.Setting{},
		&model.Tls{},
		&model.Inbound{},
		&model.Outbound{},
		&model.Service{},
		&model.Endpoint{},
		&model.User{},
		&model.Tokens{},
		&model.Stats{},
		&model.Client{},
		&model.Changes{},
	)
	if err != nil {
		return nil, err
	}

	var settings []model.Setting
	var tls []model.Tls
	var inbound []model.Inbound
	var outbound []model.Outbound
	var services []model.Service
	var endpoint []model.Endpoint
	var users []model.User
	var tokens []model.Tokens
	var clients []model.Client
	var stats []model.Stats
	var changes []model.Changes

	// Perform scans and handle errors
	if err := db.Model(&model.Setting{}).Scan(&settings).Error; err != nil {
		return nil, err
	} else if len(settings) > 0 {
		if err := backupDb.Save(settings).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Tls{}).Scan(&tls).Error; err != nil {
		return nil, err
	} else if len(tls) > 0 {
		if err := backupDb.Save(tls).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Inbound{}).Scan(&inbound).Error; err != nil {
		return nil, err
	} else if len(inbound) > 0 {
		if err := backupDb.Save(inbound).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Outbound{}).Scan(&outbound).Error; err != nil {
		return nil, err
	} else if len(outbound) > 0 {
		if err := backupDb.Save(outbound).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Service{}).Scan(&services).Error; err != nil {
		return nil, err
	} else if len(services) > 0 {
		if err := backupDb.Save(services).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Endpoint{}).Scan(&endpoint).Error; err != nil {
		return nil, err
	} else if len(endpoint) > 0 {
		if err := backupDb.Save(endpoint).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.User{}).Scan(&users).Error; err != nil {
		return nil, err
	} else if len(users) > 0 {
		if err := backupDb.Save(users).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Tokens{}).Scan(&tokens).Error; err != nil {
		return nil, err
	} else if len(tokens) > 0 {
		if err := backupDb.Save(tokens).Error; err != nil {
			return nil, err
		}
	}
	if err := db.Model(&model.Client{}).Scan(&clients).Error; err != nil {
		return nil, err
	} else if len(clients) > 0 {
		if err := backupDb.Save(clients).Error; err != nil {
			return nil, err
		}
	}

	if !exclude_stats {
		if err := db.Model(&model.Stats{}).Scan(&stats).Error; err != nil {
			return nil, err
		}
		if len(stats) > 0 {
			if err := backupDb.Save(stats).Error; err != nil {
				return nil, err
			}
		}
	}
	if !exclude_changes {
		if err := db.Model(&model.Changes{}).Scan(&changes).Error; err != nil {
			return nil, err
		}
		if len(changes) > 0 {
			if err := backupDb.Save(changes).Error; err != nil {
				return nil, err
			}
		}
	}

	// Update WAL
	err = backupDb.Exec("PRAGMA wal_checkpoint;").Error
	if err != nil {
		return nil, err
	}

	bdb, _ := backupDb.DB()
	bdb.Close()

	// Open the file for reading
	file, err := os.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the file contents
	fileContents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return fileContents, nil
}

func closeCurrentDB() {
	if db == nil {
		return
	}
	if err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		logger.Warning("checkpoint current db failed:", err)
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB == nil {
		return
	}
	if err := sqlDB.Close(); err != nil {
		logger.Warning("close current db failed:", err)
	}
	db = nil
}

func removeDBSidecars(dbPath string) {
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.Warning("remove db sidecar failed:", err)
		}
	}
}

func restoreFallbackDB(fallbackPath string) error {
	closeCurrentDB()
	dbPath := config.GetDBPath()
	removeDBSidecars(dbPath)
	if err := os.Remove(dbPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(fallbackPath, dbPath)
}

func ImportDB(file multipart.File) error {
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

	// Save the file as temporary file
	tempPath := fmt.Sprintf("%s.temp", config.GetDBPath())
	// Remove the existing fallback file (if any) before creating one
	_, err = os.Stat(tempPath)
	if err == nil {
		errRemove := os.Remove(tempPath)
		if errRemove != nil {
			return common.NewErrorf("Error removing existing temporary db file: %v", errRemove)
		}
	}
	// Create the temporary file
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return common.NewErrorf("Error creating temporary db file: %v", err)
	}

	// Remove temp file before returning
	defer os.Remove(tempPath)

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
	newDb_db, _ := newDb.DB()
	if newDb_db != nil {
		newDb_db.Close()
	}

	// Close old DB only after the uploaded database has been written and validated.
	// If validation fails before this point, the running panel keeps its active DB connection.
	closeCurrentDB()
	removeDBSidecars(config.GetDBPath())
	removeDBSidecars(tempPath)

	// Backup the current database for fallback
	fallbackPath := fmt.Sprintf("%s.backup", config.GetDBPath())
	// Remove the existing fallback file (if any)
	_, err = os.Stat(fallbackPath)
	if err == nil {
		errRemove := os.Remove(fallbackPath)
		if errRemove != nil {
			return common.NewErrorf("Error removing existing fallback db file: %v", errRemove)
		}
	}
	// Move the current database to the fallback location
	err = os.Rename(config.GetDBPath(), fallbackPath)
	if err != nil {
		if reopenErr := InitDB(config.GetDBPath()); reopenErr != nil {
			return common.NewErrorf("Error backing up temporary db file: %v; failed to reopen current db: %v", err, reopenErr)
		}
		return common.NewErrorf("Error backing up temporary db file: %v", err)
	}

	// Remove the temporary file before returning
	defer os.Remove(fallbackPath)

	// Move temp to DB path
	err = os.Rename(tempPath, config.GetDBPath())
	if err != nil {
		errRename := restoreFallbackDB(fallbackPath)
		if errRename != nil {
			return common.NewErrorf("Error moving db file and restoring fallback: %v", errRename)
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
			return common.NewErrorf("Error migrating db and restoring fallback: %v", errRename)
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
			return common.NewErrorf("Error migrating db and restoring fallback: %v", errRename)
		}
		if reopenErr := InitDB(config.GetDBPath()); reopenErr != nil {
			return common.NewErrorf("Error migrating db: %v; failed to reopen restored db: %v", err, reopenErr)
		}
		return common.NewErrorf("Error migrating db: %v", err)
	}

	// Restart app
	err = SendSighup()
	if err != nil {
		return common.NewErrorf("Error restarting app: %v", err)
	}

	return nil
}

func IsSQLiteDB(file io.Reader) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.Read(buf)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

func SendSighup() error {
	// Get the current process
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}

	// Send SIGHUP to the current process
	go func() {
		time.Sleep(3 * time.Second)
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
