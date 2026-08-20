package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
)

type Config struct {
	Version     string
	ServiceName string
	HttpPort    int
}

// The environment is read exactly once, however many times LoadConfig is
// called. once caches the result -- and the failure -- so a second caller can
// never re-parse .env or see a different answer than the first.
var (
	once   sync.Once
	cfg    *Config
	loaded error
)

// LoadConfig returns the process configuration, reading the environment on the
// first call and returning the same pointer afterwards.
//
// This is the only way to obtain a Config, and it is meant to be called from
// exactly one place -- main -- which then passes the pointer down as a
// dependency. That is why there is no package-level getter: a package that
// needs configuration receives it, instead of reaching back up for it.
//
// It reports an error rather than calling log.Fatal so the decision to end the
// process stays with main, where it belongs.
func LoadConfig() (*Config, error) {
	once.Do(func() {
		cfg, loaded = read()
	})

	return cfg, loaded
}

func read() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		return nil, fmt.Errorf("loading .env: %w", err)
	}

	version, err := required("VERSION")
	if err != nil {
		return nil, err
	}

	serviceName, err := required("SERVICE_NAME")
	if err != nil {
		return nil, err
	}

	rawPort, err := required("HTTP_PORT")
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return nil, fmt.Errorf("HTTP_PORT must be an integer, got %q", rawPort)
	}

	return &Config{
		Version:     version,
		ServiceName: serviceName,
		HttpPort:    port,
	}, nil
}

// required reads an environment variable that has no sensible default.
func required(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is not defined in the environment", key)
	}

	return value, nil
}
