package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/oob-collaborator/backend/internal/security"
)

type Config struct {
	Domain             string
	PublicIP           string
	NSHost             string
	DatabaseURL        string
	AdminPassword      string
	JWTSecret          string
	PollToken          string
	ACMEmail           string
	ACMEStaging        bool
	HTTPPort           int
	HTTPSPort          int
	DNSPort            int
	SMTPPort           int
	SMTPSPort          int
	LogUnknownTokens   bool
	WebDistPath        string
	PayloadTokenLength int
	IPReconEnabled     bool
	DevMode            bool
}

func Load() (*Config, error) {
	domain := getEnv("DOMAIN", "localhost")
	devMode := domain == "localhost" || domain == "127.0.0.1" || os.Getenv("SKIP_TLS") == "true"

	cfg := &Config{
		Domain:             domain,
		PublicIP:           getEnv("PUBLIC_IP", "127.0.0.1"),
		NSHost:             getEnv("NS_HOST", "ns1"),
		DatabaseURL:        getEnv("DATABASE_URL", defaultDatabaseURL(devMode)),
		AdminPassword:      getEnv("ADMIN_PASSWORD", ""),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		PollToken:          getEnv("POLL_TOKEN", ""),
		ACMEmail:           getEnv("ACME_EMAIL", "admin@localhost"),
		ACMEStaging:        getEnvBool("ACME_STAGING", false),
		HTTPPort:           getEnvInt("HTTP_PORT", 80),
		HTTPSPort:          getEnvInt("HTTPS_PORT", 443),
		DNSPort:            getEnvInt("DNS_PORT", 53),
		SMTPPort:           getEnvInt("SMTP_PORT", 25),
		SMTPSPort:          getEnvInt("SMTPS_PORT", 587),
		LogUnknownTokens:   getEnvBool("LOG_UNKNOWN_TOKENS", false),
		WebDistPath:        getEnv("WEB_DIST_PATH", "web/dist"),
		PayloadTokenLength: getEnvInt("PAYLOAD_TOKEN_LENGTH", 6),
		IPReconEnabled:     getEnvBool("IP_RECON_ENABLED", true),
		DevMode:            devMode,
	}

	if devMode {
		if cfg.AdminPassword == "" {
			cfg.AdminPassword = "changeme"
		}
		if cfg.JWTSecret == "" {
			cfg.JWTSecret = "dev-secret-change-me-in-production"
		}
		if cfg.PollToken == "" {
			cfg.PollToken = "changeme-poll-token"
		}
	}

	if cfg.PayloadTokenLength < 4 {
		cfg.PayloadTokenLength = 4
	}
	if cfg.PayloadTokenLength > 32 {
		cfg.PayloadTokenLength = 32
	}

	if cfg.Domain == "" {
		return nil, fmt.Errorf("DOMAIN is required")
	}

	if err := security.ValidateSecret("ADMIN_PASSWORD", cfg.AdminPassword, 16, devMode); err != nil {
		return nil, err
	}
	if err := security.ValidateSecret("JWT_SECRET", cfg.JWTSecret, 32, devMode); err != nil {
		return nil, err
	}
	if err := security.ValidateSecret("POLL_TOKEN", cfg.PollToken, 32, devMode); err != nil {
		return nil, err
	}

	return cfg, nil
}

func defaultDatabaseURL(devMode bool) string {
	if devMode {
		user := getEnv("POSTGRES_USER", "collaborator")
		pass := getEnv("POSTGRES_PASSWORD", "collaborator")
		db := getEnv("POSTGRES_DB", "collaborator")
		host := getEnv("POSTGRES_HOST", "localhost")
		port := getEnv("POSTGRES_PORT", "5432")
		sslmode := getEnv("POSTGRES_SSLMODE", "disable")
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", user, pass, host, port, db, sslmode)
	}
	return ""
}

func (c *Config) NSFQDN() string {
	return fmt.Sprintf("%s.%s", c.NSHost, c.Domain)
}

func (c *Config) WildcardDomain() string {
	return "*." + c.Domain
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvBool(key string, fallback bool) bool {
	v := strings.ToLower(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes"
}
