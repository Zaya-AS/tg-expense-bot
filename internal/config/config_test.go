package config

import (
	"net/url"
	"testing"
)

func TestLoadDatabaseURLFromPartsEscapesPassword(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "db")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASSWORD", "a@b:c/d#e")
	t.Setenv("DB_NAME", "expenses")

	value, err := loadDatabaseURL()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	password, ok := parsed.User.Password()
	if !ok || password != "a@b:c/d#e" {
		t.Fatalf("database password was not preserved in URL")
	}
	if parsed.Hostname() != "db" || parsed.Path != "/expenses" {
		t.Fatalf("unexpected database address: host=%q path=%q", parsed.Hostname(), parsed.Path)
	}
}
