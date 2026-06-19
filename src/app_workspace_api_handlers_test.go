// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestUserWorkspacesAPIRequiresBackendKeyAndReturnsWorkspace(t *testing.T) {
	app, _, standardToken, backendToken := newPolicyAPITestApp(t)

	standardResp := policyAPIRequest(t, app, http.MethodGet, "/api/user/workspaces?workspace=workspace", standardToken, "")
	if standardResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected standard agent key to be rejected with 401, got %d: %s", standardResp.Code, standardResp.Body.String())
	}

	backendResp := policyAPIRequest(t, app, http.MethodGet, "/api/user/workspaces?workspace=workspace", backendToken, "")
	if backendResp.Code != http.StatusOK {
		t.Fatalf("workspace lookup failed with %d: %s", backendResp.Code, backendResp.Body.String())
	}
	if got := backendResp.Body.String(); !containsAll(got, `"email":"workspace@example.com"`, `"name":"workspace"`) {
		t.Fatalf("unexpected workspace payload: %s", got)
	} else if strings.Contains(got, `"policy_id"`) {
		t.Fatalf("workspace payload should not expose policy fields: %s", got)
	}
}

func TestUserDriveFoldersAPIRequiresBackendKeyAndListsRefs(t *testing.T) {
	app, store, standardToken, backendToken := newPolicyAPITestApp(t)
	if err := store.CreateDriveFolderRef(&AllowedDriveFolder{
		UserID:        "usr_test",
		MailboxEmail:  "workspace@example.com",
		ReferenceName: "Test Drive",
		ReferenceKey:  "test drive",
		FolderURL:     "https://drive.google.com/drive/folders/folder_test",
		FolderID:      "folder_test",
		FolderName:    "Live Test Folder",
	}); err != nil {
		t.Fatal(err)
	}

	standardResp := policyAPIRequest(t, app, http.MethodGet, "/api/user/drive-folders?workspace=workspace", standardToken, "")
	if standardResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected standard agent key to be rejected with 401, got %d: %s", standardResp.Code, standardResp.Body.String())
	}

	backendResp := policyAPIRequest(t, app, http.MethodGet, "/api/user/drive-folders?workspace=workspace", backendToken, "")
	if backendResp.Code != http.StatusOK {
		t.Fatalf("Drive folder lookup failed with %d: %s", backendResp.Code, backendResp.Body.String())
	}
	if got := backendResp.Body.String(); !containsAll(got, `"reference_name":"Test Drive"`, `"folder_name":"Live Test Folder"`) {
		t.Fatalf("unexpected Drive folders payload: %s", got)
	} else if containsAny(got, `"allow_docs"`, `"allow_sheets"`, `"allow_slides"`, `"allow_drive_files"`, `"allowed_types"`) {
		t.Fatalf("Drive folders payload should not expose removed type filters: %s", got)
	}
}

func TestDriveFolderRefsRejectDuplicateFolderIDsCaseSensitive(t *testing.T) {
	_, store, _, _ := newPolicyAPITestApp(t)
	first := &AllowedDriveFolder{
		ID:            "dfr_first",
		UserID:        "usr_test",
		MailboxEmail:  "workspace@example.com",
		ReferenceName: "Reports",
		ReferenceKey:  "reports",
		FolderURL:     "https://drive.google.com/drive/folders/FoLdEr_123?usp=drive_link",
		FolderID:      "FoLdEr_123",
		FolderName:    "Reports",
	}
	if err := store.CreateDriveFolderRef(first); err != nil {
		t.Fatal(err)
	}
	existing, err := store.FindDriveFolderRefByFolderID("usr_test", "workspace@example.com", "folder_123")
	if err != nil {
		t.Fatal(err)
	}
	if existing != nil {
		t.Fatalf("Google Drive folder IDs are case-sensitive; lower-case lookup should not match first ref, got %+v", existing)
	}
	if err := store.CreateDriveFolderRef(&AllowedDriveFolder{
		ID:            "dfr_second",
		UserID:        "usr_test",
		MailboxEmail:  "workspace@example.com",
		ReferenceName: "Reports Copy",
		ReferenceKey:  "reports copy",
		FolderURL:     "https://drive.google.com/drive/folders/folder_123?resourcekey=ignored",
		FolderID:      "folder_123",
		FolderName:    "Reports",
	}); err != nil {
		t.Fatalf("same letters with different case should be allowed as a distinct Drive ID: %v", err)
	}
	err = store.CreateDriveFolderRef(&AllowedDriveFolder{
		ID:            "dfr_duplicate",
		UserID:        "usr_test",
		MailboxEmail:  "workspace@example.com",
		ReferenceName: "Reports Exact Copy",
		ReferenceKey:  "reports exact copy",
		FolderURL:     "https://drive.google.com/drive/folders/folder_123?usp=sharing",
		FolderID:      "folder_123",
		FolderName:    "Reports",
	})
	if err == nil {
		t.Fatal("expected exact duplicate folder ID insert to fail")
	}
}

