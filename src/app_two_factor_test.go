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

	"github.com/pquerna/otp/totp"
)

func newTwoFactorTestApp(t *testing.T) (*App, *Store, string, *User) {
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
		ID:        "usr_two_factor",
		Email:     "owner@example.com",
		Name:      "Owner",
		CreatedAt: nowUTC(),
		UpdatedAt: nowUTC(),
	}
	if err := store.db.Exec(`INSERT INTO users (id, email, name, picture, is_admin, is_suspended, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);`, user.ID, user.Email, user.Name, "", 1, 0, user.CreatedAt, user.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	crypto := NewCrypto([]byte("12345678901234567890123456789012"))
	sessionToken := "pst_two_factor"
	if err := store.CreateSession(user.ID, SHA256Hex(sessionToken), nowUTC().Add(time.Hour)); err != nil {
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
	}, store, crypto)
	return app, store, sessionToken, user
}

func TestTwoFactorEnrollmentFlow(t *testing.T) {
	app, store, sessionToken, user := newTwoFactorTestApp(t)

	startReq := httptest.NewRequest(http.MethodPost, "/settings/2fa/start", strings.NewReader(url.Values{
		"csrf_token": {app.csrfTokenFromSession(sessionToken)},
	}.Encode()))
	startReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	startReq.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	startResp := httptest.NewRecorder()
	app.handleTwoFactorSetupStart(startResp, startReq)
	if startResp.Code != http.StatusFound {
		t.Fatalf("expected redirect when starting 2FA, got %d: %s", startResp.Code, startResp.Body.String())
	}

	settings, err := store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if settings == nil || settings.Enabled || strings.TrimSpace(settings.PendingSecretEnc) == "" {
		t.Fatalf("expected pending 2FA enrollment, got %+v", settings)
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/settings/2fa/cancel", strings.NewReader(url.Values{
		"csrf_token": {app.csrfTokenFromSession(sessionToken)},
	}.Encode()))
	cancelReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	cancelReq.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	cancelResp := httptest.NewRecorder()
	app.handleTwoFactorSetupCancel(cancelResp, cancelReq)
	if cancelResp.Code != http.StatusFound {
		t.Fatalf("expected redirect when canceling 2FA enrollment, got %d: %s", cancelResp.Code, cancelResp.Body.String())
	}
	settings, err = store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if settings == nil || settings.Enabled || strings.TrimSpace(settings.PendingSecretEnc) != "" {
		t.Fatalf("expected canceled 2FA enrollment, got %+v", settings)
	}

	startResp = httptest.NewRecorder()
	app.handleTwoFactorSetupStart(startResp, startReq)
	if startResp.Code != http.StatusFound {
		t.Fatalf("expected redirect when restarting 2FA, got %d: %s", startResp.Code, startResp.Body.String())
	}
	settings, err = store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if settings == nil || settings.Enabled || strings.TrimSpace(settings.PendingSecretEnc) == "" {
		t.Fatalf("expected pending 2FA enrollment after restart, got %+v", settings)
	}
	secret, err := app.crypto.Decrypt(settings.PendingSecretEnc)
	if err != nil {
		t.Fatal(err)
	}

	qrReq := httptest.NewRequest(http.MethodGet, "/settings/2fa/qr.png", nil)
	qrReq.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	qrResp := httptest.NewRecorder()
	app.handleTwoFactorQRCode(qrResp, qrReq)
	if qrResp.Code != http.StatusOK {
		t.Fatalf("expected QR code image, got %d: %s", qrResp.Code, qrResp.Body.String())
	}
	if got := qrResp.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("unexpected QR code content type %q", got)
	}
	if qrResp.Body.Len() == 0 {
		t.Fatal("expected QR code image body")
	}

	code, err := totp.GenerateCode(secret, nowUTC())
	if err != nil {
		t.Fatal(err)
	}
	confirmReq := httptest.NewRequest(http.MethodPost, "/settings/2fa/confirm", strings.NewReader(url.Values{
		"csrf_token": {app.csrfTokenFromSession(sessionToken)},
		"code":       {code},
	}.Encode()))
	confirmReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	confirmReq.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	confirmResp := httptest.NewRecorder()
	app.handleTwoFactorSetupConfirm(confirmResp, confirmReq)
	if confirmResp.Code != http.StatusFound {
		t.Fatalf("expected redirect when confirming 2FA, got %d: %s", confirmResp.Code, confirmResp.Body.String())
	}

	settings, err = store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if settings == nil || !settings.Enabled || strings.TrimSpace(settings.SecretEnc) == "" || strings.TrimSpace(settings.PendingSecretEnc) != "" {
		t.Fatalf("expected enabled 2FA settings, got %+v", settings)
	}

	resetReq := httptest.NewRequest(http.MethodPost, "/settings/2fa/reset", strings.NewReader(url.Values{
		"csrf_token": {app.csrfTokenFromSession(sessionToken)},
	}.Encode()))
	resetReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resetReq.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resetResp := httptest.NewRecorder()
	app.handleTwoFactorReset(resetResp, resetReq)
	if resetResp.Code != http.StatusFound {
		t.Fatalf("expected redirect when resetting 2FA, got %d: %s", resetResp.Code, resetResp.Body.String())
	}

	settings, err = store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if settings == nil || settings.Enabled || strings.TrimSpace(settings.SecretEnc) != "" || strings.TrimSpace(settings.PendingSecretEnc) != "" {
		t.Fatalf("expected cleared 2FA settings after reset, got %+v", settings)
	}
}

