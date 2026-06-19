// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestAgentContextValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	agent := &AgentAccess{FriendlyName: "Codex", DefaultLocation: "local test"}
	if _, err := agentContextForAgentRequest(req, agent, true); err == nil {
		t.Fatal("expected missing required agent motive to fail")
	}
	req.Header.Set(agentMotiveHeader, "Inspecting a requested email so the user can answer it.")
	ctx, err := agentContextForAgentRequest(req, agent, true)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Name != "Codex" || ctx.Location != "local test" {
		t.Fatalf("unexpected agent context: %+v", ctx)
	}
	req.Header.Set(agentMotiveHeader, strings.Repeat("x", maxAgentMotiveLen+1))
	if _, err := agentContextForAgentRequest(req, agent, true); err == nil {
		t.Fatal("expected oversized motive to fail")
	}
}

func TestHumanApprovalHeaderValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	if _, err := validateHumanApprovalHeader(req); err == nil {
		t.Fatal("expected missing human approval to fail")
	}
	req.Header.Set(humanApprovalHeader, "Reviewed the draft with the user in chat before sending.")
	value, err := validateHumanApprovalHeader(req)
	if err != nil {
		t.Fatal(err)
	}
	if value != "Reviewed the draft with the user in chat before sending." {
		t.Fatalf("unexpected approval value %q", value)
	}
	req.Header.Set(humanApprovalHeader, strings.Repeat("x", maxHumanApprovalLen+1))
	if _, err := validateHumanApprovalHeader(req); err == nil {
		t.Fatal("expected oversized human approval to fail")
	}
}

func TestStaticVendorAssetsUseBrowserMIMETypes(t *testing.T) {
	store, _ := newLoggingTestStore(t)
	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	for _, tc := range []struct {
		path        string
		contentType string
	}{
		{path: "/static/app.css", contentType: "text/css"},
		{path: "/static/vendor/tabulator/tabulator.min.css", contentType: "text/css"},
		{path: "/static/vendor/tabulator/tabulator.min.js", contentType: "text/javascript"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		resp := httptest.NewRecorder()
		app.Routes().ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s returned HTTP %d", tc.path, resp.Code)
		}
		if got := resp.Header().Get("Content-Type"); !strings.HasPrefix(got, tc.contentType) {
			t.Fatalf("%s Content-Type = %q, want %q", tc.path, got, tc.contentType)
		}
	}
}

func TestFaviconDoesNotConsole404(t *testing.T) {
	store, _ := newLoggingTestStore(t)
	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("favicon returned HTTP %d", resp.Code)
	}
}

