package config_test

import (
	"os"
	"testing"

	"github.com/josiahbull/gubber/config"
)

func TestNewConfig(t *testing.T) {
	// save current env state
	token := os.Getenv("GITHUB_TOKEN")
	location := os.Getenv("LOCATION")
	interval := os.Getenv("INTERVAL")
	backups := os.Getenv("BACKUPS")

	// restore env state after test
	defer func() {
		os.Setenv("GITHUB_TOKEN", token)
		os.Setenv("LOCATION", location)
		os.Setenv("INTERVAL", interval)
		os.Setenv("BACKUPS", backups)
	}()

	// set env state for test
	os.Setenv("GITHUB_TOKEN", "ghp123")
	os.Setenv("LOCATION", "/tmp/gubber")
	os.Setenv("INTERVAL", "43300")
	os.Setenv("BACKUPS", "40")

	config := config.NewConfig()
	if config.Token != "ghp123" {
		t.Errorf("expected token to be ghp123, got %s", config.Token)
	}

	if config.Location != "/tmp/gubber" {
		t.Errorf("expected location to be /tmp/gubber, got %s", config.Location)
	}

	if config.Interval != 43300 {
		t.Errorf("expected interval to be 43300, got %d", config.Interval)
	}

	if config.Backups != 40 {
		t.Errorf("expected backups to be 40, got %d", config.Backups)
	}

	// assert.True(t, config.Validate() == nil)

	// test failure cases
	// os.Setenv("GITHUB_TOKEN", "")

}
