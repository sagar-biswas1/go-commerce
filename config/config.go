package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Version     string
	ServiceName string
	Environment string
	HttpPort    int
	JWT         *JWTConfig
	PG          *PGConfig
	Bootstrap   *BootstrapConfig
}

// BootstrapConfig is the first administrator, for a database that has none.
//
// Without it a fresh deployment is unusable: registration only ever creates an
// ordinary user, and creating a privileged one requires being one already. Both
// fields are optional -- leave them unset once the account exists, and nothing
// is created.
type BootstrapConfig struct {
	AdminEmail    string
	AdminPassword string
}

// Wanted reports whether an admin should be created if none exists.
func (b *BootstrapConfig) Wanted() bool {
	return b != nil && b.AdminEmail != "" && b.AdminPassword != ""
}

// The environments this service knows about. They exist so a setting whose safe
// value differs between a laptop and production -- the Secure flag on a session
// cookie, say -- can be decided from one variable instead of remembered.
const (
	EnvDevelopment = "development"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)

// IsDevelopment reports whether this process is running on somebody's machine.
// Anything unrecognised counts as not development, so a typo in the variable
// fails towards the stricter behaviour.
func (c *Config) IsDevelopment() bool {
	return c != nil && c.Environment == EnvDevelopment
}

// JWTConfig holds everything the token layer needs. Issuer and audience are
// part of it because a token that does not say who minted it and who it is for
// cannot be rejected when it arrives from somewhere else -- a validator that
// only checks the signature accepts any token signed with the same secret,
// including one issued by a different service that happens to share it.
type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	Issuer        string
	Audience      string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	// Leeway forgives small clock differences between whoever signed a token
	// and whoever is checking its expiry.
	Leeway time.Duration
}

type PGConfig struct {
	Host           string
	Database       string
	User           string
	Password       string
	SslMode        string
	ChannelBinding string
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

	jwt, err := readJWT(serviceName)
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return nil, fmt.Errorf("HTTP_PORT must be an integer, got %q", rawPort)
	}

	pg, err := readPG()
	if err != nil {
		return nil, err
	}

	return &Config{
		Version:     version,
		ServiceName: serviceName,
		Environment: optional("ENVIRONMENT", EnvDevelopment),
		HttpPort:    port,
		JWT:         jwt,
		PG:          pg,
		Bootstrap: &BootstrapConfig{
			AdminEmail:    optional("BOOTSTRAP_ADMIN_EMAIL", ""),
			AdminPassword: optional("BOOTSTRAP_ADMIN_PASSWORD", ""),
		},
	}, nil
}

// minSecretLen is the shortest HMAC key worth calling a secret. HS256 keys
// shorter than the 256-bit digest weaken the signature, and in practice a short
// one means somebody typed a placeholder into .env and forgot.
const minSecretLen = 32

func readJWT(serviceName string) (*JWTConfig, error) {
	accessSecret, err := requiredSecret("JWT_ACCESS_SECRET")
	if err != nil {
		return nil, err
	}

	refreshSecret, err := requiredSecret("JWT_REFRESH_SECRET")
	if err != nil {
		return nil, err
	}

	// Signing both token kinds with one key means a refresh token would verify
	// as an access token, turning a long-lived credential into an unexpiring
	// session. Two distinct keys are what keep the two kinds apart.
	if accessSecret == refreshSecret {
		return nil, fmt.Errorf("JWT_ACCESS_SECRET and JWT_REFRESH_SECRET must differ")
	}

	accessTTL, err := optionalDuration("JWT_ACCESS_TTL", 15*time.Minute)
	if err != nil {
		return nil, err
	}

	refreshTTL, err := optionalDuration("JWT_REFRESH_TTL", 7*24*time.Hour)
	if err != nil {
		return nil, err
	}

	leeway, err := optionalDuration("JWT_LEEWAY", 30*time.Second)
	if err != nil {
		return nil, err
	}

	return &JWTConfig{
		AccessSecret:  accessSecret,
		RefreshSecret: refreshSecret,
		Issuer:        optional("JWT_ISSUER", serviceName),
		Audience:      optional("JWT_AUDIENCE", serviceName+"-api"),
		AccessTTL:     accessTTL,
		RefreshTTL:    refreshTTL,
		Leeway:        leeway,
	}, nil
}

func readPG() (*PGConfig, error) {
	host, err := required("PGHOST")
	if err != nil {
		return nil, err
	}
	database, err := required("PGDATABASE")
	if err != nil {
		return nil, err
	}
	user, err := required("PGUSER")
	if err != nil {
		return nil, err
	}
	password, err := required("PGPASSWORD")
	if err != nil {
		return nil, err
	}
	sslMode, err := required("PGSSLMODE")
	if err != nil {
		return nil, err
	}

	return &PGConfig{
		Host:     host,
		Database: database,
		User:     user,
		Password: password,
		SslMode:  sslMode,
	}, nil
}

// optional reads an environment variable that has a sensible default.
func optional(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// optionalDuration reads a Go duration such as "15m" or "168h".
func optionalDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 15m or 168h, got %q", key, raw)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %q", key, raw)
	}
	return value, nil
}

// requiredSecret reads a signing key and refuses one too short to be worth
// signing with.
func requiredSecret(key string) (string, error) {
	value, err := required(key)
	if err != nil {
		return "", err
	}

	value = strings.TrimSpace(value)
	if len(value) < minSecretLen {
		return "", fmt.Errorf("%s must be at least %d characters", key, minSecretLen)
	}
	return value, nil
}

// required reads an environment variable that has no sensible default.
func required(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%s is not defined in the environment", key)
	}

	return value, nil
}

// GetConfig returns the already-loaded configuration, loading it if main has
// not. It reports nothing on failure, so main should keep calling LoadConfig
// first and treat this as a convenience for code far from the composition root.
func GetConfig() *Config {
	config, err := LoadConfig()
	if err != nil {
		return nil
	}
	return config
}
