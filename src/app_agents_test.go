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

func TestAgentGrantsUpdateKeepsSelectedAgentOnRedirect(t *testing.T) {
	app, store, sessionToken := newAgentActionTestApp(t)
	form := agentActionForm(app, sessionToken, "agt_test")
	form.Add("workspace", "workspace@example.com")
	form.Set("policy_workspace_at_example_dot_com", systemPolicyID)
	form.Set("require_agent_motive_workspace_at_example_dot_com", "on")

	req := httptest.NewRequest(http.MethodPost, "/agents/grants", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	app.handleAgentGrantsUpdate(resp, req)

	if resp.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d: %s", resp.Code, resp.Body.String())
	}
	location := resp.Header().Get("Location")
	if location != "/agents?agent=agt_test&agents_saved=1#agent-agt_test" {
		t.Fatalf("unexpected redirect location: %s", location)
	}
	grant, err := store.GetAgentWorkspaceGrant("usr_agent_actions", "agt_test", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if grant == nil || grant.PolicyID != systemPolicyID || !grant.RequireAgentMotive {
		t.Fatalf("grant was not saved: %+v", grant)
	}
}

func TestAgentProfileUpdateKeepsSelectedAgentOnRedirect(t *testing.T) {
	app, store, sessionToken := newAgentActionTestApp(t)
	form := agentActionForm(app, sessionToken, "agt_test")
	form.Set("field", "friendly_name")
	form.Set("value", "Renamed Agent")

	req := httptest.NewRequest(http.MethodPost, "/agents/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	app.handleAgentUpdate(resp, req)

	if resp.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d: %s", resp.Code, resp.Body.String())
	}
	location := resp.Header().Get("Location")
	if location != "/agents?agent=agt_test&agents_saved=1#agent-agt_test" {
		t.Fatalf("unexpected redirect location: %s", location)
	}
	agent, err := store.GetAgent("usr_agent_actions", "agt_test")
	if err != nil {
		t.Fatal(err)
	}
	if agent == nil || agent.FriendlyName != "Renamed Agent" || agent.DefaultLocation != "test" {
		t.Fatalf("agent profile was not updated correctly: %+v", agent)
	}
	if !agent.SkillStale || !strings.Contains(agent.SkillStaleReason, "Agent profile changed.") {
		t.Fatalf("expected agent skill stale warning after profile update, got %+v", agent)
	}
}

func TestAgentActionsReturnJSONForAjaxForms(t *testing.T) {
	app, _, sessionToken := newAgentActionTestApp(t)

	renameForm := agentActionForm(app, sessionToken, "agt_test")
	renameForm.Set("field", "friendly_name")
	renameForm.Set("value", "Renamed Agent")
	renameResp := agentAjaxPost(t, app, sessionToken, "/agents/update", renameForm, app.handleAgentUpdate)
	renamePayload := assertAgentActionJSON(t, renameResp, "Agent renamed.")
	if renamePayload["friendly_name"] != "Renamed Agent" || renamePayload["default_location"] != "test" {
		t.Fatalf("unexpected rename payload: %v", renamePayload)
	}
	if renamePayload["skill_update_required"] != true {
		t.Fatalf("expected rename payload to require skill update: %v", renamePayload)
	}

	locationForm := agentActionForm(app, sessionToken, "agt_test")
	locationForm.Set("field", "default_location")
	locationForm.Set("value", "desktop")
	locationResp := agentAjaxPost(t, app, sessionToken, "/agents/update", locationForm, app.handleAgentUpdate)
	locationPayload := assertAgentActionJSON(t, locationResp, "Agent location updated.")
	if locationPayload["friendly_name"] != "Renamed Agent" || locationPayload["default_location"] != "desktop" {
		t.Fatalf("unexpected location payload: %v", locationPayload)
	}

	grantsForm := agentActionForm(app, sessionToken, "agt_test")
	grantsForm.Add("workspace", "workspace@example.com")
	grantsForm.Set("policy_workspace_at_example_dot_com", systemPolicyID)
	grantsResp := agentAjaxPost(t, app, sessionToken, "/agents/grants", grantsForm, app.handleAgentGrantsUpdate)
	grantsPayload := assertAgentActionJSON(t, grantsResp, "Workspace grants updated.")
	if grantsPayload["skill_update_required"] != true {
		t.Fatalf("expected grants payload to require skill update: %v", grantsPayload)
	}

	toggleForm := agentActionForm(app, sessionToken, "agt_test")
	toggleResp := agentAjaxPost(t, app, sessionToken, "/agents/toggle", toggleForm, app.handleAgentToggle)
	togglePayload := assertAgentActionJSON(t, toggleResp, "Agent access status updated.")
	if togglePayload["enabled"] != false {
		t.Fatalf("expected disabled agent after toggle, got: %v", togglePayload)
	}

	rotateForm := agentActionForm(app, sessionToken, "agt_test")
	rotateResp := agentAjaxPost(t, app, sessionToken, "/agents/rotate", rotateForm, app.handleAgentRotate)
	rotatePayload := assertAgentActionJSON(t, rotateResp, "Agent API key rotated.")
	if hint, _ := rotatePayload["token_hint"].(string); !strings.HasPrefix(hint, "atk_") || len(hint) != len("atk_000...000") || !strings.Contains(hint, "...") {
		t.Fatalf("unexpected rotated token hint: %v", rotatePayload)
	}
	if rotatePayload["skill_update_required"] != true {
		t.Fatalf("expected rotate payload to require skill update: %v", rotatePayload)
	}
}

func TestAgentSkillWarningDismissClearsStoredWarning(t *testing.T) {
	app, store, sessionToken := newAgentActionTestApp(t)
	if err := store.MarkAgentSkillStale("usr_agent_actions", "agt_test", "Workspace grants changed."); err != nil {
		t.Fatal(err)
	}
	form := agentActionForm(app, sessionToken, "agt_test")
	resp := agentAjaxPost(t, app, sessionToken, "/agents/skill-warning/dismiss", form, app.handleAgentSkillWarningDismiss)
	payload := map[string]any{}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if resp.Code != http.StatusOK || payload["skill_update_required"] != false {
		t.Fatalf("unexpected dismiss response %d: %v", resp.Code, payload)
	}
	agent, err := store.GetAgent("usr_agent_actions", "agt_test")
	if err != nil {
		t.Fatal(err)
	}
	if agent == nil || agent.SkillStale || agent.SkillStaleReason != "" {
		t.Fatalf("expected stale warning to be cleared, got %+v", agent)
	}
}

func TestAgentDriveFolderGrantsUpdateAndFolderDeleteCleanup(t *testing.T) {
	app, store, sessionToken := newAgentActionTestApp(t)
	folderA := &AllowedDriveFolder{
		ID:            "dfr_a",
		UserID:        "usr_agent_actions",
		MailboxEmail:  "workspace@example.com",
		ReferenceName: "Reports",
		ReferenceKey:  "reports",
		FolderURL:     "https://drive.google.com/drive/folders/folder_a",
		FolderID:      "folder_a",
		FolderName:    "Reports",
	}
	folderB := &AllowedDriveFolder{
		ID:            "dfr_b",
		UserID:        "usr_agent_actions",
		MailboxEmail:  "workspace@example.com",
		ReferenceName: "Archive",
		ReferenceKey:  "archive",
		FolderURL:     "https://drive.google.com/drive/folders/folder_b",
		FolderID:      "folder_b",
		FolderName:    "Archive",
	}
	if err := store.CreateDriveFolderRef(folderA); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateDriveFolderRef(folderB); err != nil {
		t.Fatal(err)
	}

	form := agentActionForm(app, sessionToken, "agt_test")
	form.Set("workspace", "workspace@example.com")
	form.Add("folder_ref_id", "dfr_a")
	resp := agentAjaxPost(t, app, sessionToken, "/agents/drive-folders/grants", form, app.handleAgentDriveFolderGrantsUpdate)
	payload := assertAgentActionJSON(t, resp, "Allowed Drive folders updated.")
	if payload["allowed_count"] != float64(1) {
		t.Fatalf("unexpected folder grant payload: %v", payload)
	}
	allowed, err := store.ListAgentAllowedDriveFolderRefs("usr_agent_actions", "agt_test", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(allowed) != 1 || allowed[0].ID != "dfr_a" {
		t.Fatalf("unexpected allowed folders after save: %+v", allowed)
	}
	workspaceGrant, err := store.GetAgentWorkspaceGrant("usr_agent_actions", "agt_test", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if workspaceGrant != nil {
		t.Fatalf("Drive folder grant update should not implicitly allow Workspace access: %+v", workspaceGrant)
	}
	auditLogs, err := store.ListAuditLogs("user_audit_logs", "usr_agent_actions", 10)
	if err != nil {
		t.Fatal(err)
	}
	foundAudit := false
	for _, entry := range auditLogs {
		if entry.Action == "agent_drive_folder_grants_updated" && entry.TargetID == "agt_test" && entry.WorkspaceEmail == "" && strings.Contains(entry.DetailsJSON, `"workspace":"workspace@example.com"`) {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("expected agent drive folder grant update to be logged, got %+v", auditLogs)
	}

	folderA.ReferenceName = "Monthly Reports"
	folderA.ReferenceKey = "monthly-reports"
	if err := store.UpdateDriveFolderRef(folderA); err != nil {
		t.Fatal(err)
	}
	allowed, err = store.ListAgentAllowedDriveFolderRefs("usr_agent_actions", "agt_test", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(allowed) != 1 || allowed[0].ReferenceName != "Monthly Reports" {
		t.Fatalf("folder grant did not survive friendly name rename: %+v", allowed)
	}

	if err := store.DeleteDriveFolderRef("usr_agent_actions", "workspace@example.com", "dfr_a"); err != nil {
		t.Fatal(err)
	}
	allowed, err = store.ListAgentAllowedDriveFolderRefs("usr_agent_actions", "agt_test", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(allowed) != 0 {
		t.Fatalf("deleted folder should be removed from agent grants: %+v", allowed)
	}
}

func TestDeleteAgentRemovesAgentDataButKeepsRequestLogs(t *testing.T) {
	_, store, _ := newAgentActionTestApp(t)
	if err := store.SaveAgentWorkspaceGrants("usr_agent_actions", "agt_test", []AgentWorkspaceGrant{
		{AgentID: "agt_test", UserID: "usr_agent_actions", MailboxEmail: "workspace@example.com", PolicyID: systemPolicyID},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateDriveFolderRef(&AllowedDriveFolder{
		ID:            "dfr_agent_delete",
		UserID:        "usr_agent_actions",
		MailboxEmail:  "workspace@example.com",
		ReferenceName: "Reports",
		ReferenceKey:  "reports",
		FolderURL:     "https://drive.google.com/drive/folders/folder_delete",
		FolderID:      "folder_delete",
		FolderName:    "Reports",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAgentDriveFolderGrants("usr_agent_actions", "agt_test", "workspace@example.com", []string{"dfr_agent_delete"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAgentSkillDownloadToken("usr_agent_actions", "agt_test", SHA256Hex("download_token"), "openclaw", nowUTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRequestLog(&RequestLogEntry{
		RequestID:             "req_agent_delete_keeps_log",
		UserID:                "usr_agent_actions",
		UserEmail:             "owner@example.com",
		WorkspaceEmail:        "workspace@example.com",
		AgentID:               "agt_test",
		Method:                http.MethodGet,
		Service:               "gmail",
		Path:                  "/gmail/v1/users/me/messages",
		UserAgent:             "test-agent",
		RemoteAddr:            "127.0.0.1:1234",
		AgentName:             "Agent",
		AgentLocation:         "test",
		AgentMotive:           "testing delete retention",
		Outcome:               "Success",
		HTTPStatus:            200,
		UpstreamStatus:        200,
		PolicyID:              systemPolicyID,
		PolicyName:            "Default system policy",
		PolicyCapabilityKey:   "gmail_messages_read",
		PolicyCapabilityTitle: "Read emails",
		PolicyRuleName:        "gmail_messages_list",
	}); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteAgent("usr_agent_actions", "agt_test"); err != nil {
		t.Fatal(err)
	}
	agent, err := store.GetAgent("usr_agent_actions", "agt_test")
	if err != nil {
		t.Fatal(err)
	}
	if agent != nil {
		t.Fatalf("agent profile/API key row still exists after delete: %+v", agent)
	}
	grants, err := store.ListAgentWorkspaceGrants("usr_agent_actions", "agt_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("agent workspace grants were not deleted: %+v", grants)
	}
	folderGrants, err := store.ListAgentDriveFolderGrants("usr_agent_actions", "agt_test", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(folderGrants) != 0 {
		t.Fatalf("agent drive folder grants were not deleted: %+v", folderGrants)
	}
	_, _, _, ok, expired, err := store.ConsumeAgentSkillDownloadToken(SHA256Hex("download_token"))
	if err != nil {
		t.Fatal(err)
	}
	if ok || expired {
		t.Fatalf("agent skill download token still exists after delete: ok=%v expired=%v", ok, expired)
	}
	count, err := store.CountRequestLogsByAgentSince("usr_agent_actions", "agt_test", nowUTC().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("request logs should be retained after agent delete, got count %d", count)
	}
}

func TestProxyDeniesDriveWhenAgentHasNoAllowedDriveFolders(t *testing.T) {
	app, store, _ := newAgentActionTestApp(t)
	if err := store.SaveAgentWorkspaceGrants("usr_agent_actions", "agt_test", []AgentWorkspaceGrant{
		{AgentID: "agt_test", UserID: "usr_agent_actions", MailboxEmail: "workspace@example.com", PolicyID: systemPolicyID},
	}); err != nil {
		t.Fatal(err)
	}
	accessEnc, err := app.crypto.Encrypt("access_token")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := store.GetGmailConnection("usr_agent_actions", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	conn.AccessTokenEnc = accessEnc
	conn.TokenExpiry = nowUTC().Add(time.Hour)
	if err := store.SaveGmailConnection(conn); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/drive.googleapis.com/drive/v3/files?workspace=workspace@example.com", nil)
	req.Header.Set("Authorization", "Bearer atk_test")
	req.Header.Set("X-AIWP-Agent-Motive", "Testing Drive folder access enforcement.")
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected Drive request without folder grants to be rejected, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "no allowed Drive folders configured for this agent") {
		t.Fatalf("unexpected Drive folder denial response: %s", resp.Body.String())
	}
}

func TestAddDriveFolderRejectsDuplicateFolderLink(t *testing.T) {
	app, store, sessionToken := newAgentActionTestApp(t)
	if err := store.CreateDriveFolderRef(&AllowedDriveFolder{
		ID:            "dfr_existing",
		UserID:        "usr_agent_actions",
		MailboxEmail:  "workspace@example.com",
		ReferenceName: "Existing",
		ReferenceKey:  "existing",
		FolderURL:     "https://drive.google.com/drive/folders/folder_123?usp=drive_link",
		FolderID:      "folder_123",
		FolderName:    "Existing Folder",
	}); err != nil {
		t.Fatal(err)
	}
	accessEnc, err := app.crypto.Encrypt("access_token")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := store.GetGmailConnection("usr_agent_actions", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	conn.AccessTokenEnc = accessEnc
	conn.TokenExpiry = nowUTC().Add(time.Hour)
	if err := store.SaveGmailConnection(conn); err != nil {
		t.Fatal(err)
	}
	app.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"id":"folder_123","name":"Existing Folder","mimeType":"application/vnd.google-apps.folder"}`)),
			Request:    req,
		}, nil
	})}

	form := url.Values{
		"csrf_token":     {app.csrfTokenFromSession(sessionToken)},
		"workspace":      {"workspace@example.com"},
		"reference_name": {"Duplicate"},
		"folder_link":    {"HTTPS://DRIVE.GOOGLE.COM/drive/folders/folder_123?usp=sharing&ignored=true"},
	}
	req := httptest.NewRequest(http.MethodPost, "/workspace/drive-folders/add", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	app.handleAddDriveFolder(resp, req)

	if resp.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d: %s", resp.Code, resp.Body.String())
	}
	location := resp.Header().Get("Location")
	if !strings.Contains(location, "folder_error=This+Google+Drive+folder+is+already+allowed+for+this+Workspace.") {
		t.Fatalf("expected duplicate folder error redirect, got %s", location)
	}
}

func TestProxyRequiresMotiveWhenAgentGrantEnforcesAccountability(t *testing.T) {
	app, store, sessionToken := newAgentActionTestApp(t)
	_ = sessionToken
	if err := store.SaveAgentWorkspaceGrants("usr_agent_actions", "agt_test", []AgentWorkspaceGrant{
		{
			AgentID:            "agt_test",
			UserID:             "usr_agent_actions",
			MailboxEmail:       "workspace@example.com",
			PolicyID:           systemPolicyID,
			RequireAgentMotive: true,
		},
	}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/gmail.googleapis.com/gmail/v1/users/me/messages?workspace=workspace@example.com", nil)
	req.Header.Set("Authorization", "Bearer atk_test")
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected missing motive to be rejected, got %d: %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "agent motive is required") {
		t.Fatalf("unexpected missing motive response: %s", resp.Body.String())
	}
	logs, err := store.ListRequestLogs("usr_agent_actions", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Outcome != "Fail" || logs[0].AgentID != "agt_test" || logs[0].WorkspaceEmail != "workspace@example.com" {
		t.Fatalf("unexpected request logs after accountability denial: %+v", logs)
	}
}

func TestParseAgentFirewallValueCanonicalizesAddresses(t *testing.T) {
	for _, tc := range []struct {
		raw         string
		wantValue   string
		wantVersion string
		wantKind    string
	}{
		{raw: "203.0.113.10", wantValue: "203.0.113.10", wantVersion: "IPv4", wantKind: "Address"},
		{raw: "203.0.113.12/24", wantValue: "203.0.113.0/24", wantVersion: "IPv4", wantKind: "Range"},
		{raw: "2001:db8::1", wantValue: "2001:db8::1", wantVersion: "IPv6", wantKind: "Address"},
		{raw: "2001:db8::abcd/48", wantValue: "2001:db8::/48", wantVersion: "IPv6", wantKind: "Range"},
	} {
		value, version, kind, err := parseAgentFirewallValue(tc.raw)
		if err != nil {
			t.Fatalf("%s should parse: %v", tc.raw, err)
		}
		if value != tc.wantValue || version != tc.wantVersion || kind != tc.wantKind {
			t.Fatalf("%s parsed as value=%q version=%q kind=%q", tc.raw, value, version, kind)
		}
	}
	if _, _, _, err := parseAgentFirewallValue("not-an-ip"); err == nil {
		t.Fatal("expected invalid IP address to fail")
	}
}

func TestAgentFirewallUsesDirectRemoteAddressAndIgnoresForwardedHeaders(t *testing.T) {
	app, store, _ := newAgentActionTestApp(t)
	agent, err := store.GetAgent("usr_agent_actions", "agt_test")
	if err != nil || agent == nil {
		t.Fatalf("agent lookup failed: %v", err)
	}
	agent.FirewallEnabled = true
	if err := store.SaveAgent(agent); err != nil {
		t.Fatal(err)
	}
	_, ipVersion, addressKind, err := parseAgentFirewallValue("198.51.100.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddAgentFirewallRule(&AgentFirewallRule{
		ID:          "afw_block_test",
		AgentID:     "agt_test",
		UserID:      "usr_agent_actions",
		Value:       "198.51.100.0/24",
		IPVersion:   ipVersion,
		AddressKind: addressKind,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAgentWorkspaceGrants("usr_agent_actions", "agt_test", []AgentWorkspaceGrant{
		{AgentID: "agt_test", UserID: "usr_agent_actions", MailboxEmail: "workspace@example.com", PolicyID: systemPolicyID},
	}); err != nil {
		t.Fatal(err)
	}

	deniedReq := httptest.NewRequest(http.MethodGet, "/gmail.googleapis.com/gmail/v1/users/me/messages?workspace=workspace@example.com", nil)
	deniedReq.Header.Set("Authorization", "Bearer atk_test")
	deniedReq.Header.Set("X-Forwarded-For", "198.51.100.10")
	deniedReq.RemoteAddr = "192.0.2.77:4567"
	deniedResp := httptest.NewRecorder()
	app.Routes().ServeHTTP(deniedResp, deniedReq)
	if deniedResp.Code != http.StatusForbidden {
		t.Fatalf("expected firewall denial, got %d: %s", deniedResp.Code, deniedResp.Body.String())
	}
	if !strings.Contains(deniedResp.Body.String(), "agent firewall denied request from 192.0.2.77") {
		t.Fatalf("unexpected firewall denial: %s", deniedResp.Body.String())
	}

	if err := store.AddAgentFirewallRule(&AgentFirewallRule{
		ID:          "afw_allow_test",
		AgentID:     "agt_test",
		UserID:      "usr_agent_actions",
		Value:       "192.0.2.0/24",
		IPVersion:   "IPv4",
		AddressKind: "Range",
	}); err != nil {
		t.Fatal(err)
	}
	accessEnc, err := app.crypto.Encrypt("access_token")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := store.GetGmailConnection("usr_agent_actions", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	conn.AccessTokenEnc = accessEnc
	conn.TokenExpiry = nowUTC().Add(time.Hour)
	if err := store.SaveGmailConnection(conn); err != nil {
		t.Fatal(err)
	}
	app.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"messages":[]}`)),
			Request:    req,
		}, nil
	})}
	allowedReq := httptest.NewRequest(http.MethodGet, "/gmail.googleapis.com/gmail/v1/users/me/messages?workspace=workspace@example.com", nil)
	allowedReq.Header.Set("Authorization", "Bearer atk_test")
	allowedReq.RemoteAddr = "192.0.2.55:4567"
	allowedResp := httptest.NewRecorder()
	app.Routes().ServeHTTP(allowedResp, allowedReq)
	if allowedResp.Code != http.StatusOK {
		t.Fatalf("expected allowed request, got %d: %s", allowedResp.Code, allowedResp.Body.String())
	}
}

func newAgentActionTestApp(t *testing.T) (*App, *Store, string) {
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
	user.ID = "usr_agent_actions"
	if err := store.db.Exec(`UPDATE users SET id = ? WHERE email = ?`, user.ID, user.Email); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveGmailConnection(&GmailConnection{
		UserID:          user.ID,
		MailboxEmail:    "workspace@example.com",
		FriendlyName:    "Workspace",
		RefreshTokenEnc: "enc",
		TokenExpiry:     nowUTC().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	crypto := NewCrypto([]byte("12345678901234567890123456789012"))
	enc, err := crypto.Encrypt("atk_test")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAgent(&AgentAccess{
		ID:              "agt_test",
		UserID:          user.ID,
		FriendlyName:    "Agent",
		DefaultLocation: "test",
		TokenEnc:        enc,
		TokenHint:       "atk_tes...est",
		Enabled:         true,
	}); err != nil {
		t.Fatal(err)
	}
	sessionToken := "pst_agent_actions"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func agentActionForm(app *App, sessionToken, agentID string) url.Values {
	return url.Values{
		"csrf_token": {app.csrfTokenFromSession(sessionToken)},
		"agent_id":   {agentID},
	}
}

func agentAjaxPost(t *testing.T, app *App, sessionToken, path string, form url.Values, handler http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "fetch")
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	handler(resp, req)
	return resp
}

func assertAgentActionJSON(t *testing.T, resp *httptest.ResponseRecorder, message string) map[string]any {
	t.Helper()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "ok" || payload["message"] != message || payload["agent_id"] != "agt_test" {
		t.Fatalf("unexpected response payload: %v", payload)
	}
	return payload
}
