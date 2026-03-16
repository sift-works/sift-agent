package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Version is set at build time via -ldflags.
var Version = "dev"

type Config struct {
	Token             string
	DSN               string
	APIURL            string
	PollInterval      time.Duration
	HeartbeatInterval time.Duration
	LogLevel          string
}

func Parse() (Config, error) {
	var (
		token             string
		dsn               string
		apiURL            string
		pollInterval      int
		heartbeatInterval int
		logLevel          string
		showVersion       bool
	)

	flag.StringVar(&token, "token", "", "Sift API token")
	flag.StringVar(&dsn, "dsn", "", "Database connection string (mysql://user:pass@host:port/db)")
	flag.StringVar(&apiURL, "api-url", "https://app.sift.works", "Sift API base URL")
	flag.IntVar(&pollInterval, "poll-interval", 2, "Poll interval in seconds")
	flag.IntVar(&heartbeatInterval, "heartbeat-interval", 10, "Heartbeat interval in seconds")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(Version)
		os.Exit(0)
	}

	// Priority: flag > env var > default
	if token == "" {
		token = os.Getenv("SIFT_TOKEN")
	}
	if dsn == "" {
		dsn = os.Getenv("SIFT_DSN")
	}
	if !isFlagSet("api-url") {
		if v := os.Getenv("SIFT_API_URL"); v != "" {
			apiURL = v
		}
	}
	if !isFlagSet("poll-interval") {
		if v := os.Getenv("SIFT_POLL_INTERVAL"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return Config{}, fmt.Errorf("invalid SIFT_POLL_INTERVAL %q: %w", v, err)
			}
			pollInterval = n
		}
	}
	if !isFlagSet("heartbeat-interval") {
		if v := os.Getenv("SIFT_HEARTBEAT_INTERVAL"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				return Config{}, fmt.Errorf("invalid SIFT_HEARTBEAT_INTERVAL %q: %w", v, err)
			}
			heartbeatInterval = n
		}
	}
	if !isFlagSet("log-level") {
		if v := os.Getenv("SIFT_LOG_LEVEL"); v != "" {
			logLevel = v
		}
	}

	cfg := Config{
		Token:             token,
		DSN:               dsn,
		APIURL:            strings.TrimRight(apiURL, "/"),
		PollInterval:      time.Duration(pollInterval) * time.Second,
		HeartbeatInterval: time.Duration(heartbeatInterval) * time.Second,
		LogLevel:          logLevel,
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.Token == "" {
		return fmt.Errorf("token is required (use --token or SIFT_TOKEN)")
	}
	if c.DSN == "" {
		return fmt.Errorf("dsn is required (use --dsn or SIFT_DSN)")
	}
	if !strings.HasPrefix(c.DSN, "mysql://") {
		return fmt.Errorf("dsn must start with mysql:// (PostgreSQL support coming in v2)")
	}
	if c.PollInterval < 1*time.Second {
		return fmt.Errorf("poll-interval must be at least 1 second")
	}
	if c.HeartbeatInterval < 1*time.Second {
		return fmt.Errorf("heartbeat-interval must be at least 1 second")
	}
	return nil
}

func isFlagSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
