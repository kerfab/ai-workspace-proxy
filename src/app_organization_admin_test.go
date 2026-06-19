// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newOrganizationAdminTestApp(t *testing.T) (*App, *Store, *User, string) {
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
	user, err := store.CreateOrUpdateUser("owner@example.com", "Owner", "", false)
	if err != nil {
		t.Fatal(err)
	}
	sessionToken := "pst_org_admin"
	if err := store.CreateSession(user.ID, SHA256Hex(sessionToken), nowUTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	app := NewApp(&Config{
		AppName:             "AI Workspace Proxy",
		BaseURL:             "http://localhost:8080",
		SessionCookieName:   "session",
		MaxRequestBodyBytes: 1024 * 1024,
		EncryptionKey:       []byte("12345678901234567890123456789012"),
	}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	return app, store, user, sessionToken
}

func organizationAdminPostRequest(t *testing.T, app *App, sessionToken string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/org-admin/domain/validate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "fetch")
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	return resp
}

func organizationAdminGetRequest(t *testing.T, app *App, sessionToken, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	return resp
}

func TestEnsureOrganizationAdminVerificationCreatesAndPersistsChallenge(t *testing.T) {
	store := newOrganizationTestStore(t)
	user, err := store.CreateOrUpdateUser("owner@example.com", "Owner", "", false)
	if err != nil {
		t.Fatal(err)
	}
	record, err := store.EnsureOrganizationAdminVerification(user)
	if err != nil {
		t.Fatal(err)
	}
	if record == nil {
		t.Fatal("expected organization admin verification record")
	}
	if record.DomainName != "example.com" {
		t.Fatalf("domain = %q, want example.com", record.DomainName)
	}
	if record.VerificationHost != "_ai-workspace-proxy-verification.example.com" {
		t.Fatalf("verification host = %q", record.VerificationHost)
	}
	wantValue, err := organizationAdminVerificationTXTValueForEmail(user.Email, []byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatal(err)
	}
	if record.VerificationValue != wantValue {
		t.Fatalf("verification value = %q, want %q", record.VerificationValue, wantValue)
	}
	sameRecord, err := store.EnsureOrganizationAdminVerification(user)
	if err != nil {
		t.Fatal(err)
	}
	if sameRecord.VerificationValue != record.VerificationValue {
		t.Fatalf("verification challenge should be stable, got %q then %q", record.VerificationValue, sameRecord.VerificationValue)
	}
	if err := store.MarkOrganizationAdminVerificationVerified(user.ID, nowUTC()); err != nil {
		t.Fatal(err)
	}
	verified, err := store.GetOrganizationAdminVerification(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if verified == nil || verified.VerifiedAt.IsZero() {
		t.Fatalf("expected verified timestamp, got %+v", verified)
	}
}

func TestEnsureOrganizationAdminVerificationUsesDistinctStableChallengesPerUser(t *testing.T) {
	store := newOrganizationTestStore(t)
	firstUser, err := store.CreateOrUpdateUser("owner@example.com", "Owner", "", false)
	if err != nil {
		t.Fatal(err)
	}
	secondUser, err := store.CreateOrUpdateUser("teammate@example.com", "Teammate", "", false)
	if err != nil {
		t.Fatal(err)
	}

	firstRecord, err := store.EnsureOrganizationAdminVerification(firstUser)
	if err != nil {
		t.Fatal(err)
	}
	secondRecord, err := store.EnsureOrganizationAdminVerification(secondUser)
	if err != nil {
		t.Fatal(err)
	}
	if firstRecord == nil || secondRecord == nil {
		t.Fatalf("expected verification records, got %+v and %+v", firstRecord, secondRecord)
	}
	if firstRecord.VerificationHost != secondRecord.VerificationHost {
		t.Fatalf("verification host mismatch: %q vs %q", firstRecord.VerificationHost, secondRecord.VerificationHost)
	}
	if firstRecord.VerificationValue == secondRecord.VerificationValue {
		t.Fatalf("same-domain users should have distinct per-user TXT values, both got %q", firstRecord.VerificationValue)
	}
	wantFirstValue, err := organizationAdminVerificationTXTValueForEmail(firstUser.Email, []byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatal(err)
	}
	if firstRecord.VerificationValue != wantFirstValue {
		t.Fatalf("first verification value = %q, want %q", firstRecord.VerificationValue, wantFirstValue)
	}
	wantSecondValue, err := organizationAdminVerificationTXTValueForEmail(secondUser.Email, []byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatal(err)
	}
	if secondRecord.VerificationValue != wantSecondValue {
		t.Fatalf("second verification value = %q, want %q", secondRecord.VerificationValue, wantSecondValue)
	}
}

func TestOrganizationDomainValidationHandler(t *testing.T) {
	app, store, user, sessionToken := newOrganizationAdminTestApp(t)
	record, err := store.EnsureOrganizationAdminVerification(user)
	if err != nil {
		t.Fatal(err)
	}
	if record == nil {
		t.Fatal("expected verification record")
	}
	form := url.Values{"csrf_token": {app.csrfTokenFromSession(sessionToken)}}

	app.lookupNS = func(name string) ([]*net.NS, error) {
		if name != record.DomainName {
			t.Fatalf("NS lookup domain = %q, want %q", name, record.DomainName)
		}
		return []*net.NS{{Host: "ns1.example.net."}}, nil
	}
	app.lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		if host != "ns1.example.net" {
			t.Fatalf("nameserver host = %q, want ns1.example.net", host)
		}
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}, nil
	}
	app.lookupTXTAtServer = func(ctx context.Context, serverAddr, host string) ([]string, error) {
		if serverAddr != "203.0.113.10:53" {
			t.Fatalf("server address = %q, want 203.0.113.10:53", serverAddr)
		}
		if host != record.VerificationHost+"." {
			t.Fatalf("lookup host = %q, want %q", host, record.VerificationHost+".")
		}
		return nil, &net.DNSError{Name: host, Err: "no such host", IsNotFound: true}
	}
	resp := organizationAdminPostRequest(t, app, sessionToken, form)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on failed validation, got %d: %s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["message"] == "" {
		t.Fatalf("expected validation failure message, got %v", payload)
	}
	details, ok := payload["details"].([]any)
	if !ok || len(details) == 0 {
		t.Fatalf("expected DNS attempt details in failure payload, got %v", payload)
	}
	storedAfterFailure, err := store.GetOrganizationAdminVerification(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedAfterFailure == nil || storedAfterFailure.LastCheckedAt.IsZero() {
		t.Fatalf("expected last checked timestamp after failure, got %+v", storedAfterFailure)
	}
	audits, err := store.ListAuditLogs("user_audit_logs", user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) == 0 || audits[0].Action != "organization_domain_validation_failed" {
		t.Fatalf("expected failure audit log, got %+v", audits)
	}
	if !strings.Contains(audits[0].DetailsJSON, `"dns_attempts"`) {
		t.Fatalf("expected dns_attempts in failure audit log, got %+v", audits[0])
	}

	app.lookupTXTAtServer = func(ctx context.Context, serverAddr, host string) ([]string, error) {
		if serverAddr != "203.0.113.10:53" {
			t.Fatalf("server address = %q, want 203.0.113.10:53", serverAddr)
		}
		if host != record.VerificationHost+"." {
			t.Fatalf("lookup host = %q, want %q", host, record.VerificationHost+".")
		}
		return []string{"unrelated=value", record.VerificationValue}, nil
	}
	resp = organizationAdminPostRequest(t, app, sessionToken, form)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 on successful validation, got %d: %s", resp.Code, resp.Body.String())
	}
	payload = map[string]any{}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["verified"] != true || payload["status_text"] != "Verified" {
		t.Fatalf("unexpected success payload: %v", payload)
	}
	storedAfterSuccess, err := store.GetOrganizationAdminVerification(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedAfterSuccess == nil || storedAfterSuccess.VerifiedAt.IsZero() {
		t.Fatalf("expected verified record after success, got %+v", storedAfterSuccess)
	}
	audits, err = store.ListAuditLogs("user_audit_logs", user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) == 0 || audits[0].Action != "organization_domain_verified" {
		t.Fatalf("expected success audit log first, got %+v", audits)
	}
	if !strings.Contains(audits[0].DetailsJSON, `"lookup_mode":"authoritative"`) {
		t.Fatalf("expected authoritative lookup mode in success audit log, got %+v", audits[0])
	}
}

