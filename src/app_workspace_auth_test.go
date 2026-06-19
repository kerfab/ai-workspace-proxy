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

func newWorkspaceAuthTestApp(t *testing.T) (*App, *Store, string) {
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
		ID:        "usr_workspace",
		Email:     "owner@example.com",
		Name:      "Owner",
		CreatedAt: nowUTC(),
		UpdatedAt: nowUTC(),
	}
	if err := store.db.Exec(`INSERT INTO users (id, email, name, picture, is_admin, is_suspended, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);`, user.ID, user.Email, user.Name, "", 0, 0, user.CreatedAt, user.UpdatedAt); err != nil {
		t.Fatal(err)
	}

	crypto := NewCrypto([]byte("12345678901234567890123456789012"))
	agentToken := "atk_workspace"
	agentEnc, err := crypto.Encrypt(agentToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAgent(&AgentAccess{
		ID:              "agt_workspace",
		UserID:          user.ID,
		FriendlyName:    "Workspace agent",
		DefaultLocation: "test suite",
		TokenEnc:        agentEnc,
		TokenHint:       "atk_wor...ace",
		Enabled:         true,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveGmailConnection(&GmailConnection{
		UserID:          user.ID,
		MailboxEmail:    "workspace@example.com",
		FriendlyName:    "workspace",
		Scopes:          strings.Join((&App{}).workspaceScopes(), " "),
		RefreshTokenEnc: "refresh_token_enc",
		TokenExpiry:     nowUTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAgentWorkspaceGrants(user.ID, "agt_workspace", []AgentWorkspaceGrant{
		{AgentID: "agt_workspace", UserID: user.ID, MailboxEmail: "workspace@example.com", PolicyID: systemPolicyID},
	}); err != nil {
		t.Fatal(err)
	}

	sessionToken := "pst_workspace"
	if err := store.CreateSession(user.ID, SHA256Hex(sessionToken), nowUTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	app := NewApp(&Config{
		BaseURL:             "http://localhost:8080",
		SessionCookieName:   "session",
		MaxRequestBodyBytes: 1024 * 1024,
	}, store, crypto)
	return app, store, sessionToken
}

func workspacePostRequest(t *testing.T, app *App, sessionToken, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	return resp
}

func TestWorkspaceDisconnectClearsOAuthButKeepsSettings(t *testing.T) {
	app, store, sessionToken := newWorkspaceAuthTestApp(t)
	if err := store.CreateDriveFolderRef(&AllowedDriveFolder{
		ID:            "dfr_workspace",
		UserID:        "usr_workspace",
		MailboxEmail:  "workspace@example.com",
		ReferenceName: "Reports",
		ReferenceKey:  "reports",
		FolderURL:     "https://drive.google.com/drive/folders/reports",
		FolderID:      "reports",
		FolderName:    "Reports",
	}); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"csrf_token": {app.csrfTokenFromSession(sessionToken)},
		"workspace":  {"workspace@example.com"},
	}
	resp := workspacePostRequest(t, app, sessionToken, "/auth/workspace/disconnect", form)
	if resp.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d: %s", resp.Code, resp.Body.String())
	}
	conn, err := store.GetGmailConnection("usr_workspace", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if conn == nil {
		t.Fatal("workspace should still exist after OAuth disconnect")
	}
	if conn.RefreshTokenEnc != "" || conn.AccessTokenEnc != "" || !conn.TokenExpiry.IsZero() {
		t.Fatalf("workspace OAuth tokens should be cleared after disconnect, got %+v", conn)
	}
	folders, err := store.ListDriveFolderRefs("usr_workspace", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].ReferenceName != "Reports" {
		t.Fatalf("Drive folder settings should be preserved after disconnect, got %+v", folders)
	}
}

func TestWorkspaceDeleteRemovesWorkspaceSettings(t *testing.T) {
	app, store, sessionToken := newWorkspaceAuthTestApp(t)
	if err := store.CreateDriveFolderRef(&AllowedDriveFolder{
		ID:            "dfr_workspace",
		UserID:        "usr_workspace",
		MailboxEmail:  "workspace@example.com",
		ReferenceName: "Reports",
		ReferenceKey:  "reports",
		FolderURL:     "https://drive.google.com/drive/folders/reports",
		FolderID:      "reports",
		FolderName:    "Reports",
	}); err != nil {
		t.Fatal(err)
	}

	form := url.Values{
		"csrf_token": {app.csrfTokenFromSession(sessionToken)},
		"workspace":  {"workspace@example.com"},
	}
	resp := workspacePostRequest(t, app, sessionToken, "/workspace/delete", form)
	if resp.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d: %s", resp.Code, resp.Body.String())
	}
	conn, err := store.GetGmailConnection("usr_workspace", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if conn != nil {
		t.Fatalf("workspace should be deleted, got %+v", conn)
	}
	folders, err := store.ListDriveFolderRefs("usr_workspace", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 0 {
		t.Fatalf("Drive folder settings should be deleted with workspace settings, got %+v", folders)
	}
}

func TestProxyRelayReturnsWorkspaceReauthURLWhenWorkspaceIsDisconnected(t *testing.T) {
	app, store, _ := newWorkspaceAuthTestApp(t)
	if err := store.DisconnectGmailConnectionOAuth("usr_workspace", "workspace@example.com"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/gmail.googleapis.com/gmail/v1/users/me/messages?workspace=workspace@example.com", nil)
	req.Header.Set("Authorization", "Bearer atk_workspace")
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["error"] != "workspace_reauth_required" || payload["reauth_required"] != true {
		t.Fatalf("unexpected payload: %v", payload)
	}
	message := payload["message"].(string)
	reauthURL := payload["reauth_url"].(string)
	if !strings.Contains(message, "This URL must be opened by the human operator in a browser, not by the AI agent itself:") {
		t.Fatalf("agent error message should contain reauth instructions, got %q", message)
	}
	if !strings.HasPrefix(reauthURL, "http://localhost:8080/auth/workspace/reauth?token=") {
		t.Fatalf("unexpected reauth URL: %s", reauthURL)
	}
}

func TestWorkspaceCallbackRejectsWrongGoogleAccountWithoutOverwritingTokens(t *testing.T) {
	app, store, _ := newWorkspaceAuthTestApp(t)

	oldAccessToken := "old_access_token"
	oldRefreshToken := "old_refresh_token"
	oldAccessEnc, err := app.crypto.Encrypt(oldAccessToken)
	if err != nil {
		t.Fatal(err)
	}
	oldRefreshEnc, err := app.crypto.Encrypt(oldRefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	conn := &GmailConnection{
		UserID:          "usr_workspace",
		MailboxEmail:    "workspace@example.com",
		FriendlyName:    "workspace",
		Scopes:          strings.Join(app.workspaceScopes(), " "),
		AccessTokenEnc:  oldAccessEnc,
		RefreshTokenEnc: oldRefreshEnc,
		TokenExpiry:     nowUTC().Add(2 * time.Hour),
	}
	if err := store.SaveGmailConnection(conn); err != nil {
		t.Fatal(err)
	}

	state := "st_workspace_callback"
	if err := store.SaveWorkspaceOAuthState(SHA256Hex(state), "usr_workspace", "workspace@example.com", nowUTC().Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}

	app.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.String() {
			case googleTokenURL:
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"new_access_token","expires_in":3600,"refresh_token":"new_refresh_token","scope":"` + strings.Join(app.workspaceScopes(), " ") + `","token_type":"Bearer"}`)),
				}, nil
			case gmailProfileURL:
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"emailAddress":"wrong@example.com"}`)),
				}, nil
			default:
				t.Fatalf("unexpected outbound request to %s", req.URL.String())
				return nil, nil
			}
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/workspace/callback?state="+url.QueryEscape(state)+"&code=test-code", nil)
	resp := httptest.NewRecorder()
	app.handleWorkspaceCallback(resp, req)

	if resp.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d: %s", resp.Code, resp.Body.String())
	}
	location := resp.Header().Get("Location")
	expectedPrefix := "/?workspace=workspace%40example.com&workspace_error="
	if !strings.HasPrefix(location, expectedPrefix) {
		t.Fatalf("expected redirect to include workspace error, got %q", location)
	}
	decodedLocation, err := url.QueryUnescape(location)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(decodedLocation, "Google returned wrong@example.com, but this saved Workspace expects workspace@example.com.") {
		t.Fatalf("expected mismatch explanation in redirect, got %q", decodedLocation)
	}
	if !strings.Contains(decodedLocation, "The existing Google authorization for this Workspace was kept unchanged.") {
		t.Fatalf("expected token preservation notice in redirect, got %q", decodedLocation)
	}

	saved, err := store.GetGmailConnection("usr_workspace", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil {
		t.Fatal("expected saved workspace connection to remain")
	}
	gotAccessToken, err := app.crypto.Decrypt(saved.AccessTokenEnc)
	if err != nil {
		t.Fatal(err)
	}
	gotRefreshToken, err := app.crypto.Decrypt(saved.RefreshTokenEnc)
	if err != nil {
		t.Fatal(err)
	}
	if gotAccessToken != oldAccessToken || gotRefreshToken != oldRefreshToken {
		t.Fatalf("expected existing tokens to remain unchanged, got access=%q refresh=%q", gotAccessToken, gotRefreshToken)
	}

	wrongConn, err := store.GetGmailConnection("usr_workspace", "wrong@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if wrongConn != nil {
		t.Fatalf("wrong Google account should not create a new workspace connection, got %+v", wrongConn)
	}
}

func TestWorkspaceCallbackAcceptsMatchingStateCookieWhenStateRowIsMissing(t *testing.T) {
	app, store, _ := newWorkspaceAuthTestApp(t)

	app.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.String() {
			case googleTokenURL:
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"access_token":"new_access_token","expires_in":3600,"refresh_token":"new_refresh_token","scope":"` + strings.Join(app.workspaceScopes(), " ") + `","token_type":"Bearer"}`)),
				}, nil
			case gmailProfileURL:
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(`{"emailAddress":"workspace@example.com"}`)),
				}, nil
			default:
				t.Fatalf("unexpected outbound request to %s", req.URL.String())
				return nil, nil
			}
		}),
	}

	state := "st_workspace_cookie_fallback"
	req := httptest.NewRequest(http.MethodGet, "/auth/workspace/callback?state="+url.QueryEscape(state)+"&code=test-code", nil)
	req.AddCookie(&http.Cookie{
		Name:  workspaceOAuthStateCookieName,
		Value: encodeWorkspaceOAuthStateCookieValue(SHA256Hex(state), "usr_workspace", "workspace@example.com"),
	})
	resp := httptest.NewRecorder()
	app.handleWorkspaceCallback(resp, req)

	if resp.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d: %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Location"); got != "/?workspace=workspace%40example.com" {
		t.Fatalf("expected redirect to workspace page, got %q", got)
	}
	saved, err := store.GetGmailConnection("usr_workspace", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil {
		t.Fatal("expected workspace connection to remain")
	}
	gotRefreshToken, err := app.crypto.Decrypt(saved.RefreshTokenEnc)
	if err != nil {
		t.Fatal(err)
	}
	if gotRefreshToken != "new_refresh_token" {
		t.Fatalf("expected updated refresh token, got %q", gotRefreshToken)
	}
}
