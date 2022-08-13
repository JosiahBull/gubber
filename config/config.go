package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Token    string
	Location string
	Interval int
	Backups  int
}

func NewConfig() *Config {
	token := os.Getenv("GITHUB_TOKEN")
	location := os.Getenv("LOCATION")
	interval := os.Getenv("INTERVAL")
	backups := os.Getenv("BACKUPS")

	// parse interval as int
	interval_int, err := strconv.Atoi(interval)
	if err != nil {
		interval_int = 43200 // 12 hours
	}

	// parse backups as int
	backups_int, err := strconv.Atoi(backups)
	if err != nil {
		backups_int = 30
	}

	return &Config{
		Token:    token,
		Location: location,
		Interval: interval_int,
		Backups:  backups_int,
	}
}

func (c *Config) Validate() error {
	if c.Token == "" {
		return errors.New("token is empty")
	}
	if !strings.HasPrefix(c.Token, "ghp") {
		return errors.New("token must start with ghp")
	}
	if c.Location == "" {
		return errors.New("location is empty")
	}
	err := os.MkdirAll(c.Location, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create location due to error %w", err)
	}
	// if gubber.sh does not exist in the location, create it
	if _, err := os.Stat(c.Location + "/gubber"); os.IsNotExist(err) {
		//copy file from scripts/gubber.sh to location
		original, err := os.Open("scripts/gubber.sh")
		if err != nil {
			return fmt.Errorf("failed to open gubber.sh due to error %w", err)
		}
		defer original.Close()

		destination, err := os.Create(c.Location + "/gubber")
		if err != nil {
			return fmt.Errorf("failed to create gubber due to error %w", err)
		}
		defer destination.Close()

		_, err = io.Copy(destination, original)
		if err != nil {
			return fmt.Errorf("failed to copy gubber.sh due to error %w", err)
		}
	}

	if c.Interval == 0 {
		return errors.New("interval is empty")
	}

	return nil
}
