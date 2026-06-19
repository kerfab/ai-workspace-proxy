// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminUserRoutesAreScopedToAdministratorOrganization(t *testing.T) {
	app, store, admin, sessionToken := newOrganizationAdminTestApp(t)
	admin, err := store.CreateOrUpdateUser(admin.Email, admin.Name, admin.Picture, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkOrganizationAdminVerificationVerified(admin.ID, nowUTC()); err != nil {
		t.Fatal(err)
	}
	foreignUser, err := store.CreateOrUpdateUser("member@other-example.com", "Foreign Member", "", false)
	if err != nil {
		t.Fatal(err)
	}
	resp := organizationAdminGetRequest(t, app, sessionToken, "/admin/users/"+foreignUser.ID)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNotFound)
	}
}

func TestAdministratorCannotSuspendOrDeleteSelf(t *testing.T) {
	app, store, admin, sessionToken := newOrganizationAdminTestApp(t)
	admin, err := store.CreateOrUpdateUser(admin.Email, admin.Name, admin.Picture, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkOrganizationAdminVerificationVerified(admin.ID, nowUTC()); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		path   string
		action string
	}{
		{name: "suspend self", path: "/admin/users/" + admin.ID + "/suspend", action: "suspend"},
		{name: "delete self", path: "/admin/users/" + admin.ID + "/delete", action: "delete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(""))
			req.Header.Set("Accept", "application/json")
			req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
			resp := httptest.NewRecorder()
			app.Routes().ServeHTTP(resp, req)
			if resp.Code != http.StatusForbidden {
				t.Fatalf("%s status = %d, want %d", tc.action, resp.Code, http.StatusForbidden)
			}
		})
	}

	stored, err := store.FindUserByID(admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil {
		t.Fatal("administrator account should still exist")
	}
	if stored.IsSuspended {
		t.Fatal("administrator account should not be suspended")
	}
}
