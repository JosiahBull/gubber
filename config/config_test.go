package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewConfig_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("GITHUB_TOKEN", "ghp_testtoken123")
	t.Setenv("LOCATION", "/some/path")
	t.Setenv("INTERVAL", "3600")
	t.Setenv("BACKUPS", "7")
	t.Setenv("TEMP_LOCATION", tmpDir)

	cfg, err := NewConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Token != "ghp_testtoken123" {
		t.Errorf("Token = %q, want %q", cfg.Token, "ghp_testtoken123")
	}
	if cfg.Location != "/some/path" {
		t.Errorf("Location = %q, want %q", cfg.Location, "/some/path")
	}
	if cfg.Interval != 3600 {
		t.Errorf("Interval = %d, want %d", cfg.Interval, 3600)
	}
	if cfg.Backups != 7 {
		t.Errorf("Backups = %d, want %d", cfg.Backups, 7)
	}
	if cfg.TempLocation != tmpDir {
		t.Errorf("TempLocation = %q, want %q", cfg.TempLocation, tmpDir)
	}
}

func TestNewConfig_InvalidInterval(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("GITHUB_TOKEN", "tok")
	t.Setenv("LOCATION", "/loc")
	t.Setenv("INTERVAL", "notanumber")
	t.Setenv("BACKUPS", "5")
	t.Setenv("TEMP_LOCATION", tmpDir)

	_, err := NewConfig()
	if err == nil {
		t.Fatal("expected error for invalid INTERVAL, got nil")
	}
}

func TestNewConfig_InvalidBackups(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("GITHUB_TOKEN", "tok")
	t.Setenv("LOCATION", "/loc")
	t.Setenv("INTERVAL", "100")
	t.Setenv("BACKUPS", "abc")
	t.Setenv("TEMP_LOCATION", tmpDir)

	_, err := NewConfig()
	if err == nil {
		t.Fatal("expected error for invalid BACKUPS, got nil")
	}
}

func TestNewConfig_TempLocationNotExist(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "tok")
	t.Setenv("LOCATION", "/loc")
	t.Setenv("INTERVAL", "100")
	t.Setenv("BACKUPS", "5")
	t.Setenv("TEMP_LOCATION", filepath.Join(os.TempDir(), "nonexistent-gubber-test-dir"))

	_, err := NewConfig()
	if err == nil {
		t.Fatal("expected error for non-existent TEMP_LOCATION, got nil")
	}
}

func TestNewConfig_EmptyInterval(t *testing.T) {
	tmpDir := t.TempDir()

	t.Setenv("GITHUB_TOKEN", "tok")
	t.Setenv("LOCATION", "/loc")
	t.Setenv("INTERVAL", "")
	t.Setenv("BACKUPS", "5")
	t.Setenv("TEMP_LOCATION", tmpDir)

	_, err := NewConfig()
	if err == nil {
		t.Fatal("expected error for empty INTERVAL, got nil")
	}
}
