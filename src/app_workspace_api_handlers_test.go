package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestUserWorkspacesAPIRequiresBackendKeyAndReturnsPolicy(t *testing.T) {
	app, _, standardToken, backendToken := newPolicyAPITestApp(t)

	standardResp := policyAPIRequest(t, app, http.MethodGet, "/api/user/workspaces?workspace=workspace", standardToken, "")
	if standardResp.Code != http.StatusUnauthorized {
		t.Fatalf("expected standard agent key to be rejected with 401, got %d: %s", standardResp.Code, standardResp.Body.String())
	}

	backendResp := policyAPIRequest(t, app, http.MethodGet, "/api/user/workspaces?workspace=workspace", backendToken, "")
	if backendResp.Code != http.StatusOK {
		t.Fatalf("workspace lookup failed with %d: %s", backendResp.Code, backendResp.Body.String())
	}
	if got := backendResp.Body.String(); !containsAll(got, `"email":"workspace@example.com"`, `"policy_id":"system"`) {
		t.Fatalf("unexpected workspace payload: %s", got)
	}
}

func TestUserDriveFoldersAPIRequiresBackendKeyAndListsRefs(t *testing.T) {
	app, store, standardToken, backendToken := newPolicyAPITestApp(t)
	if err := store.CreateDriveFolderRef(&AllowedDriveFolder{
		UserID:          "usr_test",
		MailboxEmail:    "workspace@example.com",
		ReferenceName:   "Test Drive",
		ReferenceKey:    "test drive",
		FolderURL:       "https://drive.google.com/drive/folders/folder_test",
		FolderID:        "folder_test",
		FolderName:      "Live Test Folder",
		AllowDocs:       true,
		AllowDriveFiles: true,
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
	if got := backendResp.Body.String(); !containsAll(got, `"reference_name":"Test Drive"`, `"folder_name":"Live Test Folder"`, `"allow_docs":true`, `"allow_drive_files":true`) {
		t.Fatalf("unexpected Drive folders payload: %s", got)
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
