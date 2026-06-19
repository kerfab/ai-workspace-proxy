// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAgentWorkspaceAPIConfigUsesRenamedFileAndSortedWorkspaces(t *testing.T) {
	if agentWorkspaceAPIConfigPath != "config/agents-workspace-api-access.config.json" {
		t.Fatalf("unexpected agent config path: %s", agentWorkspaceAPIConfigPath)
	}

	db, err := OpenSQLite(filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	user, err := store.CreateOrUpdateUser("owner@example.com", "Owner", "", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, conn := range []GmailConnection{
		{UserID: user.ID, MailboxEmail: "zeta@example.com", FriendlyName: "zeta", RefreshTokenEnc: "enc", TokenExpiry: time.Now()},
		{UserID: user.ID, MailboxEmail: "alpha@example.com", FriendlyName: "alpha", RefreshTokenEnc: "enc", TokenExpiry: time.Now()},
	} {
		if err := store.SaveGmailConnection(&conn); err != nil {
			t.Fatal(err)
		}
	}
	crypto := NewCrypto([]byte("12345678901234567890123456789012"))
	enc, err := crypto.Encrypt("atk_test")
	if err != nil {
		t.Fatal(err)
	}
	agent := &AgentAccess{
		ID:              "agt_config",
		UserID:          user.ID,
		FriendlyName:    "Config agent",
		DefaultLocation: "test suite",
		TokenEnc:        enc,
		TokenHint:       "atk_test...",
		Enabled:         true,
	}
	if err := store.SaveAgent(agent); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAgentWorkspaceGrants(user.ID, agent.ID, []AgentWorkspaceGrant{
		{AgentID: agent.ID, UserID: user.ID, MailboxEmail: "zeta@example.com", PolicyID: systemPolicyID, RequireAgentMotive: true},
		{AgentID: agent.ID, UserID: user.ID, MailboxEmail: "alpha@example.com", PolicyID: systemPolicyID},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateDriveFolderRef(&AllowedDriveFolder{
		ID:            "dfr_config_alpha",
		UserID:        user.ID,
		MailboxEmail:  "alpha@example.com",
		ReferenceName: "Reports",
		ReferenceKey:  "reports",
		FolderURL:     "https://drive.google.com/drive/folders/reports",
		FolderID:      "reports",
		FolderName:    "Reports Real Name",
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAgentDriveFolderGrants(user.ID, agent.ID, "alpha@example.com", []string{"dfr_config_alpha"}); err != nil {
		t.Fatal(err)
	}

	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, crypto)
	config, err := app.agentWorkspaceAPIConfig(user.ID, agent.ID, "atk_test", "openclaw")
	if err != nil {
		t.Fatal(err)
	}
	if config["proxy_url"] != "http://localhost:8080" {
		t.Fatalf("unexpected proxy_url: %v", config["proxy_url"])
	}
	if config["agent_api_token"] != "atk_test" {
		t.Fatalf("unexpected agent_api_token: %v", config["agent_api_token"])
	}
	for _, key := range []string{"config_type", "agent_id", "agent_name", "agent_location", "skill_platform"} {
		if _, ok := config[key]; ok {
			t.Fatalf("agent config should not include %s: %v", key, config)
		}
	}

	workspaces := config["workspaces"].([]map[string]any)
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
	if workspaces[0]["email"] != "alpha@example.com" || workspaces[1]["email"] != "zeta@example.com" {
		t.Fatalf("workspaces are not sorted by email: %v", workspaces)
	}
	if workspaces[0]["require_agent_motive"] != false || workspaces[1]["require_agent_motive"] != true {
		t.Fatalf("unexpected workspace motive requirements: %v", workspaces)
	}
	folders := workspaces[0]["allowed_drive_folders"].([]map[string]any)
	if len(folders) != 1 || folders[0]["reference_name"] != "Reports" || folders[0]["folder_name"] != "Reports Real Name" {
		t.Fatalf("unexpected allowed Drive folders in config: %v", folders)
	}
	if folders := workspaces[1]["allowed_drive_folders"].([]map[string]any); len(folders) != 0 {
		t.Fatalf("workspace without Drive folder grants should include an empty list, got %v", folders)
	}
}

func TestTokenHintCanLimitVisibleSuffix(t *testing.T) {
	if got := tokenHint("atk_3242abcdef39af5e", 7, 3); got != "atk_324...f5e" {
		t.Fatalf("unexpected short token hint: %s", got)
	}
	if got := agentTokenHint("atk_3242...39af5e"); got != "atk_324...f5e" {
		t.Fatalf("unexpected normalized agent token hint: %s", got)
	}
	if got := tokenHint("ubk_3242abcdef39af5e", 8, 6); got != "ubk_3242...39af5e" {
		t.Fatalf("unexpected backend token hint: %s", got)
	}
	if got := tokenHint("short", 8, 3); got != "short" {
		t.Fatalf("short token should be unchanged, got: %s", got)
	}
}