func TestParseDriveFolderLinkIgnoresQueryParametersAndBaseURLCase(t *testing.T) {
	folderID, _, err := parseDriveFolderLink("HTTPS://DRIVE.GOOGLE.COM/drive/folders/FoLdEr_123?usp=drive_link")
	if err != nil {
		t.Fatal(err)
	}
	if folderID != "FoLdEr_123" {
		t.Fatalf("folderID = %q, want original-cased folder ID", folderID)
	}
}

func TestUserAgentGrantsAPIRequiresBackendKeyAndUpdatesGrantPolicy(t *testing.T) {
	app, store, standardToken, backendToken := newPolicyAPITestApp(t)
	policy := &UserPolicy{
		ID:                  "pol_agent_grant",
		UserID:              "usr_test",
		Name:                "Agent Grant Policy",
		EnabledCapabilities: []string{"gmail_messages_read"},
	}
	if err := store.SaveUserPolicy(policy); err != nil {
		t.Fatal(err)
	}

	standardResp := policyAPIRequest(t, app, http.MethodGet, "/api/user/agents/agt_test_standard/grants", standardToken, "")
	if standardResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected standard agent key to be rejected with 401, got %d: %s", standardResp.Code, standardResp.Body.String())
	}

	updateResp := policyAPIRequest(t, app, http.MethodPut, "/api/user/agents/agt_test_standard/grants", backendToken, `{
		"workspace": "workspace@example.com",
		"policy_id": "pol_agent_grant",
		"require_agent_motive": true
	}`)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("agent grant update failed with %d: %s", updateResp.Code, updateResp.Body.String())
	}
	if got := updateResp.Body.String(); !containsAll(got, `"agent_id":"agt_test_standard"`, `"workspace_email":"workspace@example.com"`, `"policy_id":"pol_agent_grant"`, `"policy_name":"Agent Grant Policy"`, `"require_agent_motive":true`) {
		t.Fatalf("unexpected agent grant update payload: %s", got)
	}

	grant, err := store.GetAgentWorkspaceGrant("usr_test", "agt_test_standard", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if grant == nil || grant.PolicyID != "pol_agent_grant" || !grant.RequireAgentMotive {
		t.Fatalf("saved grant = %+v, want policy pol_agent_grant", grant)
	}

	getResp := policyAPIRequest(t, app, http.MethodGet, "/api/user/agents/agt_test_standard/grants?workspace=workspace", backendToken, "")
	if getResp.Code != http.StatusOK {
		t.Fatalf("agent grant lookup failed with %d: %s", getResp.Code, getResp.Body.String())
	}
	if got := getResp.Body.String(); !containsAll(got, `"workspace_email":"workspace@example.com"`, `"policy_id":"pol_agent_grant"`, `"require_agent_motive":true`) {
		t.Fatalf("unexpected agent grant lookup payload: %s", got)
	}
}

