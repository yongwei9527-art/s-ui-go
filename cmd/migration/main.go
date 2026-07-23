package migration

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// schemaVersionSettingKey is separate from the historical "version" setting.
// The latter records a legacy product release (for example 1.4.x) and must not
// be overwritten when the product adopts a new release naming scheme.
const schemaVersionSettingKey = "schemaVersion"

func MigrateDb() error {
	// Avoid running on a first install. InitDB will create the current schema.
	path := config.GetDBPath()
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		println("Database not found")
		return nil
	}
	if err != nil {
		return err
	}
	return migrateDBPath(path)
}

func migrateDBPath(path string) error {
	db, err := gorm.Open(sqlite.Open(path))
	if err != nil {
		return err
	}
	defer func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	}()
	return migrateDatabase(db)
}

func migrateDatabase(db *gorm.DB) error {
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	shouldRollback := true
	defer func() {
		if shouldRollback {
			_ = tx.Rollback().Error
		}
	}()

	currentSchemaVersion := config.GetDatabaseSchemaVersion()
	dbSchemaVersion, hasSchemaVersion, legacyReleaseVersion, err := readDatabaseSchemaVersion(tx, currentSchemaVersion)
	if err != nil {
		return err
	}

	fmt.Println("Release version:", config.GetVersion(), "\nLegacy release version:", legacyReleaseVersion, "\nCurrent database schema version:", currentSchemaVersion, "\nDatabase schema version:", displaySchemaVersion(dbSchemaVersion))

	if dbSchemaVersion == "" {
		fmt.Println("Start migrating database...")
		if err = to1_1(tx); err != nil {
			return fmt.Errorf("migration to 1.1 failed: %w", err)
		}
		if err = to1_2(tx); err != nil {
			return fmt.Errorf("migration to 1.2 failed: %w", err)
		}
		dbSchemaVersion = "1.2"
	}

	comparison, err := compareSchemaVersions(dbSchemaVersion, currentSchemaVersion)
	if err != nil {
		return fmt.Errorf("compare database schema versions: %w", err)
	}
	switch {
	case comparison > 0:
		// Never downgrade or rewrite a database created by a newer schema.
		fmt.Println("Database schema is newer than this binary; no migration was applied")
		return nil
	case comparison == 0:
		if hasSchemaVersion {
			fmt.Println("Database schema is up to date, no need to migrate")
			return nil
		}
		// A legacy release at or past the latest known migration level did not
		// have a schemaVersion marker. Record it once without changing the
		// historical release version stored in settings.version.
		if err = writeDatabaseSchemaVersion(tx, currentSchemaVersion); err != nil {
			return err
		}
		if err = tx.Commit().Error; err != nil {
			return err
		}
		shouldRollback = false
		fmt.Println("Database schema version recorded")
		return nil
	}

	fmt.Println("Start migrating database...")
	major, minor, err := parseSchemaVersion(dbSchemaVersion)
	if err != nil {
		return err
	}
	if major == 1 && minor == 2 {
		if err = to1_3(tx); err != nil {
			return fmt.Errorf("migration to 1.3 failed: %w", err)
		}
		dbSchemaVersion = "1.3"
	}

	comparison, err = compareSchemaVersions(dbSchemaVersion, currentSchemaVersion)
	if err != nil {
		return fmt.Errorf("compare migrated database schema versions: %w", err)
	}
	if comparison != 0 {
		return fmt.Errorf("no migration path from database schema %s to %s", dbSchemaVersion, currentSchemaVersion)
	}
	if err = writeDatabaseSchemaVersion(tx, currentSchemaVersion); err != nil {
		return err
	}
	if err = tx.Commit().Error; err != nil {
		return err
	}
	shouldRollback = false
	fmt.Println("Migration done!")
	return nil
}

