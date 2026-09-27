package db

import "testing"

func TestMigrationFilenamePattern(t *testing.T) {
	if !upMigrationName.MatchString("000001_create_core_tables.up.sql") {
		t.Fatal("initial migration filename was not recognized")
	}
	if upMigrationName.MatchString("000001_create_core_tables.down.sql") {
		t.Fatal("down migration must not be applied on startup")
	}
}