func TestOrganizationDomainValidationUsesAuthoritativeNameserverLookup(t *testing.T) {
	app, store, user, sessionToken := newOrganizationAdminTestApp(t)
	record, err := store.EnsureOrganizationAdminVerification(user)
	if err != nil {
		t.Fatal(err)
	}
	if record == nil {
		t.Fatal("expected verification record")
	}
	form := url.Values{"csrf_token": {app.csrfTokenFromSession(sessionToken)}}
	var fallbackUsed bool
	var queryCount int
	app.lookupNS = func(name string) ([]*net.NS, error) {
		if name != record.DomainName {
			t.Fatalf("NS lookup domain = %q, want %q", name, record.DomainName)
		}
		return []*net.NS{{Host: "ns1.example.net."}}, nil
	}
	app.lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		if host != "ns1.example.net" {
			t.Fatalf("nameserver host = %q, want ns1.example.net", host)
		}
		return []net.IPAddr{
			{IP: net.ParseIP("203.0.113.10")},
			{IP: net.ParseIP("2001:db8::10")},
		}, nil
	}
	app.lookupTXTAtServer = func(ctx context.Context, serverAddr, host string) ([]string, error) {
		queryCount++
		if serverAddr != "203.0.113.10:53" {
			t.Fatalf("query should have stopped after first successful match; unexpected server %q", serverAddr)
		}
		if host != record.VerificationHost+"." {
			t.Fatalf("lookup host = %q, want %q", host, record.VerificationHost+".")
		}
		return []string{record.VerificationValue}, nil
	}
	app.lookupTXT = func(name string) ([]string, error) {
		fallbackUsed = true
		return nil, &net.DNSError{Name: name, Err: "unexpected fallback"}
	}

	resp := organizationAdminPostRequest(t, app, sessionToken, form)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 on authoritative validation, got %d: %s", resp.Code, resp.Body.String())
	}
	if fallbackUsed {
		t.Fatal("recursive TXT fallback should not be used when authoritative validation succeeds")
	}
	if queryCount != 1 {
		t.Fatalf("expected a single TXT query before validation succeeded, got %d", queryCount)
	}
	audits, err := store.ListAuditLogs("user_audit_logs", user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) == 0 || !strings.Contains(audits[0].DetailsJSON, `"lookup_mode":"authoritative"`) {
		t.Fatalf("expected authoritative lookup mode in audit log, got %+v", audits)
	}
}