func TestUserAgentDriveFolderGrantsAPIRequiresBackendKeyAndUpdatesFolders(t *testing.T) {
	app, store, standardToken, backendToken := newPolicyAPITestApp(t)
	for _, folder := range []*AllowedDriveFolder{
		{
			ID:            "dfr_api_reports",
			UserID:        "usr_test",
			MailboxEmail:  "workspace@example.com",
			ReferenceName: "Reports",
			ReferenceKey:  "reports",
			FolderURL:     "https://drive.google.com/drive/folders/reports",
			FolderID:      "reports",
			FolderName:    "Reports Real Name",
		},
		{
			ID:            "dfr_api_archive",
			UserID:        "usr_test",
			MailboxEmail:  "workspace@example.com",
			ReferenceName: "Archive",
			ReferenceKey:  "archive",
			FolderURL:     "https://drive.google.com/drive/folders/archive",
			FolderID:      "archive",
			FolderName:    "Archive Real Name",
		},
	} {
		if err := store.CreateDriveFolderRef(folder); err != nil {
			t.Fatal(err)
		}
	}

	standardResp := policyAPIRequest(t, app, http.MethodPut, "/api/user/agents/agt_test_standard/grants/drive-folders", standardToken, `{
		"workspace": "workspace@example.com",
		"folder_ref_ids": ["dfr_api_reports"]
	}`)
	if standardResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected standard agent key to be rejected with 401, got %d: %s", standardResp.Code, standardResp.Body.String())
	}

	updateResp := policyAPIRequest(t, app, http.MethodPut, "/api/user/agents/agt_test_standard/grants/drive-folders", backendToken, `{
		"workspace": "workspace@example.com",
		"folder_ref_ids": ["dfr_api_reports"]
	}`)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("agent Drive folder grants update failed with %d: %s", updateResp.Code, updateResp.Body.String())
	}
	if got := updateResp.Body.String(); !containsAll(got,
		`"allowed_drive_folders":[`,
		`"id":"dfr_api_reports"`,
		`"reference_name":"Reports"`,
		`"allowed":true`,
		`"id":"dfr_api_archive"`,
		`"allowed":false`,
	) {
		t.Fatalf("unexpected agent Drive folder grants update payload: %s", got)
	}
	allowed, err := store.ListAgentAllowedDriveFolderRefs("usr_test", "agt_test_standard", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(allowed) != 1 || allowed[0].ID != "dfr_api_reports" {
		t.Fatalf("unexpected saved API folder grants: %+v", allowed)
	}
	workspaceGrant, err := store.GetAgentWorkspaceGrant("usr_test", "agt_test_standard", "workspace@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if workspaceGrant != nil {
		t.Fatalf("Drive folder grant API should not implicitly allow Workspace access: %+v", workspaceGrant)
	}
	auditLogs, err := store.ListAuditLogs("user_audit_logs", "usr_test", 10)
	if err != nil {
		t.Fatal(err)
	}
	foundAudit := false
	for _, entry := range auditLogs {
		if entry.Action == "agent_drive_folder_grants_updated" && strings.Contains(entry.DetailsJSON, `"source":"api"`) && strings.Contains(entry.DetailsJSON, `"workspace":"workspace@example.com"`) {
			foundAudit = true
			break
		}
	}
	if !foundAudit {
		t.Fatalf("expected API Drive folder grant update to be logged, got %+v", auditLogs)
	}

	getResp := policyAPIRequest(t, app, http.MethodGet, "/api/user/agents/agt_test_standard/grants/drive-folders?workspace=workspace", backendToken, "")
	if getResp.Code != http.StatusOK {
		t.Fatalf("agent Drive folder grants lookup failed with %d: %s", getResp.Code, getResp.Body.String())
	}
	if got := getResp.Body.String(); !containsAll(got, `"allowed_drive_folders":[`, `"drive_folders":[`, `"id":"dfr_api_reports"`, `"id":"dfr_api_archive"`) {
		t.Fatalf("unexpected agent Drive folder grants lookup payload: %s", got)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}

func containsAny(value string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(value, part) {
			return true
		}
	}
	return false
}
