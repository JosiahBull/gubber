package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Token        string
	Location     string
	TempLocation string
	Interval     int
	Backups      int
}

func NewConfig() (*Config, error) {
	token := os.Getenv("GITHUB_TOKEN")
	location := os.Getenv("LOCATION")
	interval := os.Getenv("INTERVAL")
	backups := os.Getenv("BACKUPS")
	tmp_location := os.Getenv("TEMP_LOCATION")

	// ensure tmp_location exists on the filesystem
	if _, err := os.Stat(tmp_location); os.IsNotExist(err) {
		return nil, fmt.Errorf("temp location does not exist: %v", tmp_location)
	}

	// parse interval as int
	interval_int, err := strconv.Atoi(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval: %v", err)
	}

	// parse backups as int
	backups_int, err := strconv.Atoi(backups)
	if err != nil {
		return nil, fmt.Errorf("invalid backups: %v", err)
	}

	return &Config{
		Token:        token,
		Location:     location,
		Interval:     interval_int,
		Backups:      backups_int,
		TempLocation: tmp_location,
	}, nil
}
