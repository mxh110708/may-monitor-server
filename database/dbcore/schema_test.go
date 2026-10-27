package dbcore

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDatabaseSchemaBoundary(t *testing.T) {
	for _, schema := range []string{
		"",
		"CREATE TABLE configs (key TEXT PRIMARY KEY, value TEXT)",
		"CREATE TABLE configs (id INTEGER PRIMARY KEY, sitename TEXT)",
		"CREATE TABLE client_infos (uuid TEXT PRIMARY KEY)",
		"CREATE TABLE records (time TEXT)",
		"CREATE TABLE records_long_term (time TEXT)",
		"CREATE TABLE gpu_records (time TEXT)",
		"CREATE TABLE ping_records (time TEXT)",
	} {
		t.Run(schema, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "schema.db")), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			if schema != "" {
				if err := db.Exec(schema).Error; err != nil {
					t.Fatal(err)
				}
			}
			var before, after []string
			db.Raw("SELECT sql FROM sqlite_master ORDER BY name").Scan(&before)
			err = validateDatabaseSchema(db)
			wantAllowed := schema == "" || schema == "CREATE TABLE configs (key TEXT PRIMARY KEY, value TEXT)"
			if (err == nil) != wantAllowed {
				t.Fatalf("allowed=%v, error=%v", wantAllowed, err)
			}
			db.Raw("SELECT sql FROM sqlite_master ORDER BY name").Scan(&after)
			if len(before) != len(after) {
				t.Fatal("schema inspection changed tables")
			}
			for i := range before {
				if before[i] != after[i] {
					t.Fatal("schema inspection changed definitions")
				}
			}
		})
	}
}
