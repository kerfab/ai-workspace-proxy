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

	app := NewApp(&Config{BaseURL: "http://localhost:8080"}, store, NewCrypto([]byte("12345678901234567890123456789012")), nil)
	config, err := app.agentWorkspaceAPIConfig(user.ID, "ptk_test", "openclaw")
	if err != nil {
		t.Fatal(err)
	}
	if config["config_type"] != "agents_workspace_api_access" {
		t.Fatalf("unexpected config_type: %v", config["config_type"])
	}
	if config["proxy_url"] != "http://localhost:8080" {
		t.Fatalf("unexpected proxy_url: %v", config["proxy_url"])
	}
	if config["proxy_token"] != "ptk_test" {
		t.Fatalf("unexpected proxy_token: %v", config["proxy_token"])
	}
	if config["skill_platform"] != "openclaw" {
		t.Fatalf("unexpected skill_platform: %v", config["skill_platform"])
	}

	workspaces := config["workspaces"].([]map[string]string)
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
	if workspaces[0]["email"] != "alpha@example.com" || workspaces[1]["email"] != "zeta@example.com" {
		t.Fatalf("workspaces are not sorted by email: %v", workspaces)
	}
}
