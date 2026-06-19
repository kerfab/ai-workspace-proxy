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
	"strings"
	"testing"
)

func renderedWithAppBehaviorForTest(t *testing.T, html string) string {
	t.Helper()
	js, err := staticAssets.ReadFile("static/app.js")
	if err != nil {
		t.Fatalf("read static app.js: %v", err)
	}
	return html + "\n" + string(js)
}

func TestActiveSectionAndPageTitles(t *testing.T) {
	tests := []struct {
		name              string
		path              string
		selectedWorkspace string
		section           string
		title             string
	}{
		{name: "dashboard", path: "/", section: "dashboard", title: "Dashboard"},
		{name: "admin canonical", path: "/admin/", section: "admin", title: "Admin"},
		{name: "org-admin alias", path: "/org-admin", section: "admin", title: "Admin"},
		{name: "admin users", path: "/admin/users", section: "admin_users", title: "Manage accounts"},
		{name: "admin user detail", path: "/admin/users/usr_test", section: "admin_user", title: "Manage account"},
		{name: "admin accounts policy", path: "/admin/accounts-policy", section: "admin_accounts_policy", title: "Accounts Policy"},
		{name: "settings", path: "/settings", section: "settings", title: "Settings"},
		{name: "policies", path: "/policies", section: "policies", title: "Permissions editor"},
		{name: "logs", path: "/logs", section: "logs", title: "Logs"},
		{name: "agents", path: "/agents", section: "agents", title: "Agents access"},
		{name: "workspace", path: "/", selectedWorkspace: "workspace@example.com", section: "workspace", title: "Workspace"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			section := activeSectionForRequest(tc.path, tc.selectedWorkspace)
			if section != tc.section {
				t.Fatalf("active section = %q, want %q", section, tc.section)
			}
			if title := pageTitleForSection(section); title != tc.title {
				t.Fatalf("page title = %q, want %q", title, tc.title)
			}
		})
	}
}

func TestIsOrganizationCustomer(t *testing.T) {
	tests := []struct {
		name string
		user *User
		want bool
	}{
		{name: "organization user", user: &User{Email: "owner@example.com", OrganizationID: "org_test"}, want: true},
		{name: "gmail user", user: &User{Email: "owner@gmail.com"}, want: false},
		{name: "nil user", user: nil, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOrganizationCustomer(tc.user); got != tc.want {
				t.Fatalf("isOrganizationCustomer(%+v) = %v, want %v", tc.user, got, tc.want)
			}
		})
	}
}

func TestCountLabelPluralization(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		singular string
		plural   string
		want     string
	}{
		{name: "zero plural", count: 0, singular: "request", plural: "requests", want: "0 requests"},
		{name: "one singular", count: 1, singular: "request", plural: "requests", want: "1 request"},
		{name: "many plural", count: 2, singular: "request", plural: "requests", want: "2 requests"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := countLabel(tc.count, tc.singular, tc.plural); got != tc.want {
				t.Fatalf("countLabel(%d, %q, %q) = %q, want %q", tc.count, tc.singular, tc.plural, got, tc.want)
			}
		})
	}
}