func readDatabaseSchemaVersion(tx *gorm.DB, currentSchemaVersion string) (schemaVersion string, hasSchemaVersion bool, legacyReleaseVersion string, err error) {
	schemaVersion, hasSchemaVersion, err = readSetting(tx, schemaVersionSettingKey)
	if err != nil || hasSchemaVersion {
		return schemaVersion, hasSchemaVersion, "", err
	}

	legacyReleaseVersion, _, err = readSetting(tx, "version")
	if err != nil {
		return "", false, "", err
	}
	schemaVersion, err = inferLegacySchemaVersion(legacyReleaseVersion, currentSchemaVersion)
	return schemaVersion, false, legacyReleaseVersion, err
}

func readSetting(tx *gorm.DB, key string) (value string, found bool, err error) {
	var setting model.Setting
	err = tx.Where("key = ?", key).First(&setting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return setting.Value, true, nil
}

func writeDatabaseSchemaVersion(tx *gorm.DB, schemaVersion string) error {
	result := tx.Model(&model.Setting{}).Where("key = ?", schemaVersionSettingKey).Update("value", schemaVersion)
	if result.Error != nil {
		return fmt.Errorf("update database schema version: %w", result.Error)
	}
	if result.RowsAffected != 0 {
		return nil
	}
	if err := tx.Create(&model.Setting{Key: schemaVersionSettingKey, Value: schemaVersion}).Error; err != nil {
		return fmt.Errorf("create database schema version: %w", err)
	}
	return nil
}

// inferLegacySchemaVersion maps the old settings.version release marker to the
// last migration level it necessarily contains. It deliberately does not write
// back to settings.version: old 1.4.x databases keep their release metadata,
// while s-ui-go 0.1 gets its own display version from config.GetVersion().
func inferLegacySchemaVersion(legacyReleaseVersion, currentSchemaVersion string) (string, error) {
	legacyReleaseVersion = strings.TrimSpace(legacyReleaseVersion)
	if legacyReleaseVersion == "" {
		return "", nil
	}
	if legacyReleaseVersion == config.GetVersion() {
		// A database created by the current renamed product may not yet have
		// the new marker. Its schema is already created by this binary.
		return currentSchemaVersion, nil
	}

	major, minor, err := parseSchemaVersion(legacyReleaseVersion)
	if err != nil {
		return "", fmt.Errorf("cannot infer database schema from legacy release version %q: %w", legacyReleaseVersion, err)
	}
	switch {
	case major < 1 || (major == 1 && minor <= 1):
		return "", nil
	case major == 1 && minor == 2:
		return "1.2", nil
	default:
		// The explicit migrations in this codebase end at 1.3. Later 1.x
		// releases (including 1.4.x) must never be treated as a downgrade.
		return currentSchemaVersion, nil
	}
}

func compareSchemaVersions(left, right string) (int, error) {
	leftMajor, leftMinor, err := parseSchemaVersion(left)
	if err != nil {
		return 0, err
	}
	rightMajor, rightMinor, err := parseSchemaVersion(right)
	if err != nil {
		return 0, err
	}
	switch {
	case leftMajor < rightMajor || (leftMajor == rightMajor && leftMinor < rightMinor):
		return -1, nil
	case leftMajor > rightMajor || (leftMajor == rightMajor && leftMinor > rightMinor):
		return 1, nil
	default:
		return 0, nil
	}
}

// parseSchemaVersion intentionally compares only major/minor. Patch/suffix
// release labels (for example 1.4.7) were historically written to
// settings.version but did not identify a distinct migration step.
func parseSchemaVersion(value string) (major, minor int, err error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "v")
	parts := strings.Split(value, ".")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return 0, 0, fmt.Errorf("invalid version %q", value)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return 0, 0, fmt.Errorf("invalid version %q", value)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return 0, 0, fmt.Errorf("invalid version %q", value)
	}
	return major, minor, nil
}

func displaySchemaVersion(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(before 1.2)"
	}
	return value
}
