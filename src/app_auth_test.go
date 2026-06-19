// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGoogleCallbackAcceptsMatchingStateCookieWhenStateRowIsMissing(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	app := NewApp(&Config{
		BaseURL:               "http://localhost:8080",
		SessionCookieName:     "session",
		SessionTTL:            time.Hour,
		MaxRequestBodyBytes:   1024 * 1024,
		WorkspaceClientID:     "client-id",
		WorkspaceClientSecret: "client-secret",
		AllowedEmailDomains:   map[string]bool{"example.com": true},
		AdminEmails:           map[string]bool{"owner@example.com": true},
		EncryptionKey:         []byte("12345678901234567890123456789012"),
	}, store, NewCrypto([]byte("12345678901234567890123456789012")))

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

	state := "st_cookie_login"
	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state="+url.QueryEscape(state)+"&code=test-code", nil)
	req.AddCookie(&http.Cookie{Name: loginOAuthStateCookieName, Value: SHA256Hex(state)})
	resp := httptest.NewRecorder()
	app.handleGoogleCallback(resp, req)

	if resp.Code != http.StatusFound {
		t.Fatalf("expected redirect after login, got %d: %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Location"); got != "/" {
		t.Fatalf("expected redirect to home, got %q", got)
	}
	var sessionCookie *http.Cookie
	for _, cookie := range resp.Result().Cookies() {
		if cookie.Name == "session" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie")
	}
}

func TestGoogleCallbackInvalidStateRendersHelpfulHTMLPage(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	app := NewApp(&Config{
		AppName:               "AI Workspace Proxy",
		BaseURL:               "http://localhost:8080",
		SessionCookieName:     "session",
		SessionTTL:            time.Hour,
		MaxRequestBodyBytes:   1024 * 1024,
		WorkspaceClientID:     "client-id",
		WorkspaceClientSecret: "client-secret",
		AllowedEmailDomains:   map[string]bool{"example.com": true},
		AdminEmails:           map[string]bool{"owner@example.com": true},
		EncryptionKey:         []byte("12345678901234567890123456789012"),
	}, store, NewCrypto([]byte("12345678901234567890123456789012")))

	req := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=stale&code=test-code", nil)
	resp := httptest.NewRecorder()
	app.handleGoogleCallback(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid state, got %d", resp.Code)
	}
	body := resp.Body.String()
	for _, want := range []string{
		"Google sign-in could not be completed",
		"The Google sign-in state was invalid or expired.",
		"The sign-in window was left open too long before completion, so the temporary sign-in state expired.",
		"The callback page was reloaded or opened more than once after Google redirected back to the proxy.",
		"Your browser lost the temporary sign-in state or blocked the cookie used to track this sign-in attempt.",
		`Please return to the home page and start the sign-in process again.`,
		`<a class="btn" href="/">Back to home</a>`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected OAuth error page to contain %q, got:\n%s", want, body)
		}
	}
	if got := resp.Header().Get("Content-Type"); !strings.Contains(got, "text/html") {
		t.Fatalf("expected HTML response content type, got %q", got)
	}
}
