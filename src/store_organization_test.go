// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"path/filepath"
	"testing"
)

func newOrganizationTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "test.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := NewStore(db)
	store.SetOrganizationAdminVerificationKey([]byte("12345678901234567890123456789012"))
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestCreateOrUpdateUserAssignsOrganizationsByDomain(t *testing.T) {
	store := newOrganizationTestStore(t)

	alice, err := store.CreateOrUpdateUser("alice@example.com", "Alice", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if alice.OrganizationID == "" {
		t.Fatal("expected organization id for organization user")
	}

	bob, err := store.CreateOrUpdateUser("bob@example.com", "Bob", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if bob.OrganizationID != alice.OrganizationID {
		t.Fatalf("same-domain users should share organization id, got %q and %q", alice.OrganizationID, bob.OrganizationID)
	}

	gmailUser, err := store.CreateOrUpdateUser("person@gmail.com", "Person", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if gmailUser.OrganizationID != "" {
		t.Fatalf("gmail user should not be linked to an organization, got %q", gmailUser.OrganizationID)
	}

	org, err := store.FindOrganizationByID(alice.OrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if org == nil || org.Name != "example.com" {
		t.Fatalf("organization = %+v, want name example.com", org)
	}

	orgByDomain, err := store.FindOrganizationByDomain("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if orgByDomain == nil || orgByDomain.ID != alice.OrganizationID {
		t.Fatalf("organization by domain = %+v, want id %q", orgByDomain, alice.OrganizationID)
	}

	row, err := store.db.QueryOne(`SELECT COUNT(*) AS cnt FROM organization_domains WHERE organization_id = ?;`, alice.OrganizationID)
	if err != nil {
		t.Fatal(err)
	}
	if got := atoiSafe(row["cnt"]); got != 1 {
		t.Fatalf("organization_domains count = %d, want 1", got)
	}
}

func TestStoreInitBackfillsLegacyUserOrganizations(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "legacy.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		picture TEXT NOT NULL,
		is_admin INTEGER NOT NULL DEFAULT 0,
		is_suspended INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`); err != nil {
		t.Fatal(err)
	}
	now := nowUTC()
	if err := db.Exec(`INSERT INTO users (id, email, name, picture, is_admin, is_suspended, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 0, ?, ?);`, "usr_legacy_org", "legacy@example.com", "Legacy Org", "", now, now); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO users (id, email, name, picture, is_admin, is_suspended, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, 0, ?, ?);`, "usr_legacy_gmail", "legacy@gmail.com", "Legacy Gmail", "", now, now); err != nil {
		t.Fatal(err)
	}

	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}

	orgUser, err := store.FindUserByEmail("legacy@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if orgUser == nil || orgUser.OrganizationID == "" {
		t.Fatalf("expected backfilled organization user, got %+v", orgUser)
	}

	gmailUser, err := store.FindUserByEmail("legacy@gmail.com")
	if err != nil {
		t.Fatal(err)
	}
	if gmailUser == nil {
		t.Fatal("expected gmail user")
	}
	if gmailUser.OrganizationID != "" {
		t.Fatalf("gmail user should keep null organization id, got %q", gmailUser.OrganizationID)
	}

	org, err := store.FindOrganizationByDomain("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if org == nil || org.ID != orgUser.OrganizationID {
		t.Fatalf("organization by domain = %+v, want id %q", org, orgUser.OrganizationID)
	}
}

func TestSetUserSuspendedByTracksSuspensionMetadata(t *testing.T) {
	store := newOrganizationTestStore(t)

	admin, err := store.CreateOrUpdateUser("admin@example.com", "Admin User", "", true)
	if err != nil {
		t.Fatal(err)
	}
	member, err := store.CreateOrUpdateUser("member@example.com", "Member User", "", false)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.SetUserSuspendedBy(member.ID, true, admin.ID); err != nil {
		t.Fatal(err)
	}
	member, err = store.FindUserByID(member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if member == nil {
		t.Fatal("expected member user")
	}
	if !member.IsSuspended {
		t.Fatal("expected suspended member")
	}
	if member.SuspendedAt.IsZero() {
		t.Fatal("expected suspension timestamp")
	}
	if member.SuspendedByUserID != admin.ID {
		t.Fatalf("suspended by = %q, want %q", member.SuspendedByUserID, admin.ID)
	}

	if err := store.SetUserSuspendedBy(member.ID, false, admin.ID); err != nil {
		t.Fatal(err)
	}
	member, err = store.FindUserByID(member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if member == nil {
		t.Fatal("expected member user after unsuspend")
	}
	if member.IsSuspended {
		t.Fatal("expected unsuspended member")
	}
	if !member.SuspendedAt.IsZero() {
		t.Fatalf("suspension timestamp should be cleared, got %v", member.SuspendedAt)
	}
	if member.SuspendedByUserID != "" {
		t.Fatalf("suspended by should be cleared, got %q", member.SuspendedByUserID)
	}
}