func TestOrganizationDomainValidationReportsAuthoritativeLookupAttemptsOnFailure(t *testing.T) {
	app, store, user, sessionToken := newOrganizationAdminTestApp(t)
	record, err := store.EnsureOrganizationAdminVerification(user)
	if err != nil {
		t.Fatal(err)
	}
	if record == nil {
		t.Fatal("expected verification record")
	}
	form := url.Values{"csrf_token": {app.csrfTokenFromSession(sessionToken)}}
	app.lookupNS = func(name string) ([]*net.NS, error) {
		return []*net.NS{{Host: "ns1.example.net."}, {Host: "ns2.example.net."}}, nil
	}
	app.lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		switch host {
		case "ns1.example.net":
			return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}, nil
		case "ns2.example.net":
			return []net.IPAddr{{IP: net.ParseIP("203.0.113.11")}}, nil
		default:
			t.Fatalf("unexpected nameserver host %q", host)
			return nil, nil
		}
	}
	app.lookupTXTAtServer = func(ctx context.Context, serverAddr, host string) ([]string, error) {
		if host != record.VerificationHost+"." {
			t.Fatalf("lookup host = %q, want %q", host, record.VerificationHost+".")
		}
		return nil, &net.DNSError{Name: host, Err: "no such host", IsNotFound: true}
	}

	resp := organizationAdminPostRequest(t, app, sessionToken, form)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on authoritative validation failure, got %d: %s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	details, ok := payload["details"].([]any)
	if !ok || len(details) < 3 {
		t.Fatalf("expected multiple DNS attempt details, got %v", payload)
	}
	audits, err := store.ListAuditLogs("user_audit_logs", user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) == 0 || !strings.Contains(audits[0].DetailsJSON, `"lookup_mode":"authoritative"`) {
		t.Fatalf("expected authoritative lookup mode in audit log, got %+v", audits)
	}
	if !strings.Contains(audits[0].DetailsJSON, `"dns_server":"203.0.113.10:53"`) || !strings.Contains(audits[0].DetailsJSON, `"dns_server":"203.0.113.11:53"`) {
		t.Fatalf("expected authoritative DNS server details in audit log, got %+v", audits[0])
	}
}

func TestOrganizationAdminDeniedForNonAdminAfterDomainClaimed(t *testing.T) {
	app, store, owner, ownerSessionToken := newOrganizationAdminTestApp(t)
	record, err := store.EnsureOrganizationAdminVerification(owner)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkOrganizationAdminVerificationVerified(owner.ID, nowUTC()); err != nil {
		t.Fatal(err)
	}
	teammate, err := store.CreateOrUpdateUser("teammate@example.com", "Teammate", "", false)
	if err != nil {
		t.Fatal(err)
	}
	teammateSessionToken := "pst_org_admin_teammate"
	if err := store.CreateSession(teammate.ID, SHA256Hex(teammateSessionToken), nowUTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	resp := organizationAdminGetRequest(t, app, teammateSessionToken, "/org-admin")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected teammate org-admin page to be hidden after claim, got %d", resp.Code)
	}

	form := url.Values{"csrf_token": {app.csrfTokenFromSession(teammateSessionToken)}}
	resp = organizationAdminPostRequest(t, app, teammateSessionToken, form)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected teammate validation endpoint to be hidden after claim, got %d", resp.Code)
	}

	home := organizationAdminGetRequest(t, app, teammateSessionToken, "/")
	if home.Code != http.StatusOK {
		t.Fatalf("expected teammate dashboard to load, got %d", home.Code)
	}
	if strings.Contains(home.Body.String(), `Organization Admin`) || strings.Contains(home.Body.String(), `Access organization administration`) {
		t.Fatal("teammate should not see organization admin navigation after domain has been claimed")
	}

	ownerHome := organizationAdminGetRequest(t, app, ownerSessionToken, "/")
	if ownerHome.Code != http.StatusOK {
		t.Fatalf("expected owner dashboard to load, got %d", ownerHome.Code)
	}
	if !strings.Contains(ownerHome.Body.String(), `Organization Admin`) {
		t.Fatal("verified domain admin should keep organization admin navigation")
	}

	if record == nil {
		t.Fatal("expected owner verification record")
	}
}
