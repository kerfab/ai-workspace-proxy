// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUserPolicyAPIRejectsStandardAgentKey(t *testing.T) {
	app, _, standardToken, _ := newPolicyAPITestApp(t)
	resp := policyAPIRequest(t, app, http.MethodGet, "/api/user/policies", standardToken, "")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected standard agent key to be rejected with 401, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUserPolicyAPICreateUpdateApplyDelete(t *testing.T) {
	app, store, _, backendToken := newPolicyAPITestApp(t)

	createResp := policyAPIRequest(t, app, http.MethodPost, "/api/user/policies", backendToken, `{
		"name": "Test Policy",
		"capabilities": ["gmail_messages_read"]
	}`)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create failed with %d: %s", createResp.Code, createResp.Body.String())
	}
	policyID := responsePolicyID(t, createResp)
	if policyID == "" || policyID == systemPolicyID {
		t.Fatalf("unexpected created policy ID: %q", policyID)
	}

	updateResp := policyAPIRequest(t, app, http.MethodPatch, "/api/user/policies/"+policyID, backendToken, `{
		"name": "Updated Test Policy",
		"capabilities": ["calendar_read", "gmail_messages_read"]
	}`)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update failed with %d: %s", updateResp.Code, updateResp.Body.String())
	}

	applyDefaultResp := policyAPIRequest(t, app, http.MethodPost, "/api/user/policies/"+policyID+"/apply", backendToken, `{}`)
	if applyDefaultResp.Code != http.StatusOK {
		t.Fatalf("apply default failed with %d: %s", applyDefaultResp.Code, applyDefaultResp.Body.String())
	}
	settings, err := store.GetUserSettings("usr_test")
	if err != nil {
		t.Fatal(err)
	}
	if settings.DefaultPolicyID != policyID {
		t.Fatalf("default policy = %q, want %q", settings.DefaultPolicyID, policyID)
	}
	if err := store.SaveAgentWorkspaceGrants("usr_test", "agt_test_standard", []AgentWorkspaceGrant{
		{AgentID: "agt_test_standard", UserID: "usr_test", MailboxEmail: "workspace@example.com", PolicyID: policyID},
	}); err != nil {
		t.Fatal(err)
	}

	applyWorkspaceResp := policyAPIRequest(t, app, http.MethodPost, "/api/user/policies/system/apply", backendToken, `{"workspace":"workspace@example.com"}`)
	assertPolicyAPIError(t, applyWorkspaceResp, http.StatusBadRequest, "workspace_policy_deprecated")

	deleteResp := policyAPIRequest(t, app, http.MethodDelete, "/api/user/policies/"+policyID, backendToken, "")
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete failed with %d: %s", deleteResp.Code, deleteResp.Body.String())
	}
	settings, err = store.GetUserSettings("usr_test")
	if err != nil {
		t.Fatal(err)
	}
	if settings.DefaultPolicyID != systemPolicyID {
		t.Fatalf("default policy after delete = %q, want %q", settings.DefaultPolicyID, systemPolicyID)
	}
	grant, err := store.GetAgentWorkspaceGrant("usr_test", "agt_test_standard", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if grant == nil || grant.PolicyID != systemPolicyID {
		t.Fatalf("agent grant policy after delete = %+v, want system", grant)
	}
}

func TestUserPolicyAPISystemPolicyIsImmutableButApplyable(t *testing.T) {
	app, store, _, backendToken := newPolicyAPITestApp(t)

	createSystemResp := policyAPIRequest(t, app, http.MethodPost, "/api/user/policies", backendToken, `{
		"id": "system",
		"name": "System",
		"capabilities": ["gmail_messages_read"]
	}`)
	assertPolicyAPIError(t, createSystemResp, http.StatusBadRequest, "system_policy_immutable")

	updateSystemResp := policyAPIRequest(t, app, http.MethodPut, "/api/user/policies/system", backendToken, `{
		"name": "System",
		"capabilities": ["gmail_messages_read"]
	}`)
	assertPolicyAPIError(t, updateSystemResp, http.StatusBadRequest, "system_policy_immutable")

	deleteSystemResp := policyAPIRequest(t, app, http.MethodDelete, "/api/user/policies/system", backendToken, "")
	assertPolicyAPIError(t, deleteSystemResp, http.StatusBadRequest, "system_policy_immutable")

	applySystemResp := policyAPIRequest(t, app, http.MethodPost, "/api/user/policies/system/apply", backendToken, "")
	if applySystemResp.Code != http.StatusOK {
		t.Fatalf("system apply failed with %d: %s", applySystemResp.Code, applySystemResp.Body.String())
	}
	settings, err := store.GetUserSettings("usr_test")
	if err != nil {
		t.Fatal(err)
	}
	if settings.DefaultPolicyID != systemPolicyID {
		t.Fatalf("default policy = %q, want %q", settings.DefaultPolicyID, systemPolicyID)
	}
}

