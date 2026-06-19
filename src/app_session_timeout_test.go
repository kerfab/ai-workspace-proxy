// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newSessionTimeoutTestApp(t *testing.T) (*App, *Store, string, *User) {
	t.Helper()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	user := &User{
		ID:        "usr_session_timeout",
		Email:     "owner@example.com",
		Name:      "Owner",
		CreatedAt: nowUTC(),
		UpdatedAt: nowUTC(),
	}
	if err := store.db.Exec(`INSERT INTO users (id, email, name, picture, is_admin, is_suspended, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);`, user.ID, user.Email, user.Name, "", 1, 0, user.CreatedAt, user.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	sessionToken := "pst_session_timeout"
	if err := store.CreateSession(user.ID, SHA256Hex(sessionToken), nowUTC().Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	app := NewApp(&Config{
		AppName:               "AI Workspace Proxy",
		BaseURL:               "http://localhost:8080",
		SessionCookieName:     "session",
		SessionTTL:            24 * time.Hour,
		MaxRequestBodyBytes:   1024 * 1024,
		WorkspaceClientID:     "client-id",
		WorkspaceClientSecret: "client-secret",
		AllowedEmailDomains:   map[string]bool{"example.com": true},
		AdminEmails:           map[string]bool{"owner@example.com": true},
		EncryptionKey:         []byte("12345678901234567890123456789012"),
	}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	return app, store, sessionToken, user
}

func TestGetUserSettingsDefaultsSessionTimeoutTo24Hours(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	settings, err := store.GetUserSettings("usr_default")
	if err != nil {
		t.Fatal(err)
	}
	if settings.SessionTimeoutHours != defaultUserSessionTimeoutHours {
		t.Fatalf("session timeout = %d, want %d", settings.SessionTimeoutHours, defaultUserSessionTimeoutHours)
	}
}

func TestHandleSessionTimeoutUpdateAdjustsCurrentSessionAndReturnsJSON(t *testing.T) {
	app, store, sessionToken, user := newSessionTimeoutTestApp(t)

	session, err := store.FindSessionByTokenHash(SHA256Hex(sessionToken))
	if err != nil {
		t.Fatal(err)
	}
	sessionStart := nowUTC().Add(-2 * time.Hour).Truncate(time.Second)
	if err := store.db.Exec(`UPDATE sessions SET created_at = ?, expires_at = ? WHERE token_hash = ?;`, sessionStart, sessionStart.Add(24*time.Hour), session.TokenHash); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"csrf_token":            {app.csrfTokenFromSession(sessionToken)},
		"session_timeout_hours": {"6"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings/session-timeout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "fetch")
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	app.handleSessionTimeoutUpdate(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	wantExpiry := sessionStart.Add(6 * time.Hour)
	if got := int(payload["session_timeout_hours"].(float64)); got != 6 {
		t.Fatalf("session timeout in payload = %d, want 6", got)
	}
	if got := int64(payload["session_expires_at_unix_ms"].(float64)); got != wantExpiry.UnixMilli() {
		t.Fatalf("session expiry unix ms in payload = %d, want %d", got, wantExpiry.UnixMilli())
	}
	updatedSettings, err := store.GetUserSettings(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedSettings.SessionTimeoutHours != 6 {
		t.Fatalf("stored session timeout = %d, want 6", updatedSettings.SessionTimeoutHours)
	}
	updatedSession, err := store.FindSessionByTokenHash(SHA256Hex(sessionToken))
	if err != nil {
		t.Fatal(err)
	}
	if !updatedSession.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("session expiry = %s, want %s", updatedSession.ExpiresAt, wantExpiry)
	}
	result := resp.Result()
	defer result.Body.Close()
	foundCookie := false
	for _, cookie := range result.Cookies() {
		if cookie.Name == "session" {
			foundCookie = true
			if !cookie.Expires.Equal(wantExpiry) {
				t.Fatalf("cookie expiry = %s, want %s", cookie.Expires, wantExpiry)
			}
		}
	}
	if !foundCookie {
		t.Fatal("expected refreshed session cookie")
	}
	audits, err := store.ListAuditLogs("user_audit_logs", user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) != 1 || audits[0].Action != "user_settings_updated" || audits[0].TargetID != "session_timeout_hours" {
		t.Fatalf("unexpected audit logs: %+v", audits)
	}
	if !strings.Contains(audits[0].DetailsJSON, `"new_value":6`) {
		t.Fatalf("unexpected audit details: %s", audits[0].DetailsJSON)
	}
}

func TestGoogleLoginUsesStoredUserSessionTimeout(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	user := &User{
		ID:        "usr_login_timeout",
		Email:     "owner@example.com",
		Name:      "Owner",
		CreatedAt: nowUTC(),
		UpdatedAt: nowUTC(),
	}
	if err := store.db.Exec(`INSERT INTO users (id, email, name, picture, is_admin, is_suspended, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);`, user.ID, user.Email, user.Name, "", 1, 0, user.CreatedAt, user.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveUserSessionTimeoutHours(user.ID, 2); err != nil {
		t.Fatal(err)
	}
	app := NewApp(&Config{
		AppName:               "AI Workspace Proxy",
		BaseURL:               "http://localhost:8080",
		SessionCookieName:     "session",
		SessionTTL:            24 * time.Hour,
		MaxRequestBodyBytes:   1024 * 1024,
		WorkspaceClientID:     "client-id",
		WorkspaceClientSecret: "client-secret",
		AllowedEmailDomains:   map[string]bool{"example.com": true},
		AdminEmails:           map[string]bool{"owner@example.com": true},
		EncryptionKey:         []byte("12345678901234567890123456789012"),
	}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	state := "st_login_timeout"
	if err := store.SaveOAuthState(SHA256Hex(state), loginStatePurpose, "", nowUTC().Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	app.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.String() {
			case googleTokenURL:
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"login_access","expires_in":3600,"token_type":"Bearer"}`)),
				}, nil
			case googleUserInfoURL:
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"email":"owner@example.com","email_verified":true,"name":"Owner","picture":""}`)),
				}, nil
			default:
				t.Fatalf("unexpected outbound request to %s", req.URL.String())
				return nil, nil
			}
		}),
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=test-code", nil)
	callbackResp := httptest.NewRecorder()
	before := nowUTC()
	app.handleGoogleCallback(callbackResp, callbackReq)
	after := nowUTC()

	if callbackResp.Code != http.StatusFound {
		t.Fatalf("expected redirect after login, got %d: %s", callbackResp.Code, callbackResp.Body.String())
	}
	var sessionCookie *http.Cookie
	for _, cookie := range callbackResp.Result().Cookies() {
		if cookie.Name == "session" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie")
	}
	session, err := store.FindSessionByTokenHash(SHA256Hex(sessionCookie.Value))
	if err != nil {
		t.Fatal(err)
	}
	if session == nil {
		t.Fatal("expected stored session")
	}
	minExpected := before.Add(2 * time.Hour).Add(-2 * time.Second)
	maxExpected := after.Add(2 * time.Hour).Add(2 * time.Second)
	if session.ExpiresAt.Before(minExpected) || session.ExpiresAt.After(maxExpected) {
		t.Fatalf("session expiry = %s, expected between %s and %s", session.ExpiresAt, minExpected, maxExpected)
	}
	if sessionCookie.Expires.Before(minExpected) || sessionCookie.Expires.After(maxExpected) {
		t.Fatalf("cookie expiry = %s, expected between %s and %s", sessionCookie.Expires, minExpected, maxExpected)
	}
}