func TestDashboardShowsKPIIndicators(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTemplate.Execute(&out, map[string]any{
		"AppName":       "AI Workspace Proxy",
		"User":          &User{Email: "owner@example.com", Name: "Owner"},
		"CSRFToken":     "csrf_test",
		"ActiveSection": "dashboard",
		"PageTitle":     "Dashboard",
		"ShowDashboard": true,
		"DashboardKPIs": []map[string]any{
			{"Value": "0", "Lines": []string{"Workspaces Connected"}},
			{"Value": "0", "Lines": []string{"Visible Drive Folders"}},
			{"Value": "0", "Lines": []string{"Agents Configured"}},
			{"Value": "1", "Lines": []string{"Permissions", "(Largest Policy)"}},
			{"Value": "0", "Lines": []string{"Agent Requests", "(Last 7 Days)"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, want := range []string{
		`<link rel="stylesheet" href="/static/app.css">`,
		`<div class="app-frame">`,
		`<header class="page-header topbar">`,
		`<img class="topbar-brand-logo" src="/static/brand/sentry-proxy-logo.png" alt="AI Workspace Proxy">`,
		`<strong id="pageCrumbTitle">Dashboard</strong>`,
		`<div class="topbar-actions">`,
		`<button class="topbar-link topbar-link-button topbar-logout-button" type="submit">`,
		`<span>Logout</span>`,
		`<strong class="truncate">Settings</strong>`,
		`<div class="dashboard-kpi-grid">`,
		`class="dashboard-kpi-tile"`,
		`<p class="dashboard-kpi-value">0</p>`,
		`<p class="dashboard-kpi-label"><span class="dashboard-kpi-label-line">Workspaces Connected</span></p>`,
		`<p class="dashboard-kpi-label"><span class="dashboard-kpi-label-line">Visible Drive Folders</span></p>`,
		`<p class="dashboard-kpi-label"><span class="dashboard-kpi-label-line">Agents Configured</span></p>`,
		`<p class="dashboard-kpi-value">1</p>`,
		`<p class="dashboard-kpi-label"><span class="dashboard-kpi-label-line">Permissions</span><span class="dashboard-kpi-label-line">(Largest Policy)</span></p>`,
		`<p class="dashboard-kpi-label"><span class="dashboard-kpi-label-line">Agent Requests</span><span class="dashboard-kpi-label-line">(Last 7 Days)</span></p>`,
		`<h2>Get started</h2>`,
		`class="dashboard-get-started-panel"`,
		`What AI Workspace Proxy does`,
		`AI Workspace Proxy is the control layer between your AI agents and your Google Workspace data.`,
		`Workspace management:</strong> connect one or more Google Workspace accounts and organize them with friendly names.`,
		`Permissions control:</strong> create custom access policies so agents receive only the operations you want them to use.`,
		`Agent access:</strong> create agent profiles, attach Workspace grants, and generate skill packages that match the current settings.`,
		`Auditing:</strong> review activity logs to understand what happened, when it happened, and which identity performed it.`,
		`Fast-track setup`,
		`You can usually get a first agent running in a few short steps:`,
		`Add a Workspace and complete Google authorization for the correct Google account.`,
		`Register any Drive folders you want the proxy to manage for that Workspace.`,
		`Review the <strong>Permissions editor</strong> and create a custom policy if read-only access is not enough.`,
		`Create an agent in <strong>Agent access</strong>, attach one or more Workspace grants, then choose the policy for each grant.`,
		`Download the generated skill for that agent and install it in your AI agent execution platform, for example OpenClaw.`,
		`update the agent skill again when the interface tells you it is needed.`,
		`Security guidelines`,
		`Good defaults help, but careful setup still matters.`,
		`Give each agent only the Workspace access, Drive folders, and permissions it truly needs.`,
		`Use agent accountability, firewall rules, and human review requirements when a workflow needs stronger oversight.`,
		`Check activity logs regularly, especially after policy changes, agent updates, or unusual behavior.`,
		`Protect your own access with a reasonable session duration and optional two-factor authentication.`,
		`The proxy keeps the Google Workspace access tokens on your behalf, while each agent uses its own unique proxy key. If an agent's proxy key is ever exposed or no longer trusted, rotate it immediately.`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("dashboard template missing %q", want)
		}
	}
	for _, removed := range []string{
		`Agent Workspace access`,
		`Open Agents access`,
		`Proxy API Configurations`,
		`Download config`,
		`Rotate API key`,
	} {
		if strings.Contains(html, removed) {
			t.Fatalf("dashboard should not render old dashboard content %q", removed)
		}
	}
}

func TestDashboardPartialNavigationResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-AIWP-Partial-Navigation", "1")
	rec := httptest.NewRecorder()
	renderDashboardResponse(rec, req, map[string]any{
		"AppName":       "AI Workspace Proxy",
		"User":          &User{Email: "owner@example.com", Name: "Owner"},
		"CSRFToken":     "csrf_test",
		"ActiveSection": "dashboard",
		"PageTitle":     "Dashboard",
		"PageSubtitle":  "See your proxy overview, current setup, and security guidance.",
		"ShowDashboard": true,
		"DashboardKPIs": []map[string]any{
			{"Value": "0", "Lines": []string{"Workspaces Connected"}},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("partial response status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"topbar_crumb", "sidebar", "page_header", "page_content"} {
		value, _ := payload[key].(string)
		if strings.TrimSpace(value) == "" {
			t.Fatalf("partial response missing %q: %#v", key, payload)
		}
	}
	if strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Fatal("partial response should not return a full HTML document")
	}
	if !strings.Contains(payload["page_content"].(string), `dashboard-kpi-grid`) {
		t.Fatal("partial page content should include dashboard content")
	}
}

func TestAdminManageAccountsUsesSharedAdminShell(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTemplate.Execute(&out, map[string]any{
		"AppName":       "AI Workspace Proxy",
		"User":          &User{Email: "owner@example.com", Name: "Owner", OrganizationID: "org_test"},
		"CSRFToken":     "csrf_test",
		"ActiveSection": "admin_users",
		"AdminMode":     true,
		"PageTitle":     "Manage accounts",
		"OrganizationAdmin": map[string]any{
			"Verified": true,
			"Domain":   "example.com",
		},
		"ShowAdminUsers": true,
		"AdminUserRows": []map[string]any{
			{
				"ID":                  "usr_1",
				"Name":                "Owner",
				"Email":               "owner@example.com",
				"Role":                "Administrator",
				"RoleKey":             "administrator",
				"Status":              "Active",
				"StatusKey":           "active",
				"StatusClass":         "admin-user-status admin-user-status-active",
				"LastActivityDisplay": "2026-06-18 13:00:00 UTC",
				"LastActivityUnix":    int64(1760000000),
				"EditURL":             "/admin/users/usr_1",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, want := range []string{
		`<span>Admin</span><span class="sep">&gt;</span><strong class="truncate">Manage accounts</strong>`,
		`<p class="sidebar-label">Admin Mode</p>`,
		`<a class="workspace-link admin-nav-link active" href="/admin/users">`,
		`<span class="sidebar-entry-icon" aria-hidden="true"><svg viewBox="0 0 24 24" focusable="false"><circle cx="9" cy="8" r="3"></circle>`,
		`<strong class="truncate">Manage accounts</strong>`,
		`<h2>Manage accounts</h2>`,
		`Search name or email`,
		`<button class="admin-users-sort" type="button" data-admin-sort="role" aria-label="Sort by Role" aria-pressed="false">↑↓</button>`,
		`Rows per page`,
		`owner@example.com`,
		`Administrator`,
		`admin-user-status admin-user-status-active`,
		`data-admin-users-page`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("admin manage accounts template missing %q", want)
		}
	}
	for _, removed := range []string{
		`Denied log:`,
		`/data/logs/denied.log`,
		`Review and manage user accounts`,
	} {
		if strings.Contains(html, removed) {
			t.Fatalf("admin manage accounts template should not contain %q", removed)
		}
	}
}

func TestAdminManageAccountShowsSummaryAndDeleteWarning(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTemplate.Execute(&out, map[string]any{
		"AppName":       "AI Workspace Proxy",
		"User":          &User{Email: "owner@example.com", Name: "Owner", OrganizationID: "org_test"},
		"CSRFToken":     "csrf_test",
		"ActiveSection": "admin_user",
		"AdminMode":     true,
		"PageTitle":     "Manage account",
		"OrganizationAdmin": map[string]any{
			"Verified": true,
			"Domain":   "example.com",
		},
		"ShowAdminUser":             true,
		"AdminUser":                 &User{ID: "usr_1", Email: "member@example.com", Name: "Member", IsAdmin: false, IsSuspended: true},
		"AdminUserCanSuspendDelete": true,
		"AdminUserRoleLabel":        "User",
		"AdminUserStatusLabel":      "Suspended",
		"AdminUserStatusClass":      "admin-user-status admin-user-status-suspended",
		"AdminUserCreatedAtLocal":   "2026-06-01 10:00:00 UTC",
		"AdminUserLastActivity":     "2026-06-18 13:00:00 UTC",
		"AdminUserSuspendedAtLocal": "2026-06-18 14:30:00 UTC",
		"AdminUserSuspendedBy":      "by Owner (owner@example.com)",
	})
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	source := renderedWithAppBehaviorForTest(t, html)
	for _, want := range []string{
		`<span>Admin</span><span class="sep">&gt;</span><strong class="truncate">Manage account</strong>`,
		`<h2>Manage account</h2>`,
		`<strong>Role:</strong> User`,
		`<strong>Status:</strong> <span class="admin-user-status admin-user-status-suspended">Suspended</span>`,
		`<strong>Created:</strong> 2026-06-01 10:00:00 UTC`,
		`<strong>Last activity:</strong> 2026-06-18 13:00:00 UTC`,
		`<strong>Suspension date:</strong> 2026-06-18 14:30:00 UTC by Owner (owner@example.com)`,
		`onclick="return showAdminDeleteUserOverlay()"`,
		`<div id="adminDeleteUserOverlay" class="discovery-overlay"`,
		`<strong>WARNING - IMPORTANT:</strong>`,
		`Deleting this user will permanently remove all user data from the system, including all user-related logs.`,
		`Suspended users do not count towards billing.`,
		`<strong>Do you want to proceed with the user deletion?</strong>`,
		`Yes, delete the user`,
		`function showAdminDeleteUserOverlay(){`,
		`function closeAdminDeleteUserOverlay(){`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("admin manage account template missing %q", want)
		}
	}
	for _, removed := range []string{
		`<h2>Last 30 days</h2>`,
		`Delete user and stored credentials?`,
		`<strong>Admin:</strong>`,
		`<strong>Suspended:</strong>`,
	} {
		if strings.Contains(html, removed) {
			t.Fatalf("admin manage account template should not contain %q", removed)
		}
	}
}

func TestAdminManageOwnAccountHidesSuspendAndDeleteActions(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTemplate.Execute(&out, map[string]any{
		"AppName":       "AI Workspace Proxy",
		"User":          &User{ID: "usr_admin", Email: "owner@example.com", Name: "Owner", OrganizationID: "org_test"},
		"CSRFToken":     "csrf_test",
		"ActiveSection": "admin_user",
		"AdminMode":     true,
		"PageTitle":     "Manage account",
		"OrganizationAdmin": map[string]any{
			"Verified": true,
			"Domain":   "example.com",
		},
		"ShowAdminUser":             true,
		"AdminUser":                 &User{ID: "usr_admin", Email: "owner@example.com", Name: "Owner", IsAdmin: true},
		"AdminUserCanSuspendDelete": false,
		"AdminUserRoleLabel":        "Administrator",
		"AdminUserStatusLabel":      "Active",
		"AdminUserStatusClass":      "admin-user-status admin-user-status-active",
		"AdminUserCreatedAtLocal":   "2026-06-01 10:00:00 UTC",
		"AdminUserLastActivity":     "2026-06-18 13:00:00 UTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, want := range []string{
		`You cannot suspend or delete your own administrator account.`,
		`<strong>Role:</strong> Administrator`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("admin own account template missing %q", want)
		}
	}
	for _, removed := range []string{
		`Suspend user`,
		`Delete user`,
		`<div id="adminDeleteUserOverlay"`,
	} {
		if strings.Contains(html, removed) {
			t.Fatalf("admin own account template should not contain %q", removed)
		}
	}
}

func TestOrganizationAdminNavigationAndPlaceholderPage(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTemplate.Execute(&out, map[string]any{
		"AppName":                    "AI Workspace Proxy",
		"User":                       &User{Email: "owner@example.com", Name: "Owner", OrganizationID: "org_test"},
		"CSRFToken":                  "csrf_test",
		"ActiveSection":              "admin",
		"AdminMode":                  true,
		"PageTitle":                  "Admin",
		"ShowOrgAdmin":               true,
		"IsOrganizationCustomer":     true,
		"CanAccessOrganizationAdmin": true,
		"OrganizationAdmin": map[string]any{
			"Domain":               "example.com",
			"TXTRecordName":        "_ai-workspace-proxy-verification",
			"TXTRecordZone":        "example.com",
			"TXTRecordHost":        "_ai-workspace-proxy-verification.example.com",
			"TXTRecordValue":       "ai-workspace-proxy-verification=orgv_test",
			"Verified":             false,
			"HasVerifiedAdmin":     false,
			"ShowClaimIntro":       true,
			"StatusText":           "Unverified",
			"StatusClass":          "logging-status-disabled",
			"VerifiedAtDisplay":    "",
			"ValidationTargetFQDN": "_ai-workspace-proxy-verification.example.com",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	source := renderedWithAppBehaviorForTest(t, html)
	for _, want := range []string{
		`<body class="admin-mode" data-active-section="admin">`,
		`<link rel="stylesheet" href="/static/app.css">`,
		`<header class="page-header topbar">`,
		`<img class="topbar-brand-logo" src="/static/brand/sentry-proxy-logo.png" alt="AI Workspace Proxy">`,
		`<span class="topbar-brand-mode">Admin</span>`,
		`<p class="sidebar-label">Admin Mode</p>`,
		`<a class="workspace-link admin-return-link" href="/">`,
		`<span class="sidebar-entry-icon" aria-hidden="true"><svg viewBox="0 0 24 24" focusable="false"><path d="M10 6l-6 6 6 6"></path>`,
		`<strong class="truncate">Return to user mode</strong>`,
		`<p class="sidebar-label">Overview</p>`,
		`<a class="workspace-link admin-nav-link active" href="/admin/">`,
		`<strong class="truncate">Claim administration</strong>`,
		`<strong id="pageCrumbTitle">Admin</strong>`,
		`<div id="organizationAdminVerifiedOverlay" class="discovery-overlay" aria-hidden="true">`,
		`<h2 id="organizationAdminVerifiedOverlayTitle">Domain successfully verified</h2>`,
		`<div id="organizationAdminVerifiedOverlayBody"></div>`,
		`<button class="overlay-close" type="button" aria-label="Close overlay" onclick="closeOrganizationAdminVerifiedOverlay()">&times;</button>`,
		`<button class="btn" type="button" onclick="closeOrganizationAdminVerifiedOverlay()">Understood</button>`,
		`<h2>Claim the administration for your domain</h2>`,
		`<strong>Domain verification status:</strong> <span id="organizationAdminStatusText" class="logging-status-disabled">Unverified</span>`,
		`This domain does not have any registered administrators yet. Claim administration to allow the management of other user accounts under this domain.`,
		`<button class="btn" type="button" onclick="return showOrganizationAdminClaimFlow()">Claim administration for the domain</button>`,
		`function showOrganizationAdminVerifiedOverlay(data){`,
		`title.textContent = 'Domain ' + domain + ' successfully verified';`,
		`You are now the primary admin for this domain in AI Workspace Proxy, and the DNS TXT record can safely be deleted.`,
		`When you close this message, you will be redirected to the Admin Dashboard.`,
		`function closeOrganizationAdminVerifiedOverlay(){`,
		`window.location.assign('/admin/');`,
		`function showOrganizationAdminClaimFlow(){`,
		`<h3>TXT record name</h3>`,
		`Create this DNS TXT record name inside the DNS zone for <strong>example.com</strong>:`,
		`onclick="copyElementText('organizationAdminTXTRecordName', this)"`,
		`aria-label="Copy TXT record name"`,
		`<code id="organizationAdminTXTRecordName">_ai-workspace-proxy-verification</code>`,
		`<h3>TXT record value</h3>`,
		`onclick="copyElementText('organizationAdminTXTRecordValue', this)"`,
		`aria-label="Copy TXT record value"`,
		`<code id="organizationAdminTXTRecordValue">ai-workspace-proxy-verification=orgv_test</code>`,
		`<li>Once the domain is verified, the TXT record can be safely deleted.</li>`,
		`The proxy verifies the full DNS name <code>_ai-workspace-proxy-verification.example.com</code> directly against the domain's authoritative DNS servers.`,
		`<button id="organizationAdminVerifyButton" class="btn" type="submit">Validate organization domain</button>`,
		`data-ajax-update="org-domain-verify"`,
		`function copyElementText(id, button){`,
		`function startOrganizationAdminCooldown(form){`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("organization admin template missing %q", want)
		}
	}
	for _, removed := range []string{
		`Dashboard</strong>`,
		`Workspaces</p>`,
		`Agent Access</p>`,
		`Permissions</p>`,
		`Logs</p>`,
	} {
		if strings.Contains(html, removed) {
			t.Fatalf("organization admin page should not render standard-mode navigation content %q", removed)
		}
	}
	if strings.Contains(source, `function updateOrganizationAdminVerificationUI(data){`) {
		t.Fatal("organization admin page should not contain the old inline page update helper")
	}

	out.Reset()
	err = dashboardTemplate.Execute(&out, map[string]any{
		"AppName":                    "AI Workspace Proxy",
		"User":                       &User{Email: "owner@example.com", Name: "Owner", OrganizationID: "org_test"},
		"CSRFToken":                  "csrf_test",
		"ActiveSection":              "dashboard",
		"PageTitle":                  "Dashboard",
		"ShowDashboard":              true,
		"DashboardKPIs":              []map[string]any{},
		"IsOrganizationCustomer":     true,
		"CanAccessOrganizationAdmin": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	html = out.String()
	for _, want := range []string{
		`<a class="workspace-link org-admin-link" href="/admin/">`,
		`<span class="sidebar-entry-icon" aria-hidden="true"><svg viewBox="0 0 24 24" focusable="false"><path d="M12 3l7 3v5c0 5-3.2 8.2-7 10-3.8-1.8-7-5-7-10V6l7-3z"></path>`,
		`<strong class="truncate">Organization Admin</strong>`,
		`<a class="workspace-link active" href="/">`,
		`<strong class="truncate">Dashboard</strong>`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("standard navigation should show organization admin entry %q", want)
		}
	}

	out.Reset()
	err = dashboardTemplate.Execute(&out, map[string]any{
		"AppName":                    "AI Workspace Proxy",
		"User":                       &User{Email: "owner@example.com", Name: "Owner", OrganizationID: "org_test"},
		"CSRFToken":                  "csrf_test",
		"ActiveSection":              "admin",
		"AdminMode":                  true,
		"PageTitle":                  "Admin",
		"ShowOrgAdmin":               true,
		"IsOrganizationCustomer":     true,
		"CanAccessOrganizationAdmin": true,
		"OrganizationAdmin": map[string]any{
			"Domain":               "example.com",
			"TXTRecordName":        "_ai-workspace-proxy-verification",
			"TXTRecordZone":        "example.com",
			"TXTRecordHost":        "_ai-workspace-proxy-verification.example.com",
			"TXTRecordValue":       "ai-workspace-proxy-verification=orgv_test",
			"Verified":             true,
			"HasVerifiedAdmin":     true,
			"ShowClaimIntro":       false,
			"StatusText":           "Verified",
			"StatusClass":          "logging-status-enabled",
			"VerifiedAtDisplay":    "2026-06-17 13:07:11 UTC",
			"ValidationTargetFQDN": "_ai-workspace-proxy-verification.example.com",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	html = out.String()
	for _, want := range []string{
		`<strong class="truncate">Dashboard</strong>`,
		`<p class="sidebar-label">User management</p>`,
		`<a class="workspace-link admin-nav-link " href="/admin/users">`,
		`<strong class="truncate">Manage accounts</strong>`,
		`<a class="workspace-link admin-nav-link " href="/admin/accounts-policy">`,
		`<strong class="truncate">Accounts Policy</strong>`,
		`<strong>Domain verification status:</strong> <span id="organizationAdminStatusText" class="logging-status-enabled">Verified</span>`,
		`<strong><span class="logging-status-enabled">Verified</span></strong>: you are the AI Workspace Proxy admin for the domain. The DNS TXT record can now be safely deleted.`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("verified organization admin template missing %q", want)
		}
	}
	for _, removed := range []string{
		`review all user accounts under the domain, manage their roles, accesses, and privileges`,
		`Administrative API capabilities will also be added here`,
		`Designate at least one secondary administrator`,
		`Protect all privileged administrator accounts with two-factor authentication.`,
	} {
		if strings.Contains(html, removed) {
			t.Fatalf("verified organization admin template should not contain removed guidance %q", removed)
		}
	}

	out.Reset()
	err = dashboardTemplate.Execute(&out, map[string]any{
		"AppName":                    "AI Workspace Proxy",
		"User":                       &User{Email: "owner@example.com", Name: "Owner", OrganizationID: "org_test"},
		"CSRFToken":                  "csrf_test",
		"ActiveSection":              "dashboard",
		"PageTitle":                  "Dashboard",
		"ShowDashboard":              true,
		"ShowOrgAdmin":               false,
		"DashboardKPIs":              []map[string]any{},
		"IsOrganizationCustomer":     true,
		"CanAccessOrganizationAdmin": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	html = out.String()
	if strings.Contains(html, `<h2>Organization admin</h2>`) {
		t.Fatal("dashboard page should not render the organization admin content card")
	}
}

func TestFormatDashboardKPIValue(t *testing.T) {
	tests := []struct {
		value int
		want  string
	}{
		{value: 0, want: "0"},
		{value: 12, want: "12"},
		{value: 999, want: "999"},
		{value: 1000, want: "1k"},
		{value: 128500, want: "128.5k"},
		{value: 1234567, want: "1.2M"},
		{value: 1999000000, want: "2B"},
		{value: -1250, want: "-1.2k"},
	}
	for _, tc := range tests {
		if got := formatDashboardKPIValue(tc.value); got != tc.want {
			t.Fatalf("formatDashboardKPIValue(%d) = %q, want %q", tc.value, got, tc.want)
		}
	}
}

func TestWorkspaceFormsUseCompactSizingClasses(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTemplate.Execute(&out, map[string]any{
		"AppName":               "AI Workspace Proxy",
		"User":                  &User{Email: "owner@example.com", Name: "Owner"},
		"CSRFToken":             "csrf_test",
		"ActiveSection":         "workspace",
		"PageTitle":             "Workspace",
		"WorkspaceConnected":    true,
		"WorkspaceAccountError": "Google returned wrong@example.com, but this saved Workspace expects workspace@example.com.",
		"Workspace": map[string]any{
			"Email":            "workspace@example.com",
			"Name":             "Spam",
			"Selector":         "workspace@example.com",
			"Scopes":           []string{"Google Drive access"},
			"ConnectedSince":   "2026-06-14 12:00:00 UTC",
			"ConnectionStatus": "Active",
			"AuthConnected":    true,
		},
		"DriveFolders": []AllowedDriveFolder{
			{
				ID:            "folder_test",
				ReferenceName: "Test Folder",
				FolderName:    "Test Folder",
				FolderURL:     "https://drive.google.com/drive/folders/folder_test",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	source := renderedWithAppBehaviorForTest(t, html)
	for _, want := range []string{
		`<link rel="stylesheet" href="/static/app.css">`,
		`Workspace account - <span id="workspaceAccountTitleName">Spam</span>`,
		`<strong>Google auth status:</strong> <span style="color:#137333;font-weight:700;">Active</span>`,
		`aria-controls="workspaceAccountDetails" onclick="toggleDetails(this)">Show more details</button>`,
		`id="workspaceAccountDetails" class="details-panel" hidden`,
		`Workspace's friendly name`,
		`<em>"Get my unread emails from my Perso mailbox."</em>`,
		`Refresh Google auth &amp; scopes`,
		`Disconnect Workspace`,
		`Delete workspace settings`,
		`Remove the stored Google authorization tokens for this Workspace but keep its proxy settings.`,
		`Delete all saved proxy settings for this Workspace? This will remove its settings from the proxy. Agents configured to use this Workspace will no longer work. This has no impact on your Google account and data.`,
		`account-update-form workspace-friendly-form`,
		`data-ajax-form="true" data-ajax-update="workspace-friendly-name" data-suppress-success-flash="true"`,
		`document.getElementById('workspaceAccountTitleName')`,
		`name="friendly_name"`,
		`Add an allowed Drive folder`,
		`data-show-label="Show help &amp; guidelines" data-hide-label="Hide help &amp; guidelines" onclick="toggleDetails(this)">Show help &amp; guidelines</button>`,
		`id="driveFolderGuidelines" class="details-panel" hidden`,
		`This section is where you register Google Drive folders for this Workspace account.`,
		`Registering a folder here does not give AI agents access to it by itself.`,
		`Agent access is granted later, in the <code>Agent access</code> settings`,
		`The proxy only allows AI agents to work with Drive folders that you have registered and later allowed through agent access settings.`,
		`give it a unique <code>Reference Name</code>.`,
		`different folders happen to have similar or identical names.`,
		`the proxy explores and caches its subfolder structure.`,
		`use the "Refresh tree" button for that folder. This updates the proxy's cached folder structure`,
		`An agent can also request a tree refresh, but this should be done only when needed because it may consume Google API quota`,
		`Allowed Drive folders`,
		`class="folder-form"`,
		`class="drive-folders-table"`,
		`<th class="reference-column">Reference Name</th>`,
		`<th class="actions-column">Actions</th>`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("workspace template missing %q", want)
		}
	}
	if strings.Contains(html, "Google Workspace account") {
		t.Fatal("workspace template still contains Google Workspace account title")
	}
	for _, removed := range []string{
		`Applied access policy:`,
		`id="workspaceAppliedPolicyName"`,
		`Workspace proxy policy`,
		`id="workspace_policy_id"`,
		`Agent Workspace API key`,
		`name="allow_docs"`,
		`name="allow_sheets"`,
		`name="allow_slides"`,
		`name="allow_drive_files"`,
		`Allowed types`,
		`Other file types`,
		`prepareDriveFolderSubmit`,
		`ensureDriveTypeSelection`,
	} {
		if strings.Contains(html, removed) {
			t.Fatalf("workspace template still contains removed workspace policy/API-key or Drive folder type element %q", removed)
		}
	}
	if strings.Contains(html, "Use a unique Reference Name.") {
		t.Fatal("workspace template still contains old Drive folder helper copy")
	}
	oldHyphenatedLabel := "Non-" + "Google files"
	oldSpacedLabel := "Non " + "Google files"
	if strings.Contains(html, oldHyphenatedLabel) || strings.Contains(html, oldSpacedLabel) {
		t.Fatal("workspace template still contains old Drive file type label")
	}
	detailsEnd := strings.Index(html, `</div>
    <div class="account-actions">`)
	disconnectIndex := strings.Index(html, `Disconnect Workspace`)
	if detailsEnd == -1 || disconnectIndex == -1 || disconnectIndex < detailsEnd {
		t.Fatal("disconnect button should remain visible outside the hidden workspace details panel")
	}
	accountActionsIndex := strings.Index(html, `<div class="account-actions">`)
	workspaceErrorIndex := strings.Index(html, `Google returned wrong@example.com, but this saved Workspace expects workspace@example.com.`)
	friendlyNameSectionIndex := strings.Index(html, `Workspace's friendly name`)
	if accountActionsIndex == -1 || workspaceErrorIndex == -1 || friendlyNameSectionIndex == -1 {
		t.Fatal("expected workspace actions, workspace auth error, and friendly name section to all be present")
	}
	if workspaceErrorIndex < accountActionsIndex || workspaceErrorIndex > friendlyNameSectionIndex {
		t.Fatal("workspace auth error should appear below the top workspace action buttons and before the friendly name section")
	}
}

func TestRequestLogTableStaysConstrainedInsideBrowser(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTemplate.Execute(&out, map[string]any{
		"AppName":       "AI Workspace Proxy",
		"User":          &User{Email: "owner@example.com", Name: "Owner"},
		"CSRFToken":     "csrf_test",
		"ActiveSection": "logs",
		"PageTitle":     "Logs",
		"ShowLogs":      true,
		"LoggingSettings": &UserLoggingSettings{
			UserID:              "usr_test",
			RequireAgentContext: false,
			RetentionDays:       7,
		},
		"LoggingRetentionUnit": "days",
		"LogRangeStart":        "2026-06-08T00:00:00Z",
		"LogRangeEnd":          "2026-06-14T00:00:00Z",
		"LogRangeStartDate":    "2026-06-08",
		"LogRangeEndDate":      "2026-06-14",
	})
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	source := renderedWithAppBehaviorForTest(t, html)
	for _, want := range []string{
		`<link rel="stylesheet" href="/static/app.css">`,
		`<link rel="stylesheet" href="/static/vendor/tabulator/tabulator.min.css">`,
		`<div class="log-table-wrap">`,
		`<h2>Activity logs</h2>`,
		`<h3 id="retentionTitle">Retention: 7 days</h3>`,
		`Standard accounts keep logs for 7 days before automatic cleanup.`,
		`<div id="logTypeFilters" class="log-type-filters" aria-label="Log types"></div>`,
		`<div id="requestLogTable"`,
		`<div id="logDetailOverlay" class="log-detail-overlay" role="dialog" aria-modal="true" aria-labelledby="logDetailTitle" hidden>`,
		`<button class="btn" id="copyLogDetail" type="button">Copy</button>`,
		`<button class="btn" id="closeLogDetail" type="button">Close</button>`,
		`<div id="logRowContextMenu" class="log-context-menu" hidden></div>`,
		`<button class="btn" id="toggleLogColumns" type="button">Columns</button>`,
		`<button class="btn" id="exportCurrentJSONL" type="button">Export view JSONL</button>`,
		`<button class="btn" id="exportAllJSONL" type="button">Export all JSONL</button>`,
		`<button class="btn" id="exportAllCSV" type="button">Export all CSV</button>`,
		`<button class="btn" id="resetLogColumnSelection" type="button">Reset selection</button>`,
		`<button class="btn" id="resetLogColumnOrder" type="button">Reset column order</button>`,
		`<button class="btn" id="applyLogColumns" type="button">Apply changes</button>`,
		`const selectedLogTypes = () => logTypeFilters ? [...logTypeFilters.querySelectorAll('input[data-log-type]:checked')].map(input => input.value) : [];`,
		`logTypes.forEach(logType => params.append('log_type', logType));`,
		`params.set('log_type', '');`,
		`const renderLogTypeFilters = () => {`,
		`checkbox.dataset.logType = logType.id;`,
		`help.className = 'risk-help';`,
		`help.dataset.tooltip = logType.description || logType.label;`,
		`state.logTypes = catalog.log_types || [];`,
		`renderLogTypeFilters();`,
		`const columnPickerOrderedColumns = () => [...state.columns];`,
		`let pendingColumnOrder = null;`,
		`const defaultAwareColumnOrder = columns => {`,
		`const selectedColumnsInPendingOrder = columns => {`,
		`const baseOrder = pendingColumnOrder && pendingColumnOrder.length ? pendingColumnOrder : state.visible;`,
		`const next = selectedColumnsInPendingOrder(checked);`,
		`pendingColumnOrder = defaultVisibleColumns.slice();`,
		`pendingColumnOrder = defaultAwareColumnOrder(selectedColumnPickerValues());`,
		`const setColumnPickerSelection = columns => {`,
		`document.getElementById('resetLogColumnSelection')?.addEventListener('click', () => {`,
		`document.getElementById('resetLogColumnOrder')?.addEventListener('click', () => {`,
		`window.alert('Column order will be reset once the changes are applied.');`,
		`const maxInitialColumnChars = 30;`,
		`const widthForChars = chars => Math.max(48, Math.round((chars * 8.4) + 28));`,
		`const selectedColumnWidthTotal = () => {`,
		`const liveWidth = state.table.getColumns().reduce((total, column) => {`,
		`tableEl.style.minWidth = Math.max(selectedColumnWidthTotal(), visibleWidth || 0) + 'px';`,
		`layout: 'fitDataTable'`,
		`const savedWidth = Number(state.widths[id] || 0);`,
		`maxInitialWidth: widthForChars(maxInitialColumnChars),`,
		`selectableRows: true`,
		`selectableRowsPersistence: true`,
		`selectableRowsRangeMode: 'click'`,
		`state.table.on('rowDblClick', function(e, row){ showDetail(row.getData()); });`,
		`state.table.on('rowContext', function(e, row){`,
		`Open entry details`,
		`Copy details (view only)`,
		`Copy details (full)`,
		`Export in CSV (view only)`,
		`Export in CSV (all details)`,
		`Export in JSONL (view only)`,
		`Export in JSONL (all details)`,
		`downloadText('selected-activity-logs-view.csv'`,
		`downloadText('selected-activity-logs-full.jsonl'`,
		`const params = scope === 'all' ? new URLSearchParams() : queryParams(1);`,
		`if (scope === 'all') {`,
		`params.set('start_date', startInput ? startInput.dataset.value || '' : '');`,
		`params.set('end_date', endInput ? endInput.dataset.value || '' : '');`,
		`const redrawLogTable = () => {`,
		`new ResizeObserver(redrawLogTable)`,
		`observer.observe(tableEl.parentElement)`,
		`state.table.on('dataLoaded', function(){`,
		`window.requestAnimationFrame(syncLogTableScrollWidth);`,
		`columnPanel.hidden = true;`,
		`function containScrollWithin(element){`,
		`document.body.classList.add('modal-open');`,
		`document.body.classList.remove('modal-open');`,
		`containScrollWithin(detailBody);`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("logs template missing %q", want)
		}
	}
	if strings.Contains(source, `layout: 'fitColumns'`) {
		t.Fatal("request log table should not use fitColumns because it prevents horizontal scrolling to wide columns")
	}
	if strings.Contains(source, `#requestLogTable.tabulator{width:max-content;`) {
		t.Fatal("request log table should not force width:max-content on the Tabulator root because it constrains manual column resizing")
	}
	if strings.Contains(source, `observer.observe(tableEl);`) {
		t.Fatal("request log table should not observe the table element itself because that breaks manual column resizing")
	}
	if strings.Contains(source, `id="logDetailDrawer"`) || strings.Contains(source, `log-detail-drawer`) || strings.Contains(source, `state.table.on('rowClick', function(e, row){ showDetail(row.getData()); });`) {
		t.Fatal("logs page should use a double-click modal instead of the old detail drawer")
	}
	if strings.Contains(html, `<h2>Request logs</h2>`) || strings.Contains(html, `<h2>Audit logs</h2>`) {
		t.Fatal("logs page should show the Activity logs browser without the old audit log section")
	}
	if strings.Contains(html, "Timestamp UTC") || strings.Contains(html, "timestamp_utc") {
		t.Fatal("request log UI should expose a single Timestamp column in the user's timezone")
	}
	if strings.Contains(html, `id="toggleLogColumns" type="button">Columns</button>`) && strings.Contains(html, `class="btn secondary" id="toggleLogColumns"`) {
		t.Fatal("request log toolbar buttons should use the primary blue button style")
	}
	for _, removed := range []string{
		`id="exportCurrentJSON"`,
		`id="exportAllJSON"`,
		`id="loggingStatusForm"`,
		`id="loggingStatusButton"`,
		`Logging status:`,
		`Disable logging`,
		`Enable logging`,
		`action="/logs/clear"`,
		`Clear logs`,
		`Clear all logs`,
		`Update retention duration`,
		`name="retention_days"`,
		`Agent accountability:`,
		`Enforce AI agent accountability`,
		`Disable AI agent accountability`,
		`action="/logs/settings"`,
		`setting_action" value="agent_accountability"`,
		`id="agentAccountabilityForm"`,
		`id="agentAccountabilityButton"`,
		`id="agentAccountabilityValue"`,
	} {
		if strings.Contains(html, removed) {
			t.Fatalf("logs page should not expose removed logging control %q", removed)
		}
	}
}

func TestSectionMessagesRenderDismissibleFlashes(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTemplate.Execute(&out, map[string]any{
		"AppName":        "AI Workspace Proxy",
		"User":           &User{Email: "owner@example.com", Name: "Owner"},
		"CSRFToken":      "csrf_test",
		"ActiveSection":  "settings",
		"PageTitle":      "Settings",
		"ShowSettings":   true,
		"SettingsError":  "Unable to save settings.",
		"SettingsSaved":  true,
		"WorkspaceCount": 0,
		"Settings": map[string]any{
			"Timezone":               "UTC",
			"CurrentTime":            "2026-06-14 12:00:00 UTC",
			"SessionTimeoutHours":    24,
			"SessionStartedAt":       "2026-06-14 10:00:00 UTC",
			"SessionExpiresAt":       "2026-06-15 10:00:00 UTC",
			"SessionExpiresAtUnixMS": int64(1749981600000),
			"Timezones": []map[string]any{
				{"Name": "UTC", "Selected": true},
			},
		},
		"TwoFactor": map[string]any{
			"Enabled":      false,
			"Pending":      false,
			"Status":       "Disabled",
			"AppName":      "AI Workspace Proxy",
			"AccountEmail": "owner@example.com",
			"QRCodeURL":    "/settings/2fa/qr.png",
			"CanReset":     false,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	source := renderedWithAppBehaviorForTest(t, html)
	for _, want := range []string{
		`<div class="flash flash-error"><span>Unable to save settings.</span><button class="flash-close" type="button" aria-label="Close message" onclick="dismissFlash(this)">x</button></div>`,
		`<div class="flash flash-success"><span>Settings saved.</span><button class="flash-close" type="button" aria-label="Close message" onclick="dismissFlash(this)">x</button></div>`,
		`Two-factor authentication`,
		`User Interface Session Timeout`,
		`Choose how long your browser session stays signed in before the User Interface asks you to sign in again.`,
		`action="/settings/session-timeout"`,
		`name="session_timeout_hours" min="1" max="8760" step="1" value="24"`,
		`session-timeout-field`,
		`Update session duration`,
		`Session time left: <span id="sessionTimeLeft" data-expires-at-ms="1749981600000"></span>`,
		`function initSavedCurrentTimeClock(){`,
		`function initSessionTimeLeftClock(){`,
		`const dayLabel = days === 1 ? 'day' : 'days';`,
		`String(days).padStart(2, '0') + ' ' + dayLabel + ', ' + String(hours).padStart(2, '0') + ' h ' + String(minutes).padStart(2, '0') + ' m ' + String(seconds).padStart(2, '0') + ' s'`,
		`Protect your browser sign-in with a six-digit code from an authenticator app such as Google Authenticator.`,
		`<strong>Status:</strong> <span style="font-weight:700;">Disabled</span>`,
		`Enable 2FA`,
		`function dismissFlash(button)`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("settings template missing %q", want)
		}
	}
	for _, removed := range []string{
		`Scan the QR code with your authenticator app, then confirm the six-digit code to enable 2FA.`,
		`Disable or reset 2FA enrollment`,
	} {
		if strings.Contains(html, removed) {
			t.Fatalf("settings template should not render %q in the disabled 2FA state", removed)
		}
	}
}

func TestPolicyDefaultSelectorUsesCompactSizing(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTemplate.Execute(&out, map[string]any{
		"AppName":       "AI Workspace Proxy",
		"User":          &User{Email: "owner@example.com", Name: "Owner"},
		"CSRFToken":     "csrf_test",
		"ActiveSection": "policies",
		"PageTitle":     "Permissions editor",
		"ShowPolicies":  true,
		"PolicyEditor": map[string]any{
			"Options": []map[string]any{
				{"ID": systemPolicyID, "Name": systemPolicyName, "Selected": true, "IsDefault": true, "System": true, "FullLabel": "System policy (default) (not editable & read permissions only)", "DisplayLabel": "System policy (default) (not editable & read permissions only)"},
				{"ID": "pol_custom", "Name": "Long custom policy name", "Selected": false, "IsDefault": false, "System": false, "FullLabel": "Long custom policy name", "DisplayLabel": "Long custom policy name"},
			},
			"SelectedName":      systemPolicyName,
			"SelectedID":        systemPolicyID,
			"SelectedIsSystem":  true,
			"SelectedIsDefault": true,
			"CapabilityGroups": []map[string]any{
				{"Name": "Calendar", "ShowSubgroups": false, "Subgroups": []map[string]any{}},
				{"Name": "Contacts", "ShowSubgroups": false, "Subgroups": []map[string]any{}},
				{"Name": "Drive", "ShowSubgroups": false, "Subgroups": []map[string]any{}},
				{"Name": "Gmail", "ShowSubgroups": false, "Subgroups": []map[string]any{}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	source := renderedWithAppBehaviorForTest(t, html)
	for _, want := range []string{
		`<h2>Policy library</h2>`,
		`<label for="policy_select">Open policy</label>`,
		`<h2>Default policy</h2>`,
		`title="System policy (default) (not editable &amp; read permissions only)"`,
		`<select id="policy_select" data-size-to-options="true" data-size-max-ch="75" onchange="selectPolicyForEditing(this)">`,
		`<select id="default_policy_id" name="default_policy_id" data-size-to-options="true" data-size-max-ch="75">`,
		`sizeSelectToOptions(document.getElementById('default_policy_id'));`,
		`sizeSelectToOptions(document.getElementById('policy_select'));`,
		`const policySelect = document.getElementById('policy_select');`,
		`function truncatePolicyOptionLabel(text){`,
		`src="/static/google-workspace-icons/calendar.png"`,
		`src="/static/google-workspace-icons/contacts.png"`,
		`src="/static/google-workspace-icons/drive.png"`,
		`src="/static/google-workspace-icons/gmail.png"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("policy template missing %q", want)
		}
	}
}

func TestAgentsAccessTemplateShowsAgentGrantsAndScopedSkillDownload(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTemplate.Execute(&out, map[string]any{
		"AppName":       "AI Workspace Proxy",
		"User":          &User{Email: "owner@example.com", Name: "Owner"},
		"CSRFToken":     "csrf_test",
		"ActiveSection": "agents",
		"PageTitle":     "Agents access",
		"ShowAgents":    true,
		"Agents": []map[string]any{
			{
				"ID":                "agt_test",
				"Name":              "OpenClaw",
				"Location":          "webchat",
				"Active":            true,
				"SkillStale":        true,
				"SkillStaleReasons": []string{"Workspace grants changed."},
			},
		},
		"SelectedAgents": []map[string]any{
			{
				"ID":                   "agt_test",
				"Name":                 "OpenClaw",
				"Location":             "webchat",
				"Enabled":              true,
				"TokenHint":            "atk_tes...est",
				"Activity24Hours":      12,
				"Activity24HoursLabel": "12 requests",
				"SkillStale":           true,
				"SkillStaleReasons":    []string{"Workspace grants changed."},
				"FirewallEnabled":      true,
				"FirewallRuleCount":    1,
				"FirewallRules": []map[string]any{
					{
						"ID":        "afw_test",
						"Type":      "IPv4",
						"DateAdded": "2026-06-16 10:00:00 UTC",
						"Value":     "203.0.113.0/24",
					},
				},
				"WorkspaceRows": []map[string]any{
					{
						"Email":              "workspace@example.com",
						"Name":               "Workspace",
						"Checked":            true,
						"RequireAgentMotive": true,
						"FormKey":            "workspace_at_example_dot_com",
						"PolicyOptions": []map[string]any{
							{"ID": "system", "Name": "Default system policy", "DisplayName": "Default system policy", "Selected": true},
						},
						"AllowedDriveFolderCount":      1,
						"AllowedDriveFolderCountLabel": "1 folder allowed",
						"DriveFolderRows": []map[string]any{
							{
								"ID":            "dfr_reports",
								"ReferenceName": "Reports",
								"FolderName":    "Reports Real Name",
								"FolderURL":     "https://drive.google.com/drive/folders/reports",
								"Checked":       true,
							},
							{
								"ID":            "dfr_archive",
								"ReferenceName": "Archive",
								"FolderName":    "Archive Real Name",
								"FolderURL":     "https://drive.google.com/drive/folders/archive",
								"Checked":       false,
							},
						},
					},
				},
			},
		},
		"SelectedAgentID":   "agt_test",
		"SelectedAgentName": "OpenClaw",
	})
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	source := renderedWithAppBehaviorForTest(t, html)
	for _, want := range []string{
		`<link rel="stylesheet" href="/static/app.css">`,
		`<header class="page-header topbar">`,
		`<span>Agents access</span><span class="sep">&gt;</span><strong class="truncate">OpenClaw</strong>`,
		`href="/auth/workspace/connect"><span class="sidebar-button-icon" aria-hidden="true"><svg viewBox="0 0 24 24" focusable="false"><circle cx="12" cy="12" r="9"></circle>`,
		`<span>Add a Google account</span></a>`,
		`<p class="sidebar-label">AI Agents Profiles &amp; Accesses</p>`,
		`<button class="btn sidebar-connect" type="button" onclick="return showAgentCreateOverlay()"><span class="sidebar-button-icon" aria-hidden="true"><svg viewBox="0 0 24 24" focusable="false"><circle cx="12" cy="12" r="9"></circle>`,
		`<span>Create an agent profile</span></button>`,
		`<a id="agentNav_agt_test" class="workspace-link active" href="/agents?agent=agt_test">`,
		`<span class="sidebar-entry-icon" aria-hidden="true"><svg viewBox="0 0 24 24" focusable="false"><rect x="7" y="8" width="10" height="8" rx="2"></rect>`,
		`<strong id="agentNavName_agt_test" class="truncate">OpenClaw</strong>`,
		`<span id="agentSkillStaleBadge_agt_test" class="agent-skill-stale-badge" title="Agent skill update needed" aria-label="Agent skill update needed" >!</span>`,
		`<div id="agentSkillWarning_agt_test" class="flash flash-error agent-skill-warning" >`,
		`SKILL UPDATE REQUIRED`,
		`Update the installed skill so this agent receives the latest access settings. You may re-download the skill package or ask your agent to update the skill.`,
		`<li>Workspace grants changed.</li>`,
		`onclick="dismissAgentSkillWarning('agt_test')"`,
		`<p class="sidebar-label">Permissions</p>`,
		`<p class="sidebar-label">Logs</p>`,
		`<div id="agentCreateOverlay" class="discovery-overlay" role="dialog" aria-modal="true" aria-labelledby="agentCreateTitle">`,
		`<button class="overlay-close" type="button" aria-label="Close overlay" onclick="closeAgentCreateOverlay()">&times;</button>`,
		`<div id="agentSkillInstallError" class="flash flash-error" hidden><span></span><button class="flash-close" type="button" aria-label="Close message" onclick="dismissFlash(this)">x</button></div>`,
		`<form method="post" action="/agents/create" class="overlay-form" data-require-complete="true">`,
		`name="friendly_name" maxlength="256" required`,
		`name="default_location" maxlength="512" required`,
		`<button class="btn warn" type="button" onclick="closeAgentCreateOverlay()">Cancel</button>`,
		`<button class="btn" type="submit">Confirm</button>`,
		`function showAgentCreateOverlay()`,
		`function closeAgentCreateOverlay()`,
		`function showAgentEditOverlay(agentID, field)`,
		`function closeAgentEditOverlay()`,
		`<form id="agentEditForm" method="post" action="/agents/update" class="overlay-form" data-ajax-form="true" data-ajax-update="agent-profile" data-suppress-success-flash="true">`,
		`measurer.getBoundingClientRect().width`,
		`const arrowAllowance = 28;`,
		`function updateAgentToggleUI(data)`,
		`function updateAgentProfileUI(data)`,
		`function updateAgentTokenUI(data)`,
		`function updateAgentDeletedUI(data)`,
		`function formatMatchingLogEntries(count){`,
		`function showDriveFolderGrantOverlay(agentID, workspaceKey)`,
		`function closeDriveFolderGrantOverlay(agentID, workspaceKey)`,
		`function applyDriveFolderGrantOverlay(agentID, workspaceKey)`,
		`Changes were made and will not be saved. Close without saving?`,
		`/agents/drive-folders/grants`,
		`<div class="card" id="agent-agt_test">`,
		`<h2 id="agentName_agt_test">OpenClaw</h2>`,
		`<strong>Location:</strong> <span id="agentLocation_agt_test">webchat</span>`,
		`<strong>Access status:</strong> <span id="agentStatus_agt_test" class="logging-status-enabled">Enabled</span>`,
		`<strong>Activity last 24 hours:</strong> 12 requests`,
		`<strong>Agent ID:</strong> agt_test`,
		`<strong>API key:</strong> <span id="agentTokenHint_agt_test">atk_tes...est</span>`,
		`onclick="return showAgentEditOverlay('agt_test','friendly_name')">Rename agent</button>`,
		`onclick="return showAgentEditOverlay('agt_test','default_location')">Edit location</button>`,
		`href="/logs?agent_id=agt_test"`,
		`id="agentToggleForm_agt_test" method="post" action="/agents/toggle" data-ajax-form="true" data-ajax-update="agent-toggle" data-flash-target="agentActionsFlash_agt_test" data-suppress-success-flash="true"`,
		`id="agentToggleButton_agt_test" class="btn warn" type="submit">Suspend access</button>`,
		`action="/agents/rotate" data-ajax-form="true" data-ajax-update="agent-rotate" data-flash-target="agentActionsFlash_agt_test" data-confirm="Rotate this agent API key? Existing installed skills for this agent will stop working until updated."`,
		`action="/agents/delete" data-ajax-form="true" data-ajax-update="agent-delete" data-flash-target="agentActionsFlash_agt_test" data-confirm="Delete this agent identity and revoke its Workspace access?"`,
		`<button class="btn warn" type="submit">Delete agent profile</button>`,
		`<div id="agentActionsFlash_agt_test" class="form-flash-slot" aria-live="polite"></div>`,
		`<form method="post" action="/agents/grants" class="log-settings-section" data-ajax-form="true" data-ajax-update="agent-grants" data-success-message="Workspace grants updated.">`,
		`<input type="hidden" name="agent_id" value="agt_test">`,
		`<table class="agent-grants-table">`,
		`<th class="agent-grants-access">Access <span class="risk-help"`,
		`Allow or remove this agent`,
		`<th>Workspaces <span class="risk-help"`,
		`The connected Workspace account this grant applies to.`,
		`<th class="agent-grants-policy">Policy <span class="risk-help"`,
		`The proxy policy used to decide which Google operations this agent can perform in this Workspace.`,
		`<th class="agent-grants-accountability">Agent accountability <span class="risk-help"`,
		`When enforced, this agent must provide a motive for requests to this Workspace.`,
		`The motive is written by the AI agent and is used only for logging purposes and agent activity reviews.`,
		`<th class="agent-grants-drive-folders">Allowed Drive folders <span class="risk-help"`,
		`The registered Drive folders this agent can access in this Workspace.`,
		`<td class="agent-grants-access"><label class="checkbox"><input type="checkbox" name="workspace" value="workspace@example.com" checked> Allowed</label></td>`,
		`<input type="checkbox" name="require_agent_motive_workspace_at_example_dot_com" checked> Enforce`,
		`<td class="agent-grants-workspace"><strong>Workspace</strong><br><small>workspace@example.com</small></td>`,
		`<td class="agent-grants-policy">`,
		`<select name="policy_workspace_at_example_dot_com" data-size-to-options="true">`,
		`<option value="system" title="Default system policy"`,
		`>Default system policy</option>`,
		`<td class="agent-grants-drive-folders">`,
		`<div id="driveFolderGrantCount_agt_test_workspace_at_example_dot_com" class="drive-folder-grant-count">1 folder allowed</div>`,
		`onclick="return showDriveFolderGrantOverlay('agt_test','workspace_at_example_dot_com')">View / Edit folders accesses</button>`,
		`id="driveFolderGrantOverlay_agt_test_workspace_at_example_dot_com" class="discovery-overlay" role="dialog" aria-modal="true"`,
		`Allowed Drive folders - Workspace`,
		`Select which registered Drive folders this agent can access in this Workspace.`,
		`The proxy checks these folder grants before it evaluates policy permissions for Drive, Docs, Sheets, and Slides requests.`,
		`<thead><tr><th>Access</th><th>Reference Name</th><th>Real Name</th><th>Google Drive link</th></tr></thead>`,
		`value="dfr_reports" data-folder-grant-checkbox data-saved="true" onchange="syncDriveFolderGrantOverlay(this.closest('.discovery-overlay'))" checked> Allowed`,
		`<td>Reports</td>`,
		`<td>Reports Real Name</td>`,
		`<a href="https://drive.google.com/drive/folders/reports" target="_blank" rel="noopener noreferrer">https://drive.google.com/drive/folders/reports</a>`,
		`value="dfr_archive" data-folder-grant-checkbox data-saved="false" onchange="syncDriveFolderGrantOverlay(this.closest('.discovery-overlay'))" > Allowed`,
		`<button class="btn secondary" type="button" onclick="return closeDriveFolderGrantOverlay('agt_test','workspace_at_example_dot_com')">Close</button>`,
		`<button class="btn" type="button" data-folder-grant-apply onclick="return applyDriveFolderGrantOverlay('agt_test','workspace_at_example_dot_com')" disabled>Apply</button>`,
		`<div class="log-settings-actions"><button class="btn" type="submit">Update Workspace grants</button></div>`,
		`<h3>Firewall</h3>`,
		`<strong>Firewall status:</strong> <span id="agentFirewallStatus_agt_test" class="logging-status-enabled">Enabled</span>`,
		`id="agentFirewallToggleForm_agt_test" method="post" action="/agents/firewall/toggle" data-ajax-form="true" data-ajax-update="agent-firewall-toggle" data-flash-target="agentFirewallFlash_agt_test" data-suppress-success-flash="true"`,
		`<button id="agentFirewallToggleButton_agt_test" class="btn warn" type="submit">Disable firewall</button>`,
		`When Firewall is enabled, only AI agents connecting from the allowed egress IP addresses below can communicate with the AI Workspace Proxy.`,
		`ignores HTTP headers such as X-Forwarded-For`,
		`onclick="return showAgentFirewallAddressOverlay('agt_test')"`,
		`<table id="agentFirewallTable_agt_test" class="agent-firewall-table" >`,
		`<td>IPv4</td>`,
		`<td>2026-06-16 10:00:00 UTC</td>`,
		`<td>203.0.113.0/24</td>`,
		`onclick="return deleteAgentFirewallRule('agt_test','afw_test')"`,
		`id="agentFirewallAddressOverlay_agt_test" class="discovery-overlay" role="dialog" aria-modal="true"`,
		`<small>You can also enter an IP range using CIDR slash notation.</small>`,
		`<button class="btn" type="submit">Add address</button>`,
		`<h3>Download skill</h3>`,
		`The skill package is dynamically produced for this agent identity, its location, API key, Workspace grants, and current policy settings`,
		`so the generated instructions stay focused and token-efficient`,
		`If you change this agent's settings, Workspace grants, or policies, the installed skill may need to be updated.`,
		`An agent already running this skill can update it when you ask.`,
		`<em>"Update your AI Workspace Proxy skill."</em>`,
		`id="agentSkillPlatform_agt_test" data-size-to-options="true"`,
		`startAgentSkillDownload('agt_test','agentSkillPlatform_agt_test','agentSkillDownloadButton_agt_test')`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("agents template missing %q", want)
		}
	}
	for _, removed := range []string{
		`<h2>Agents access</h2>`,
		`Agents access updated.`,
		`Agent identities and grants`,
		`At: webchat`,
		`Disable agent`,
		`Delete agent access`,
		`Delete agent</button>`,
		`<thead><tr><th>Workspace</th><th>Allow</th><th>Policy</th></tr></thead>`,
		`<thead><tr><th>Access</th><th>Workspaces</th><th>Policy</th></tr></thead>`,
		`Create agent identities, attach Workspace access, and choose the policy each agent uses for each Workspace.`,
	} {
		if strings.Contains(html, removed) {
			t.Fatalf("agents template should not include removed intro/success element %q", removed)
		}
	}
	assertOrderedSubstrings(t, html, []string{
		`<strong>Location:</strong> <span id="agentLocation_agt_test">webchat</span>`,
		`<strong>Access status:</strong> <span id="agentStatus_agt_test" class="logging-status-enabled">Enabled</span>`,
		`<strong>Activity last 24 hours:</strong> 12 requests`,
		`<strong>Agent ID:</strong> agt_test`,
		`<strong>API key:</strong> <span id="agentTokenHint_agt_test">atk_tes...est</span>`,
	})
	assertOrderedSubstrings(t, html, []string{
		`<h3>Download skill</h3>`,
		`<h3>Workspace grants</h3>`,
		`<h3>Firewall</h3>`,
	})
}

func TestAgentSidebarEmptyStateCopy(t *testing.T) {
	var out bytes.Buffer
	err := dashboardTemplate.Execute(&out, map[string]any{
		"AppName":       "AI Workspace Proxy",
		"User":          &User{Email: "owner@example.com", Name: "Owner"},
		"CSRFToken":     "csrf_test",
		"ActiveSection": "dashboard",
		"PageTitle":     "Dashboard",
		"ShowDashboard": true,
		"DashboardKPIs": []map[string]any{},
		"Workspaces":    []map[string]any{},
		"Agents":        []map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	html := out.String()
	for _, want := range []string{
		`<p class="sidebar-label">AI Agents Profiles &amp; Accesses</p>`,
		`<button class="btn sidebar-connect" type="button" onclick="return showAgentCreateOverlay()"><span class="sidebar-button-icon" aria-hidden="true"><svg viewBox="0 0 24 24" focusable="false"><circle cx="12" cy="12" r="9"></circle>`,
		`<span>Create an agent profile</span></button>`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("agent sidebar empty state missing %q", want)
		}
	}
	if strings.Contains(html, `No AI agent profiles created yet.`) {
		t.Fatal("agent sidebar empty state should not render legacy empty copy")
	}
}

func assertOrderedSubstrings(t *testing.T, value string, parts []string) {
	t.Helper()
	offset := 0
	for _, part := range parts {
		index := strings.Index(value[offset:], part)
		if index < 0 {
			t.Fatalf("expected %q after offset %d", part, offset)
		}
		offset += index + len(part)
	}
}