func TestGoogleLoginRequiresTwoFactorWhenEnabled(t *testing.T) {
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
		ID:        "usr_login_two_factor",
		Email:     "owner@example.com",
		Name:      "Owner",
		CreatedAt: nowUTC(),
		UpdatedAt: nowUTC(),
	}
	if err := store.db.Exec(`INSERT INTO users (id, email, name, picture, is_admin, is_suspended, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);`, user.ID, user.Email, user.Name, "", 1, 0, user.CreatedAt, user.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	crypto := NewCrypto([]byte("12345678901234567890123456789012"))
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
	}, store, crypto)

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      app.twoFactorAppName(),
		AccountName: user.Email,
	})
	if err != nil {
		t.Fatal(err)
	}
	secretEnc, err := app.crypto.Encrypt(key.Secret())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableUserTwoFactor(user.ID, secretEnc); err != nil {
		t.Fatal(err)
	}

	state := "st_login_two_factor"
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
	app.handleGoogleCallback(callbackResp, callbackReq)
	if callbackResp.Code != http.StatusFound {
		t.Fatalf("expected redirect to 2FA challenge, got %d: %s", callbackResp.Code, callbackResp.Body.String())
	}
	if got := callbackResp.Header().Get("Location"); got != "/auth/2fa" {
		t.Fatalf("expected redirect to /auth/2fa, got %q", got)
	}

	var sessionCookie *http.Cookie
	for _, cookie := range callbackResp.Result().Cookies() {
		if cookie.Name == "session" {
			sessionCookie = cookie
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected login callback to set session cookie")
	}
	session, err := store.FindSessionByTokenHash(SHA256Hex(sessionCookie.Value))
	if err != nil {
		t.Fatal(err)
	}
	if session == nil || session.SecondFactorVerified {
		t.Fatalf("expected pending 2FA session, got %+v", session)
	}
	sessionTokenHash := session.TokenHash

	code, err := totp.GenerateCode(key.Secret(), nowUTC())
	if err != nil {
		t.Fatal(err)
	}
	verifyReq := httptest.NewRequest(http.MethodPost, "/auth/2fa", strings.NewReader(url.Values{
		"csrf_token": {app.csrfTokenFromSession(sessionCookie.Value)},
		"code":       {code},
	}.Encode()))
	verifyReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	verifyReq.AddCookie(sessionCookie)
	verifyResp := httptest.NewRecorder()
	app.handleTwoFactorChallengeVerify(verifyResp, verifyReq)
	if verifyResp.Code != http.StatusFound {
		t.Fatalf("expected redirect after successful 2FA verification, got %d: %s", verifyResp.Code, verifyResp.Body.String())
	}
	if got := verifyResp.Header().Get("Location"); got != "/" {
		t.Fatalf("expected redirect to / after successful 2FA verification, got %q", got)
	}
	session, err = store.FindSessionByTokenHash(sessionTokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if session == nil || !session.SecondFactorVerified {
		t.Fatalf("expected verified session after 2FA challenge, got %+v", session)
	}
}