func TestPolicyEditorViewDefaultsToSavedCustomPolicy(t *testing.T) {
	app, store, _, _ := newPolicyAPITestApp(t)

	policy := &UserPolicy{
		UserID:              "usr_test",
		Name:                "Custom default policy",
		EnabledCapabilities: []string{"gmail_messages_read"},
	}
	if err := store.SaveUserPolicy(policy); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDefaultPolicyID("usr_test", policy.ID); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/policies", nil)
	view := app.policyEditorView("usr_test", req)

	if got := view["SelectedID"]; got != policy.ID {
		t.Fatalf("selected policy id = %v, want %q", got, policy.ID)
	}
	if got := view["SelectedName"]; got != policy.Name {
		t.Fatalf("selected policy name = %v, want %q", got, policy.Name)
	}
	if got := view["SelectedIsDefault"]; got != true {
		t.Fatalf("selected policy default flag = %v, want true", got)
	}

	options, ok := view["Options"].([]map[string]any)
	if !ok {
		t.Fatalf("options type = %T, want []map[string]any", view["Options"])
	}
	foundSelected := false
	for _, option := range options {
		if option["ID"] == policy.ID {
			if option["Selected"] != true {
				t.Fatalf("custom default option selected = %v, want true", option["Selected"])
			}
			foundSelected = true
		}
	}
	if !foundSelected {
		t.Fatalf("did not find custom default policy option %q", policy.ID)
	}
	if len(options) < 2 {
		t.Fatalf("expected at least 2 options, got %d", len(options))
	}
	if got := options[0]["ID"]; got != policy.ID {
		t.Fatalf("first option id = %v, want %q", got, policy.ID)
	}
	if got := options[1]["ID"]; got != systemPolicyID {
		t.Fatalf("second option id = %v, want %q", got, systemPolicyID)
	}
}

func TestPolicyOptionsForUserOrdersRemainingCustomPoliciesAlphabetically(t *testing.T) {
	app, store, _, _ := newPolicyAPITestApp(t)
	userID := "usr_test"
	for _, policy := range []*UserPolicy{
		{ID: "pol_zulu", UserID: userID, Name: "Zulu", EnabledCapabilities: []string{"gmail_messages_read"}},
		{ID: "pol_alpha", UserID: userID, Name: "Alpha", EnabledCapabilities: []string{"gmail_messages_read"}},
		{ID: "pol_beta", UserID: userID, Name: "beta", EnabledCapabilities: []string{"gmail_messages_read"}},
	} {
		if err := store.SaveUserPolicy(policy); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SaveDefaultPolicyID(userID, "pol_beta"); err != nil {
		t.Fatal(err)
	}

	options := app.policyOptionsForUser(userID, "pol_beta")
	if len(options) != 4 {
		t.Fatalf("options length = %d, want 4", len(options))
	}
	want := []string{"pol_beta", systemPolicyID, "pol_alpha", "pol_zulu"}
	for i, wantID := range want {
		if got := options[i]["ID"]; got != wantID {
			t.Fatalf("option %d id = %v, want %q", i, got, wantID)
		}
	}
}

func newPolicyAPITestApp(t *testing.T) (*App, *Store, string, string) {
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
		ID:        "usr_test",
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
	standardToken := "atk_test_standard"
	standardEnc, err := crypto.Encrypt(standardToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAgent(&AgentAccess{
		ID:              "agt_test_standard",
		UserID:          user.ID,
		FriendlyName:    "Test agent",
		DefaultLocation: "test suite",
		TokenEnc:        standardEnc,
		TokenHint:       "atk_test...",
		Enabled:         true,
	}); err != nil {
		t.Fatal(err)
	}
	backendToken := "ubk_test_backend"
	backendEnc, err := crypto.Encrypt(backendToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveUserBackendAPIToken(user.ID, backendEnc, "ubk_test..."); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveGmailConnection(&GmailConnection{
		UserID:          user.ID,
		MailboxEmail:    "workspace@example.com",
		FriendlyName:    "workspace",
		Scopes:          "scope",
		RefreshTokenEnc: "refresh",
		TokenExpiry:     time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	app := NewApp(&Config{
		BaseURL:             "http://localhost:8080",
		MaxRequestBodyBytes: 1 << 20,
	}, store, crypto)
	return app, store, standardToken, backendToken
}

func policyAPIRequest(t *testing.T, app *App, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	return resp
}

func responsePolicyID(t *testing.T, resp *httptest.ResponseRecorder) string {
	t.Helper()
	var payload struct {
		Policy struct {
			ID string `json:"id"`
		} `json:"policy"`
	}
	if err := json.NewDecoder(bytes.NewReader(resp.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload.Policy.ID
}

func assertPolicyAPIError(t *testing.T, resp *httptest.ResponseRecorder, wantStatus int, wantError string) {
	t.Helper()
	if resp.Code != wantStatus {
		t.Fatalf("expected status %d, got %d: %s", wantStatus, resp.Code, resp.Body.String())
	}
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(bytes.NewReader(resp.Body.Bytes())).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Error != wantError {
		t.Fatalf("expected error %q, got %q", wantError, payload.Error)
	}
}
