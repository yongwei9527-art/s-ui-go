package migration

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/yongwei9527-art/s-ui-go/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func MigrateDb() error {
	// void running on first install
	path := config.GetDBPath()
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		println("Database not found")
		return nil
	}
	if err != nil {
		return err
	}

	db, err := gorm.Open(sqlite.Open(path))
	if err != nil {
		return err
	}
	defer func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	}()
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	shouldRollback := true
	defer func() {
		if shouldRollback {
			tx.Rollback()
		}
	}()
	currentVersion := config.GetVersion()
	dbVersion := ""
	if err = tx.Raw("SELECT value FROM settings WHERE key = ?", "version").Find(&dbVersion).Error; err != nil {
		return err
	}
	fmt.Println("Current version:", currentVersion, "\nDatabase version:", dbVersion)

	if currentVersion == dbVersion {
		fmt.Println("Database is up to date, no need to migrate")
		return nil
	}

	fmt.Println("Start migrating database...")

	// Before 1.2
	if dbVersion == "" {
		err = to1_1(tx)
		if err != nil {
			return fmt.Errorf("migration to 1.1 failed: %w", err)
		}
		err = to1_2(tx)
		if err != nil {
			return fmt.Errorf("migration to 1.2 failed: %w", err)
		}
		dbVersion = "1.2"
	}

	// Before 1.3
	if strings.HasPrefix(dbVersion, "1.2") {
		err = to1_3(tx)
		if err != nil {
			return fmt.Errorf("migration to 1.3 failed: %w", err)
		}
	}

	// Set version
	err = tx.Exec("UPDATE settings SET value = ? WHERE key = ?", currentVersion, "version").Error
	if err != nil {
		return fmt.Errorf("update version failed: %w", err)
	}
	if err = tx.Commit().Error; err != nil {
		return err
	}
	shouldRollback = false
	fmt.Println("Migration done!")
	return nil
}
