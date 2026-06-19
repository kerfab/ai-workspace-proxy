// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppName                 = "AI Workspace Proxy"
	defaultBindAddr                = ":8080"
	defaultDBPath                  = "./db/ai_workspace_proxy.sqlite3"
	defaultSessionCookieName       = "ai_workspace_proxy_session"
	defaultSessionTTLHours         = 168
	defaultUserSessionTimeoutHours = 24
	maxUserSessionTimeoutHours     = 8760
	defaultHTTPTimeoutSec          = 30
	defaultMaxRequestBody          = 10 * 1024 * 1024

	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	gmailProfileURL   = "https://gmail.googleapis.com/gmail/v1/users/me/profile"
	gmailAPIBase      = "https://gmail.googleapis.com"
	peopleAPIBase     = "https://people.googleapis.com"
	calendarAPIBase   = "https://www.googleapis.com"
	driveAPIBase      = "https://www.googleapis.com"
	docsAPIBase       = "https://docs.googleapis.com"
	sheetsAPIBase     = "https://sheets.googleapis.com"
	slidesAPIBase     = "https://slides.googleapis.com"
)

type Config struct {
	AppName             string
	BindAddr            string
	BaseURL             string
	DBPath              string
	SessionCookieName   string
	CookieSecure        bool
	SessionTTL          time.Duration
	HTTPClientTimeout   time.Duration
	MaxRequestBodyBytes int64

	WorkspaceClientID     string
	WorkspaceClientSecret string

	EncryptionKey       []byte
	AllowedEmailDomains map[string]bool
	AdminEmails         map[string]bool
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		AppName:               getenvDefault("APP_NAME", defaultAppName),
		BindAddr:              getenvDefault("APP_BIND_ADDR", defaultBindAddr),
		BaseURL:               strings.TrimRight(os.Getenv("APP_BASE_URL"), "/"),
		DBPath:                getenvDefault("DB_PATH", defaultDBPath),
		SessionCookieName:     getenvDefault("SESSION_COOKIE_NAME", defaultSessionCookieName),
		CookieSecure:          strings.EqualFold(getenvDefault("COOKIE_SECURE", "false"), "true"),
		SessionTTL:            time.Duration(getenvIntDefault("SESSION_TTL_HOURS", defaultSessionTTLHours)) * time.Hour,
		HTTPClientTimeout:     time.Duration(getenvIntDefault("HTTP_CLIENT_TIMEOUT_SEC", defaultHTTPTimeoutSec)) * time.Second,
		MaxRequestBodyBytes:   int64(getenvIntDefault("MAX_REQUEST_BODY_BYTES", defaultMaxRequestBody)),
		WorkspaceClientID:     os.Getenv("GOOGLE_WORKSPACE_CLIENT_ID"),
		WorkspaceClientSecret: os.Getenv("GOOGLE_WORKSPACE_CLIENT_SECRET"),
		AllowedEmailDomains:   parseAllowedEmailDomains(os.Getenv("ALLOWED_EMAIL_DOMAINS")),
		AdminEmails:           map[string]bool{},
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("APP_BASE_URL is required")
	}
	if cfg.WorkspaceClientID == "" || cfg.WorkspaceClientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_WORKSPACE_CLIENT_ID and GOOGLE_WORKSPACE_CLIENT_SECRET are required")
	}
	if len(cfg.AllowedEmailDomains) == 0 {
		return nil, fmt.Errorf("ALLOWED_EMAIL_DOMAINS is required")
	}
	key, err := parseEncryptionKey(strings.TrimSpace(os.Getenv("PROXY_ENCRYPTION_KEY")))
	if err != nil {
		return nil, fmt.Errorf("invalid PROXY_ENCRYPTION_KEY: %w", err)
	}
	cfg.EncryptionKey = key

	adminCount := 0
	for _, part := range strings.Split(os.Getenv("ADMIN_EMAILS"), ",") {
		part = normalizeEmail(part)
		if part == "" {
			continue
		}
		if !cfg.IsEmailAllowed(part) {
			return nil, fmt.Errorf("admin email %q is outside ALLOWED_EMAIL_DOMAINS", part)
		}
		cfg.AdminEmails[part] = true
		adminCount++
	}
	if adminCount == 0 {
		return nil, fmt.Errorf("ADMIN_EMAILS must contain at least one email")
	}
	return cfg, nil
}

func (c *Config) LoginRedirectURI() string     { return c.BaseURL + "/auth/google/callback" }
func (c *Config) WorkspaceRedirectURI() string { return c.BaseURL + "/auth/workspace/callback" }
func (c *Config) GmailRedirectURI() string     { return c.WorkspaceRedirectURI() }

func (c *Config) IsEmailAllowed(email string) bool {
	email = normalizeEmail(email)
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	return c.AllowedEmailDomains[parts[1]]
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(domain, "@")
	return domain
}

func parseAllowedEmailDomains(raw string) map[string]bool {
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		domain := normalizeDomain(part)
		if domain != "" {
			out[domain] = true
		}
	}
	return out
}

func parseEncryptionKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, fmt.Errorf("value is empty")
	}
	if len(raw) == 64 {
		if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	return nil, fmt.Errorf("must be 32 raw bytes, 64 hex characters, or base64 for exactly 32 bytes")
}

func getenvDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getenvIntDefault(key string, defaultValue int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	return n
}
