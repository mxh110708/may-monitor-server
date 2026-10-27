package dbcore

import (
	"fmt"

	"gorm.io/gorm"
)

// Validate before AutoMigrate: retired database layouts must never be silently
// treated as a fresh May installation or have their archived history discarded.
func validateDatabaseSchema(db *gorm.DB) error {
	if db.Migrator().HasTable("configs") {
		columns, err := db.Migrator().ColumnTypes("configs")
		if err != nil {
			return fmt.Errorf("inspect configuration schema: %w", err)
		}
		for _, column := range columns {
			if column.Name() == "sitename" || column.Name() == "id" {
				return fmt.Errorf("unsupported pre-May configuration schema; restore the archived service version to export it, or use a separate fresh May data directory")
			}
		}
	}
	for _, table := range []string{"client_infos", "records", "records_long_term", "gpu_records", "ping_records"} {
		if db.Migrator().HasTable(table) {
			return fmt.Errorf("unsupported pre-May table %s; the original database was left intact, use the archived service version to export it", table)
		}
	}
	return nil
}
