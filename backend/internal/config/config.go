package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Domain           string
	PublicIP         string
	NSHost           string
	DatabaseURL      string
	AdminPassword    string
	JWTSecret        string
	PollToken        string
	ACMEmail         string
	ACMEStaging      bool
	HTTPPort         int
	HTTPSPort        int
	DNSPort          int
	SMTPPort         int
	SMTPSPort        int
	LogUnknownTokens   bool
	WebDistPath        string
	PayloadTokenLength int
	IPReconEnabled     bool
}

func Load() (*Config, error) {
	cfg := &Config{
		Domain:           getEnv("DOMAIN", "localhost"),
		PublicIP:         getEnv("PUBLIC_IP", "127.0.0.1"),
		NSHost:           getEnv("NS_HOST", "ns1"),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://collaborator:collaborator@localhost:5432/collaborator?sslmode=disable"),
		AdminPassword:    getEnv("ADMIN_PASSWORD", "changeme"),
		JWTSecret:        getEnv("JWT_SECRET", "dev-secret-change-me"),
		PollToken:        getEnv("POLL_TOKEN", "changeme-poll-token"),
		ACMEmail:         getEnv("ACME_EMAIL", "admin@localhost"),
		ACMEStaging:      getEnvBool("ACME_STAGING", false),
		HTTPPort:         getEnvInt("HTTP_PORT", 80),
		HTTPSPort:        getEnvInt("HTTPS_PORT", 443),
		DNSPort:          getEnvInt("DNS_PORT", 53),
		SMTPPort:         getEnvInt("SMTP_PORT", 25),
		SMTPSPort:        getEnvInt("SMTPS_PORT", 587),
		LogUnknownTokens:   getEnvBool("LOG_UNKNOWN_TOKENS", true),
		WebDistPath:        getEnv("WEB_DIST_PATH", "web/dist"),
		PayloadTokenLength: getEnvInt("PAYLOAD_TOKEN_LENGTH", 6),
		IPReconEnabled:     getEnvBool("IP_RECON_ENABLED", true),
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
	if cfg.AdminPassword == "" {
		return nil, fmt.Errorf("ADMIN_PASSWORD is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
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