func TestRequestLogStoreSettingsAndStandardRetention(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	settings, err := store.GetUserLoggingSettings(userID)
	if err != nil {
		t.Fatal(err)
	}
	if settings.RequireAgentContext || settings.RetentionDays != 7 {
		t.Fatalf("unexpected default logging settings: %+v", settings)
	}
	if err := store.SaveUserLoggingSettings(userID, true, 3); err != nil {
		t.Fatal(err)
	}
	settings, err = store.GetUserLoggingSettings(userID)
	if err != nil {
		t.Fatal(err)
	}
	if !settings.RequireAgentContext || settings.RetentionDays != 3 {
		t.Fatalf("unexpected saved logging settings: %+v", settings)
	}

	oldEntry := &RequestLogEntry{
		RequestID:      "req_old",
		UserID:         userID,
		UserEmail:      "owner@example.com",
		WorkspaceEmail: "workspace@example.com",
		CreatedAt:      nowUTC().AddDate(0, 0, -8),
		Method:         http.MethodGet,
		Service:        "gmail",
		Path:           "/gmail/v1/users/me/profile",
		Outcome:        "Success",
		HTTPStatus:     200,
	}
	newEntry := *oldEntry
	newEntry.RequestID = "req_new"
	newEntry.CreatedAt = nowUTC()
	oldAudit := &AuditLogEntry{
		UserID:         userID,
		UserEmail:      "owner@example.com",
		WorkspaceEmail: "workspace@example.com",
		CreatedAt:      nowUTC().AddDate(0, 0, -8),
		Action:         "policy_updated",
		TargetID:       "pol_old",
		TargetName:     "Old policy",
	}
	newAudit := *oldAudit
	newAudit.ID = ""
	newAudit.CreatedAt = nowUTC()
	newAudit.TargetID = "pol_new"
	newAudit.TargetName = "New policy"
	if err := store.SaveRequestLog(oldEntry); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRequestLog(&newEntry); err != nil {
		t.Fatal(err)
	}
	if err := store.SavePolicyAuditLog(oldAudit); err != nil {
		t.Fatal(err)
	}
	userAudit := *oldAudit
	userAudit.ID = ""
	userAudit.Action = "user_settings_updated"
	userAudit.TargetID = "timezone"
	userAudit.TargetName = "Timezone"
	if err := store.SaveUserAuditLog(&userAudit); err != nil {
		t.Fatal(err)
	}
	if err := store.SavePolicyAuditLog(&newAudit); err != nil {
		t.Fatal(err)
	}
	if err := store.CleanupExpiredLogs(); err != nil {
		t.Fatal(err)
	}
	logs, err := store.ListRequestLogs(userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].RequestID != "req_new" {
		t.Fatalf("unexpected retained logs: %+v", logs)
	}
	audits, err := store.ListAuditLogs("policy_audit_logs", userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) != 1 || audits[0].TargetID != "pol_new" {
		t.Fatalf("unexpected retained audit logs: %+v", audits)
	}
	userAudits, err := store.ListAuditLogs("user_audit_logs", userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(userAudits) != 0 {
		t.Fatalf("expected old user audit logs to be cleaned, got %+v", userAudits)
	}
}

func TestPolicyDecisionIncludesCapabilityMetadata(t *testing.T) {
	engine := newTestPolicyEngine(t, []string{"contacts_read"})
	decision, err := engine.EvaluateDecision(http.MethodGet, "/v1/people:searchContacts", nil, PolicyEvalContext{})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed {
		t.Fatalf("expected request to be allowed: %+v", decision)
	}
	if decision.CapabilityKey != "contacts_read" || decision.CapabilityTitle != "Read contacts" || decision.RuleName != "people_contacts_search" {
		t.Fatalf("unexpected decision metadata: %+v", decision)
	}
}

func TestAgentSkillDownloadRequestLogging(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	crypto := NewCrypto([]byte("12345678901234567890123456789012"))
	agentToken := "atk_logging_download"
	enc, err := crypto.Encrypt(agentToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAgent(&AgentAccess{
		ID:              "agt_logging_download",
		UserID:          userID,
		FriendlyName:    "Codex",
		DefaultLocation: "local test",
		TokenEnc:        enc,
		TokenHint:       "atk_log...",
		Enabled:         true,
	}); err != nil {
		t.Fatal(err)
	}
	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, crypto)

	reqWithoutMotive := httptest.NewRequest(http.MethodGet, "/api/agent-skill/download?platform=generic", nil)
	reqWithoutMotive.Header.Set("Authorization", "Bearer "+agentToken)
	respWithoutMotive := httptest.NewRecorder()
	app.Routes().ServeHTTP(respWithoutMotive, reqWithoutMotive)
	if respWithoutMotive.Code != http.StatusOK {
		t.Fatalf("expected skill download without agent motive to succeed, got %d: %s", respWithoutMotive.Code, respWithoutMotive.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agent-skill/download?platform=generic", nil)
	req.Header.Set("Authorization", "Bearer "+agentToken)
	req.Header.Set(agentMotiveHeader, "Refreshing the installed Workspace skill at the user's request.")
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected skill download to succeed, got %d: %s", resp.Code, resp.Body.String())
	}

	logs, err := store.ListRequestLogs(userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 request logs, got %d", len(logs))
	}
	for _, log := range logs {
		if log.Service != "agent-skill" || log.Outcome != "Success" || log.AgentID != "agt_logging_download" || log.AgentName != "Codex" {
			t.Fatalf("unexpected agent-skill request log: %+v", log)
		}
	}
	if logs[0].AgentMotive == "" && logs[1].AgentMotive == "" {
		t.Fatal("expected one successful agent-skill download log to keep the provided agent motive")
	}
}

func TestAgentSkillInstallTokenDownloadIsUserAuditNotAgentRequest(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	crypto := NewCrypto([]byte("12345678901234567890123456789012"))
	agentToken := "atk_install_token_download"
	enc, err := crypto.Encrypt(agentToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAgent(&AgentAccess{
		ID:              "agt_install_download",
		UserID:          userID,
		FriendlyName:    "Jim",
		DefaultLocation: "OpenClaw on laptop",
		TokenEnc:        enc,
		TokenHint:       agentTokenHint(agentToken),
		Enabled:         true,
	}); err != nil {
		t.Fatal(err)
	}
	rawDownloadToken := "askl_install_download"
	if err := store.SaveAgentSkillDownloadToken(userID, "agt_install_download", SHA256Hex(rawDownloadToken), "generic", nowUTC().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, crypto)

	req := httptest.NewRequest(http.MethodGet, "/api/agent-skill/download?token="+url.QueryEscape(rawDownloadToken), nil)
	req.Header.Set("User-Agent", "curl/8.5.0")
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected install-token skill download to succeed, got %d: %s", resp.Code, resp.Body.String())
	}

	requestLogs, err := store.ListRequestLogs(userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(requestLogs) != 0 {
		t.Fatalf("install-token downloads should not create agent request logs: %+v", requestLogs)
	}
	audits, err := store.ListAuditLogs("user_audit_logs", userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) != 1 {
		t.Fatalf("expected one user audit log, got %d: %+v", len(audits), audits)
	}
	audit := audits[0]
	if audit.Action != "agent_skill_downloaded" || audit.TargetID != "agt_install_download" || audit.TargetName != "Jim" {
		t.Fatalf("unexpected user audit log: %+v", audit)
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(audit.DetailsJSON), &details); err != nil {
		t.Fatal(err)
	}
	if details["source"] != "install_token" || details["platform"] != "generic" || details["user_agent"] != "curl/8.5.0" {
		t.Fatalf("unexpected install-token audit details: %v", details)
	}
}

func TestRequestAndAuditLogsCannotBeDisabledByUserSettings(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	if err := store.SaveUserLoggingSettings(userID, true, 7); err != nil {
		t.Fatal(err)
	}
	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	user := &User{ID: userID, Email: "owner@example.com", Name: "Owner"}

	app.saveRequestLog(RequestLogEntry{
		RequestID:  "req_disabled",
		UserID:     userID,
		UserEmail:  "owner@example.com",
		CreatedAt:  nowUTC(),
		Method:     http.MethodGet,
		Service:    "gmail",
		Path:       "/gmail.googleapis.com/gmail/v1/users/me/profile",
		Outcome:    "Success",
		HTTPStatus: http.StatusOK,
	})
	auditReq := httptest.NewRequest(http.MethodPost, "/policies/save", nil)
	auditReq.RemoteAddr = "127.0.0.1:4567"
	auditReq.Header.Set("User-Agent", "audit-test-agent")
	app.logPolicyAudit(auditReq, user, "", "policy_created", "pol_test", "Test policy", map[string]any{})
	logs, err := store.ListRequestLogs(userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected request logging to remain enabled, got %+v", logs)
	}
	audits, err := store.ListAuditLogs("policy_audit_logs", userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) != 1 {
		t.Fatalf("expected audit logging to remain enabled, got %+v", audits)
	}
}

func TestUserAuditLogsCaptureAccountAndSettingsActions(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	user := &User{ID: userID, Email: "owner@example.com"}
	signInReq := httptest.NewRequest(http.MethodGet, "/auth/google/callback", nil)
	signInReq.RemoteAddr = "127.0.0.1:1234"
	signInReq.Header.Set("User-Agent", "test-agent")
	signInReq.Header.Set("X-Forwarded-For", "198.51.100.10")
	signInReq.Header.Set("X-Real-IP", "198.51.100.11")
	signInReq.Header.Set("Forwarded", "for=198.51.100.12")
	signInReq.Header.Set("CF-Connecting-IP", "198.51.100.13")
	app.logUserAudit(signInReq, user, "user_signed_in", userID, user.Email, map[string]any{
		"method":      "google_oauth",
		"remote_addr": "127.0.0.1:1234",
		"user_agent":  "test-agent",
	})
	settingsReq := httptest.NewRequest(http.MethodPost, "/settings/update", nil)
	settingsReq.RemoteAddr = "127.0.0.1:5678"
	app.logUserAudit(settingsReq, user, "user_settings_updated", "timezone", "Timezone", map[string]any{
		"previous_value": "UTC",
		"new_value":      "Europe/Paris",
		"source":         "ui",
	})
	audits, err := store.ListAuditLogs("user_audit_logs", userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) != 2 {
		t.Fatalf("expected 2 user audit logs, got %+v", audits)
	}
	byAction := map[string]AuditLogEntry{}
	for _, audit := range audits {
		byAction[audit.Action] = audit
	}
	if !strings.Contains(byAction["user_settings_updated"].DetailsJSON, `"new_value":"Europe/Paris"`) {
		t.Fatalf("unexpected settings audit record: %+v", byAction["user_settings_updated"])
	}
	if !strings.Contains(byAction["user_signed_in"].DetailsJSON, `"remote_addr":"127.0.0.1:1234"`) {
		t.Fatalf("unexpected sign-in audit record: %+v", byAction["user_signed_in"])
	}
	signInAudit := byAction["user_signed_in"]
	if signInAudit.RemoteAddr != "127.0.0.1:1234" || signInAudit.UserAgent != "test-agent" || signInAudit.XForwardedFor != "198.51.100.10" || signInAudit.XRealIP != "198.51.100.11" || signInAudit.Forwarded != "for=198.51.100.12" || signInAudit.CFConnectingIP != "198.51.100.13" {
		t.Fatalf("sign-in audit did not capture client metadata: %+v", signInAudit)
	}
	if byAction["user_settings_updated"].RemoteAddr != "127.0.0.1:5678" {
		t.Fatalf("settings audit did not capture remote address: %+v", byAction["user_settings_updated"])
	}
}

func TestParseRetentionDaysUsesWholeNumbersAndMax(t *testing.T) {
	if _, err := parseRetentionDays("1.5", 7); err == nil {
		t.Fatal("expected decimal retention days to fail")
	}
	if _, err := parseRetentionDays("8", 7); err == nil {
		t.Fatal("expected standard user max retention to be enforced")
	}
	n, err := parseRetentionDays("7", 7)
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Fatalf("unexpected retention days: %d", n)
	}
}

func TestRequestLogViewSettingsPersist(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, NewCrypto([]byte("12345678901234567890123456789012")))

	defaults, err := app.requestLogViewSettings(userID)
	if err != nil {
		t.Fatal(err)
	}
	if defaults.PageSize != 100 || len(defaults.VisibleColumns) == 0 {
		t.Fatalf("unexpected default request log view settings: %+v", defaults)
	}
	expectedDefaultColumns := "timestamp_local,log_type,service,action,target_object_type,target_name,outcome,workspace_email,agent_name"
	if strings.Join(defaults.VisibleColumns, ",") != expectedDefaultColumns {
		t.Fatalf("unexpected default request log columns: got %s want %s", strings.Join(defaults.VisibleColumns, ","), expectedDefaultColumns)
	}

	if err := store.SaveUserLogViewSettings(&UserLogViewSettings{
		UserID:         userID,
		ViewKey:        requestLogViewKey,
		VisibleColumns: []string{"timestamp_local", "outcome", "service"},
		ColumnWidths:   map[string]int{"timestamp_local": 260, "outcome": 12, "unknown": 999},
		PageSize:       150,
	}); err != nil {
		t.Fatal(err)
	}
	saved, err := app.requestLogViewSettings(userID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.PageSize != 150 || strings.Join(saved.VisibleColumns, ",") != "timestamp_local,outcome,service" {
		t.Fatalf("unexpected saved request log view settings: %+v", saved)
	}
	if saved.ColumnWidths["timestamp_local"] != 260 || saved.ColumnWidths["outcome"] != 48 || saved.ColumnWidths["unknown"] != 0 {
		t.Fatalf("unexpected saved request log column widths: %+v", saved.ColumnWidths)
	}
}

func TestRequestLogViewSettingsAPIAuditsChanges(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	cfg := &Config{
		BaseURL:           "http://localhost:8080",
		SessionCookieName: "session",
		EncryptionKey:     []byte("12345678901234567890123456789012"),
	}
	app := NewApp(cfg, store, NewCrypto([]byte("12345678901234567890123456789012")))
	sessionToken := "pst_log_view_settings"
	if err := store.CreateSession(userID, SHA256Hex(sessionToken), nowUTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	form := url.Values{
		"csrf_token":      {app.csrfTokenFromSession(sessionToken)},
		"visible_columns": {"timestamp_local,service,outcome"},
		"column_widths":   {`{"service":180}`},
		"page_size":       {"150"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/logs/request/view-settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("view settings API returned HTTP %d: %s", resp.Code, resp.Body.String())
	}
	audits, err := store.ListAuditLogs("user_audit_logs", userID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(audits) != 1 || audits[0].Action != "user_settings_updated" || audits[0].TargetID != requestLogViewKey {
		t.Fatalf("unexpected user audit logs: %+v", audits)
	}
	if !strings.Contains(audits[0].DetailsJSON, `"new_page_size":150`) || !strings.Contains(audits[0].DetailsJSON, `"source":"api"`) {
		t.Fatalf("view settings audit lacks meaningful details: %+v", audits[0])
	}
}

func TestRequestLogColumnsExposeSingleUserTimezoneTimestamp(t *testing.T) {
	columns := requestLogColumnByID()
	expectedDefaultColumns := "timestamp_local,log_type,service,action,target_object_type,target_name,outcome,workspace_email,agent_name"
	if strings.Join(defaultRequestLogColumns(), ",") != expectedDefaultColumns {
		t.Fatalf("unexpected default request log column order: got %s want %s", strings.Join(defaultRequestLogColumns(), ","), expectedDefaultColumns)
	}
	if columns["request_id"].Category != "Request" {
		t.Fatalf("Request ID should be listed in the Request column category, got %q", columns["request_id"].Category)
	}
	for id, col := range columns {
		wantDefault := strings.Contains(","+expectedDefaultColumns+",", ","+id+",")
		if col.DefaultVisible != wantDefault {
			t.Fatalf("default_visible mismatch for %s: got %v want %v", id, col.DefaultVisible, wantDefault)
		}
	}
	for _, col := range requestLogColumns {
		if col.ID == "timestamp_utc" || col.Label == "Timestamp UTC" {
			t.Fatalf("request log columns should expose only the user-timezone Timestamp column, got %+v", col)
		}
	}
	normalized := normalizeRequestLogColumns([]string{"timestamp_local", "timestamp_utc", "request_id"})
	if strings.Join(normalized, ",") != "timestamp_local,request_id" {
		t.Fatalf("timestamp_utc should be ignored when normalizing log columns, got %+v", normalized)
	}
	row := requestLogRowMap(RequestLogEntry{CreatedAt: time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)}, "Europe/Paris")
	if row["timestamp_utc"] != nil {
		t.Fatalf("request log rows should not expose timestamp_utc, got %+v", row)
	}
	if got := row["timestamp_local"]; got != "2026-06-15 14:00:00 CEST" {
		t.Fatalf("timestamp should use the user's timezone, got %q", got)
	}
}

func TestRequestLogColumnsFollowCanonicalOrder(t *testing.T) {
	expectedOrder := []string{
		"timestamp_local",
		"log_type",
		"service",
		"action",
		"method",
		"path",
		"query",
		"request_id",
		"log_id",
		"target_object_type",
		"target_name",
		"target_object_id",
		"outcome",
		"http_status",
		"upstream_status",
		"error_message",
		"policy_capability_title",
		"policy_name",
		"policy_rule_name",
		"policy_id",
		"policy_capability_key",
		"workspace_email",
		"user_email",
		"agent_name",
		"agent_location",
		"agent_id",
		"agent_motive",
		"human_approval",
		"remote_addr",
		"cf_connecting_ip",
		"x_forwarded_for",
		"x_real_ip",
		"forwarded",
		"user_agent",
		"details_json",
	}
	actualOrder := make([]string, 0, len(requestLogColumns))
	for _, col := range requestLogColumns {
		actualOrder = append(actualOrder, col.ID)
	}
	if strings.Join(actualOrder, ",") != strings.Join(expectedOrder, ",") {
		t.Fatalf("unexpected request log canonical order: got %+v want %+v", actualOrder, expectedOrder)
	}
}

func TestInferRequestLogTargetObjectFromPathAndQuery(t *testing.T) {
	tests := []struct {
		name     string
		service  string
		path     string
		query    string
		wantType string
		wantID   string
	}{
		{name: "gmail message", service: "gmail", path: "/gmail/v1/users/me/messages/msg_123", wantType: "Gmail message", wantID: "msg_123"},
		{name: "gmail draft", service: "gmail", path: "/gmail/v1/users/me/drafts/draft_123", wantType: "Gmail draft", wantID: "draft_123"},
		{name: "calendar event", service: "calendar", path: "/calendar/v3/calendars/primary/events/event_123", wantType: "Calendar event", wantID: "primary/event_123"},
		{name: "drive file", service: "drive", path: "/drive/v3/files/file_123", wantType: "Drive file", wantID: "file_123"},
		{name: "drive comment reply", service: "drive", path: "/drive/v3/files/file_123/comments/comment_456/replies/reply_789", wantType: "Drive comment reply", wantID: "file_123/comment_456/reply_789"},
		{name: "drive folder id query", service: "drive", path: "/drive/v3/files", query: "driveFolderId=folder_123", wantType: "Drive folder", wantID: "folder_123"},
		{name: "drive ref query", service: "docs", path: "/v1/documents", query: "driveRef=Reports", wantType: "Drive folder reference", wantID: "Reports"},
		{name: "google doc", service: "docs", path: "/v1/documents/doc_123:batchUpdate", wantType: "Google Doc", wantID: "doc_123"},
		{name: "google sheet", service: "sheets", path: "/v4/spreadsheets/sheet_123/values/A1", wantType: "Google Sheet", wantID: "sheet_123"},
		{name: "google slide", service: "slides", path: "/v1/presentations/slide_123/pages", wantType: "Google Slide", wantID: "slide_123"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotID := inferRequestLogTargetObject(tc.service, tc.path, tc.query)
			if gotType != tc.wantType || gotID != tc.wantID {
				t.Fatalf("target = (%q, %q), want (%q, %q)", gotType, gotID, tc.wantType, tc.wantID)
			}
		})
	}
}

func TestRequestLogFilteringPaginationAndProjection(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	now := nowUTC()
	for _, entry := range []RequestLogEntry{
		{RequestID: "req_success", UserID: userID, UserEmail: "owner@example.com", WorkspaceEmail: "workspace@example.com", CreatedAt: now.Add(-10 * time.Minute), Method: http.MethodGet, Service: "gmail", Path: "/gmail", Outcome: "Success", HTTPStatus: http.StatusOK},
		{RequestID: "req_fail", UserID: userID, UserEmail: "owner@example.com", WorkspaceEmail: "workspace@example.com", CreatedAt: now.Add(-5 * time.Minute), Method: http.MethodPost, Service: "agent-skill", Path: "/skill", Outcome: "Fail", HTTPStatus: http.StatusForbidden, ErrorMessage: "agent name is required"},
	} {
		if err := store.SaveRequestLog(&entry); err != nil {
			t.Fatal(err)
		}
	}
	rows, total, err := app.filteredRequestLogRows(userID, "UTC", requestLogAPIQuery{
		Page:     1,
		PageSize: 1,
		Start:    now.Add(-1 * time.Hour),
		End:      now.Add(1 * time.Minute),
		Filters:  map[string]*regexp.Regexp{"outcome": regexp.MustCompile("^Fail$")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0]["request_id"] != "req_fail" {
		t.Fatalf("unexpected filtered rows total=%d rows=%+v", total, rows)
	}
	projected := projectLogRows(rows, []string{"request_id", "outcome"})
	if len(projected[0]) != 2 || projected[0]["request_id"] != "req_fail" || projected[0]["service"] != nil {
		t.Fatalf("unexpected projected rows: %+v", projected)
	}
}

func TestActivityLogRowsConsolidateAllLogTables(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	now := nowUTC()
	if err := store.SaveRequestLog(&RequestLogEntry{
		RequestID:      "req_activity",
		UserID:         userID,
		UserEmail:      "owner@example.com",
		WorkspaceEmail: "workspace@example.com",
		CreatedAt:      now.Add(-5 * time.Minute),
		Method:         http.MethodGet,
		Service:        "gmail",
		Path:           "/gmail",
		Outcome:        "Success",
		HTTPStatus:     http.StatusOK,
	}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		table  string
		action string
		target string
		when   time.Time
		save   func(*AuditLogEntry) error
	}{
		{table: "policy", action: "policy_updated", target: "Policy A", when: now.Add(-4 * time.Minute), save: store.SavePolicyAuditLog},
		{table: "workspace", action: "workspace_friendly_name_updated", target: "Workspace A", when: now.Add(-3 * time.Minute), save: store.SaveWorkspaceAuditLog},
		{table: "drive", action: "drive_folder_added", target: "Reports", when: now.Add(-2 * time.Minute), save: store.SaveDriveFolderAuditLog},
		{table: "user", action: "user_signed_in", target: "owner@example.com", when: now.Add(-1 * time.Minute), save: store.SaveUserAuditLog},
	} {
		if err := tc.save(&AuditLogEntry{
			UserID:         userID,
			UserEmail:      "owner@example.com",
			WorkspaceEmail: "workspace@example.com",
			CreatedAt:      tc.when,
			Action:         tc.action,
			TargetID:       "target_" + tc.table,
			TargetName:     tc.target,
			DetailsJSON:    `{"remote_addr":"127.0.0.1:1000","user_agent":"test-agent"}`,
		}); err != nil {
			t.Fatal(err)
		}
	}
	rows, total, err := app.filteredRequestLogRows(userID, "UTC", requestLogAPIQuery{
		Page:     1,
		PageSize: 50,
		Start:    now.Add(-1 * time.Hour),
		End:      now.Add(1 * time.Minute),
		Filters:  map[string]*regexp.Regexp{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 || len(rows) != 5 {
		t.Fatalf("expected one activity row from each log table, total=%d rows=%+v", total, rows)
	}
	seen := map[string]bool{}
	for _, row := range rows {
		seen[fmt.Sprint(row["log_type"])] = true
	}
	for _, want := range []string{"Request", "Policy audit", "Workspace audit", "Drive folder audit", "User audit"} {
		if !seen[want] {
			t.Fatalf("missing consolidated log type %q in rows %+v", want, rows)
		}
	}
	if rows[0]["log_type"] != "User audit" || rows[0]["target_object_type"] != "User session" {
		t.Fatalf("expected newest user audit first with normalized target, got %+v", rows[0])
	}
	rows, total, err = app.filteredRequestLogRows(userID, "UTC", requestLogAPIQuery{
		Page:     1,
		PageSize: 50,
		Start:    now.Add(-1 * time.Hour),
		End:      now.Add(1 * time.Minute),
		Filters:  map[string]*regexp.Regexp{},
		LogTypes: map[string]bool{"Request": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 || rows[0]["log_type"] != "Request" {
		t.Fatalf("expected log type filtering to keep only request logs, total=%d rows=%+v", total, rows)
	}
}

func TestRequestLogTargetObjectPersistsAndProjects(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	now := nowUTC()
	app.saveRequestLog(RequestLogEntry{
		RequestID:      "req_target",
		UserID:         userID,
		UserEmail:      "owner@example.com",
		WorkspaceEmail: "workspace@example.com",
		CreatedAt:      now,
		Method:         http.MethodGet,
		Service:        "calendar",
		Path:           "/calendar/v3/calendars/primary/events/event_123",
		Query:          "",
		Outcome:        "Success",
		HTTPStatus:     http.StatusOK,
	})
	rows, total, err := app.filteredRequestLogRows(userID, "UTC", requestLogAPIQuery{
		Page:     1,
		PageSize: 50,
		Start:    now.Add(-1 * time.Minute),
		End:      now.Add(1 * time.Minute),
		Filters:  map[string]*regexp.Regexp{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("unexpected target log rows total=%d rows=%+v", total, rows)
	}
	if rows[0]["target_object_type"] != "Calendar event" || rows[0]["target_object_id"] != "primary/event_123" || rows[0]["target_name"] != "primary/event_123" {
		t.Fatalf("unexpected projected target fields: %+v", rows[0])
	}
}

func TestAuditLogRowMapNormalizesAgentAuditTargetKinds(t *testing.T) {
	row := auditLogRowMap(AuditLogEntry{
		Action:     "agent_firewall_address_added",
		TargetID:   "agt_test",
		TargetName: "Jim",
	}, "UTC", "User audit", "user", "User account", "Success")
	if row["service"] != "firewall" {
		t.Fatalf("expected firewall service label, got %+v", row)
	}
	if row["target_object_type"] != "Agent firewall address" {
		t.Fatalf("expected normalized firewall target type, got %+v", row)
	}

	row = auditLogRowMap(AuditLogEntry{
		Action:     "agent_profile_updated",
		TargetID:   "agt_test",
		TargetName: "Jim",
	}, "UTC", "User audit", "user", "User account", "Success")
	if row["service"] != "agent" {
		t.Fatalf("expected agent service label, got %+v", row)
	}
	if row["target_object_type"] != "Agent profile" {
		t.Fatalf("expected normalized agent profile target type, got %+v", row)
	}
}

func TestRequestLogsAPIUsesTabulatorPaginationEnvelope(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	app := NewApp(&Config{
		BaseURL:           "http://localhost:8080",
		SessionCookieName: "session",
		EncryptionKey:     []byte("12345678901234567890123456789012"),
	}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	sessionToken := "pst_test_session"
	if err := store.CreateSession(userID, SHA256Hex(sessionToken), nowUTC().Add(1*time.Hour)); err != nil {
		t.Fatal(err)
	}
	now := nowUTC()
	createdAt := now.Add(-5 * time.Minute)
	if err := store.SaveRequestLog(&RequestLogEntry{
		RequestID:      "req_api",
		UserID:         userID,
		UserEmail:      "owner@example.com",
		WorkspaceEmail: "workspace@example.com",
		CreatedAt:      createdAt,
		Method:         http.MethodGet,
		Service:        "gmail",
		Path:           "/gmail",
		Outcome:        "Success",
		HTTPStatus:     http.StatusOK,
	}); err != nil {
		t.Fatal(err)
	}
	q := url.Values{}
	q.Set("page", "1")
	q.Set("page_size", "50")
	q.Set("columns", "request_id,outcome")
	q.Set("start", now.Add(-1*time.Hour).Format(time.RFC3339Nano))
	q.Set("end", now.Add(1*time.Minute).Format(time.RFC3339Nano))
	req := httptest.NewRequest(http.MethodGet, "/api/logs/request?"+q.Encode(), nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("request logs API returned HTTP %d: %s", resp.Code, resp.Body.String())
	}
	var payload struct {
		Data     []map[string]any `json:"data"`
		LastPage int              `json:"last_page"`
		Total    int              `json:"total"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.LastPage != 1 || payload.Total != 1 || len(payload.Data) != 1 || payload.Data[0]["request_id"] != "req_api" {
		t.Fatalf("unexpected Tabulator payload: %+v", payload)
	}
}

func TestRequestLogsAPIDateFiltersUseUserTimezone(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	if err := store.SaveUserTimezone(userID, "UTC"); err != nil {
		t.Fatal(err)
	}
	settings, err := store.GetUserSettings(userID)
	if err != nil {
		t.Fatal(err)
	}
	loggingSettings, err := store.GetUserLoggingSettings(userID)
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	startBound, endBound := logRetentionBounds(settings.Timezone, loggingSettings.RetentionDays)
	query, err := app.parseRequestLogAPIQuery(url.Values{
		"start_date": {formatLogDateKey(startBound, settings.Timezone)},
		"end_date":   {formatLogDateKey(endBound, settings.Timezone)},
		"columns":    {"request_id,outcome"},
	}, &User{ID: userID}, settings, loggingSettings, false)
	if err != nil {
		t.Fatal(err)
	}
	if !query.Start.Equal(startBound) {
		t.Fatalf("date-only start used wrong timezone: got %s want %s", query.Start, startBound)
	}
	if query.End.After(endBound.Add(1 * time.Second)) {
		t.Fatalf("date-only end exceeded server bound: got %s want <= %s", query.End, endBound)
	}
}

func TestRequestLogExportAllUsesDateRangeAllColumnsAndIgnoresFilters(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	if err := store.SaveUserTimezone(userID, "UTC"); err != nil {
		t.Fatal(err)
	}
	app := NewApp(&Config{
		BaseURL:           "http://localhost:8080",
		SessionCookieName: "session",
		EncryptionKey:     []byte("12345678901234567890123456789012"),
	}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	sessionToken := "pst_export_all"
	if err := store.CreateSession(userID, SHA256Hex(sessionToken), nowUTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	now := nowUTC()
	for _, entry := range []RequestLogEntry{
		{RequestID: "req_before_range", UserID: userID, CreatedAt: now.Add(-6 * time.Hour), Outcome: "Success", HTTPStatus: http.StatusOK},
		{RequestID: "req_in_range_success", UserID: userID, CreatedAt: now.Add(-30 * time.Minute), Outcome: "Success", HTTPStatus: http.StatusOK},
		{RequestID: "req_in_range_fail", UserID: userID, CreatedAt: now.Add(-20 * time.Minute), Outcome: "Fail", HTTPStatus: http.StatusForbidden},
	} {
		entry.UserEmail = "owner@example.com"
		entry.WorkspaceEmail = "workspace@example.com"
		entry.Method = http.MethodGet
		entry.Service = "gmail"
		entry.Path = "/gmail"
		if err := store.SaveRequestLog(&entry); err != nil {
			t.Fatal(err)
		}
	}
	q := url.Values{}
	q.Set("scope", "all")
	q.Set("format", "jsonl")
	q.Set("columns", "request_id")
	q.Set("filter_outcome", "^NoMatch$")
	q.Set("start", now.Add(-1*time.Hour).Format(time.RFC3339Nano))
	q.Set("end", now.Add(1*time.Minute).Format(time.RFC3339Nano))
	req := httptest.NewRequest(http.MethodGet, "/api/logs/request/export?"+q.Encode(), nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("export all returned HTTP %d: %s", resp.Code, resp.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(resp.Body.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("export all should include all rows in selected range and ignore regex filters, got %q", resp.Body.String())
	}
	var payload []map[string]any
	for _, line := range lines {
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		payload = append(payload, row)
	}
	if len(payload[0]) != len(allRequestLogColumnIDs()) {
		t.Fatalf("export all should include every column, got %d columns", len(payload[0]))
	}
	if _, ok := payload[0]["workspace_email"]; !ok {
		t.Fatalf("export all should project all columns, got first row %+v", payload[0])
	}
	for _, row := range payload {
		if row["request_id"] == "req_before_range" {
			t.Fatalf("export all should honor selected date range, got %+v", payload)
		}
	}

	q.Set("format", "csv")
	req = httptest.NewRequest(http.MethodGet, "/api/logs/request/export?"+q.Encode(), nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp = httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("export all CSV returned HTTP %d: %s", resp.Code, resp.Body.String())
	}
	records, err := csv.NewReader(strings.NewReader(resp.Body.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("export all CSV should include all rows in selected range and ignore regex filters, got %+v", records)
	}
	if len(records[0]) != len(allRequestLogColumnIDs()) {
		t.Fatalf("export all CSV should include every column, got %d columns", len(records[0]))
	}
	for _, record := range records[1:] {
		if len(record) > 2 && record[2] == "req_before_range" {
			t.Fatalf("export all CSV should honor selected date range, got %+v", records)
		}
	}
}

func TestRequestLogExportViewUsesSelectedColumnsFiltersAndDateRange(t *testing.T) {
	store, userID := newLoggingTestStore(t)
	if err := store.SaveUserTimezone(userID, "UTC"); err != nil {
		t.Fatal(err)
	}
	app := NewApp(&Config{
		BaseURL:           "http://localhost:8080",
		SessionCookieName: "session",
		EncryptionKey:     []byte("12345678901234567890123456789012"),
	}, store, NewCrypto([]byte("12345678901234567890123456789012")))
	sessionToken := "pst_export_view"
	if err := store.CreateSession(userID, SHA256Hex(sessionToken), nowUTC().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	now := nowUTC()
	for _, entry := range []RequestLogEntry{
		{RequestID: "req_before_range", UserID: userID, CreatedAt: now.Add(-6 * time.Hour), Outcome: "Success", HTTPStatus: http.StatusOK},
		{RequestID: "req_in_range_success", UserID: userID, CreatedAt: now.Add(-30 * time.Minute), Outcome: "Success", HTTPStatus: http.StatusOK},
		{RequestID: "req_in_range_fail", UserID: userID, CreatedAt: now.Add(-20 * time.Minute), Outcome: "Fail", HTTPStatus: http.StatusForbidden},
	} {
		entry.UserEmail = "owner@example.com"
		entry.WorkspaceEmail = "workspace@example.com"
		entry.Method = http.MethodGet
		entry.Service = "gmail"
		entry.Path = "/gmail"
		if err := store.SaveRequestLog(&entry); err != nil {
			t.Fatal(err)
		}
	}
	q := url.Values{}
	q.Set("scope", "current")
	q.Set("format", "jsonl")
	q.Set("columns", "request_id,outcome")
	q.Set("filter_outcome", "^Success$")
	q.Set("start", now.Add(-1*time.Hour).Format(time.RFC3339Nano))
	q.Set("end", now.Add(1*time.Minute).Format(time.RFC3339Nano))
	req := httptest.NewRequest(http.MethodGet, "/api/logs/request/export?"+q.Encode(), nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp := httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("export view returned HTTP %d: %s", resp.Code, resp.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(resp.Body.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("export view should honor selected date range and regex filters, got %q", resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["request_id"] != "req_in_range_success" {
		t.Fatalf("export view should honor selected date range and regex filters, got %+v", payload)
	}
	if len(payload) != 2 {
		t.Fatalf("export view should use selected columns only, got %+v", payload)
	}
	if _, ok := payload["workspace_email"]; ok {
		t.Fatalf("export view should not include unselected columns, got %+v", payload)
	}

	q.Set("format", "csv")
	req = httptest.NewRequest(http.MethodGet, "/api/logs/request/export?"+q.Encode(), nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	resp = httptest.NewRecorder()
	app.Routes().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("export view CSV returned HTTP %d: %s", resp.Code, resp.Body.String())
	}
	records, err := csv.NewReader(strings.NewReader(resp.Body.String())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || strings.Join(records[0], ",") != "Request ID,Outcome" || strings.Join(records[1], ",") != "req_in_range_success,Success" {
		t.Fatalf("export view CSV should honor selected columns, filters, and date range, got %+v", records)
	}
}

func TestCSVSafePrefixesFormulaValues(t *testing.T) {
	if got := csvSafe("=cmd"); got != "'=cmd" {
		t.Fatalf("expected formula value to be prefixed, got %q", got)
	}
	if got := csvSafe("normal"); got != "normal" {
		t.Fatalf("expected normal value to be unchanged, got %q", got)
	}
}

func newLoggingTestStore(t *testing.T) (*Store, string) {
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
	userID := "usr_logging"
	if err := store.db.Exec(`INSERT INTO users (id, email, name, picture, is_admin, is_suspended, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?);`, userID, "owner@example.com", "Owner", "", 0, 0, nowUTC(), nowUTC()); err != nil {
		t.Fatal(err)
	}
	return store, userID
}
