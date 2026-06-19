// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func friendlyWorkspaceScopes(scopes string) []string {
	labels := map[string]string{
		"https://www.googleapis.com/auth/calendar":                "Google Calendar access",
		"https://www.googleapis.com/auth/contacts.other.readonly": "Other contacts lookup",
		"https://www.googleapis.com/auth/contacts.readonly":       "Google Contacts lookup",
		"https://www.googleapis.com/auth/directory.readonly":      "Google Directory lookup",
		"https://www.googleapis.com/auth/drive":                   "Google Drive access",
		"https://www.googleapis.com/auth/documents":               "Google Docs access",
		"https://www.googleapis.com/auth/gmail.labels":            "Gmail label management",
		"https://www.googleapis.com/auth/gmail.modify":            "Gmail email and draft management",
		"https://www.googleapis.com/auth/presentations":           "Google Slides access",
		"https://www.googleapis.com/auth/spreadsheets":            "Google Sheets access",
	}
	seen := map[string]bool{}
	out := []string{}
	for _, scope := range strings.Fields(scopes) {
		label := labels[scope]
		if label == "" {
			label = strings.TrimPrefix(scope, "https://www.googleapis.com/auth/")
		}
		if label != "" && !seen[label] {
			seen[label] = true
			out = append(out, label)
		}
	}
	sort.Strings(out)
	return out
}
func formatUserTime(t time.Time, timezone string) string {
	loc, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		loc = time.UTC
	}
	return t.In(loc).Format("2006-01-02 15:04:05 MST")
}
func activeSectionForRequest(path, selectedWorkspace string) string {
	if strings.HasPrefix(path, "/admin/users/") {
		return "admin_user"
	}
	if strings.HasPrefix(path, "/admin/users") {
		return "admin_users"
	}
	if strings.HasPrefix(path, "/admin/accounts-policy") {
		return "admin_accounts_policy"
	}
	switch path {
	case "/admin", "/admin/", "/org-admin":
		return "admin"
	case "/settings":
		return "settings"
	case "/settings/2fa/enroll":
		return "settings"
	case "/agents":
		return "agents"
	case "/policies":
		return "policies"
	case "/logs":
		return "logs"
	}
	if strings.TrimSpace(selectedWorkspace) != "" {
		return "workspace"
	}
	return "dashboard"
}
func pageTitleForSection(section string) string {
	switch section {
	case "admin":
		return "Admin"
	case "admin_users":
		return "Manage accounts"
	case "admin_user":
		return "Manage account"
	case "admin_accounts_policy":
		return "Accounts Policy"
	case "settings":
		return "Settings"
	case "agents":
		return "Agents access"
	case "policies":
		return "Permissions editor"
	case "logs":
		return "Logs"
	case "workspace":
		return "Workspace"
	default:
		return "Dashboard"
	}
}

func pageSubtitleForSection(section string) string {
	switch section {
	case "admin":
		return "Verify your domain and access organization-wide administration controls."
	case "admin_users":
		return "Review organization user accounts, access status, roles, and recent activity."
	case "admin_user":
		return "Review this user account and manage its organization access."
	case "admin_accounts_policy":
		return "Set organization-wide defaults and enforcement rules for user accounts."
	case "settings":
		return "Manage interface preferences, session duration, and sign-in security."
	case "agents":
		return "Configure agent identities, Workspace grants, firewall rules, and skill access."
	case "policies":
		return "Review and edit the access policies that define what your agents may do."
	case "logs":
		return "Review request activity, audit events, and exportable forensic records."
	case "workspace":
		return "Manage one connected Workspace account, its Drive folders, and its proxy settings."
	default:
		return "See your proxy overview, current setup, and security guidance."
	}
}

func isOrganizationCustomer(user *User) bool {
	return user != nil && strings.TrimSpace(user.OrganizationID) != ""
}
func timezoneOptions(selected string) []map[string]any {
	selected = strings.TrimSpace(selected)
	zones := availableTimezones()
	out := make([]map[string]any, 0, len(zones))
	for _, zone := range zones {
		out = append(out, map[string]any{
			"Name":     zone,
			"Selected": zone == selected,
		})
	}
	return out
}

func (a *App) agentWorkspaceOptions(conns []GmailConnection) []map[string]any {
	out := make([]map[string]any, 0, len(conns))
	for _, conn := range conns {
		out = append(out, map[string]any{
			"Email":   conn.MailboxEmail,
			"Name":    conn.FriendlyName,
			"FormKey": formKey(conn.MailboxEmail),
		})
	}
	return out
}

func (a *App) agentAccessViews(userID string, conns []GmailConnection, selectedAgentID string, timezone string) []map[string]any {
	agents, err := a.store.ListAgents(userID)
	if err != nil {
		return nil
	}
	selectedAgentID = strings.TrimSpace(selectedAgentID)
	out := make([]map[string]any, 0, len(agents))
	for _, agent := range agents {
		grants, _ := a.store.ListAgentWorkspaceGrants(userID, agent.ID)
		grantByWorkspace := map[string]AgentWorkspaceGrant{}
		for _, grant := range grants {
			grantByWorkspace[normalizeEmail(grant.MailboxEmail)] = grant
		}
		workspaceRows := make([]map[string]any, 0, len(conns))
		for _, conn := range conns {
			grant, checked := grantByWorkspace[normalizeEmail(conn.MailboxEmail)]
			policyID := systemPolicyID
			if checked {
				policyID = grant.PolicyID
			}
			driveFolders, _ := a.store.ListDriveFolderRefs(userID, conn.MailboxEmail)
			driveFolderGrants, _ := a.store.ListAgentDriveFolderGrants(userID, agent.ID, conn.MailboxEmail)
			grantedDriveFolderIDs := map[string]bool{}
			for _, driveFolderGrant := range driveFolderGrants {
				grantedDriveFolderIDs[driveFolderGrant.FolderRefID] = true
			}
			driveFolderRows := make([]map[string]any, 0, len(driveFolders))
			allowedDriveFolderCount := 0
			for _, driveFolder := range driveFolders {
				folderChecked := grantedDriveFolderIDs[driveFolder.ID]
				if folderChecked {
					allowedDriveFolderCount++
				}
				driveFolderRows = append(driveFolderRows, map[string]any{
					"ID":            driveFolder.ID,
					"ReferenceName": driveFolder.ReferenceName,
					"FolderName":    driveFolder.FolderName,
					"FolderURL":     driveFolder.FolderURL,
					"Checked":       folderChecked,
				})
			}
			allowedDriveFolderCountLabel := countLabel(allowedDriveFolderCount, "folder allowed", "folders allowed")
			workspaceRows = append(workspaceRows, map[string]any{
				"Email":                        conn.MailboxEmail,
				"Name":                         conn.FriendlyName,
				"FormKey":                      formKey(conn.MailboxEmail),
				"Checked":                      checked,
				"RequireAgentMotive":           checked && grant.RequireAgentMotive,
				"PolicyOptions":                a.agentGrantPolicyOptionsForUser(userID, policyID),
				"DriveFolderRows":              driveFolderRows,
				"AllowedDriveFolderCount":      allowedDriveFolderCount,
				"AllowedDriveFolderCountLabel": allowedDriveFolderCountLabel,
			})
		}
		count, _ := a.store.CountRequestLogsByAgentSince(userID, agent.ID, nowUTC().Add(-24*time.Hour))
		firewallRules, _ := a.store.ListAgentFirewallRules(userID, agent.ID)
		firewallRuleRows := make([]map[string]any, 0, len(firewallRules))
		for _, rule := range firewallRules {
			firewallRuleRows = append(firewallRuleRows, map[string]any{
				"ID":          rule.ID,
				"Type":        rule.IPVersion,
				"AddressKind": rule.AddressKind,
				"Value":       rule.Value,
				"DateAdded":   formatUserTime(rule.CreatedAt, timezone),
			})
		}
		lastUsed := "Never"
		if !agent.LastUsedAt.IsZero() {
			lastUsed = formatUserTime(agent.LastUsedAt, "UTC")
		}
		out = append(out, map[string]any{
			"ID":                   agent.ID,
			"Name":                 agent.FriendlyName,
			"Location":             agent.DefaultLocation,
			"Enabled":              agent.Enabled,
			"TokenHint":            a.displayAgentTokenHint(agent),
			"FirewallEnabled":      agent.FirewallEnabled,
			"FirewallRules":        firewallRuleRows,
			"FirewallRuleCount":    len(firewallRuleRows),
			"LastUsed":             lastUsed,
			"Activity24Hours":      count,
			"Activity24HoursLabel": countLabel(count, "request", "requests"),
			"WorkspaceRows":        workspaceRows,
			"SkillStale":           agent.SkillStale,
			"SkillStaleReason":     agent.SkillStaleReason,
			"SkillStaleReasons":    splitSkillStaleReasons(agent.SkillStaleReason),
			"Active":               selectedAgentID != "" && agent.ID == selectedAgentID,
		})
	}
	return out
}

func (a *App) agentGrantPolicyOptionsForUser(userID, selectedPolicyID string) []map[string]any {
	options := a.policyOptionsForUser(userID, selectedPolicyID)
	for _, option := range options {
		name, _ := option["Name"].(string)
		option["DisplayName"] = truncateForSelectLabel(name, 80)
	}
	return options
}

func truncateForSelectLabel(value string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func selectedAgentViews(agents []map[string]any, selectedAgentID string) []map[string]any {
	selectedAgentID = strings.TrimSpace(selectedAgentID)
	if selectedAgentID == "" {
		return nil
	}
	for _, agent := range agents {
		if fmt.Sprint(agent["ID"]) == selectedAgentID {
			return []map[string]any{agent}
		}
	}
	return nil
}

func availableTimezones() []string {
	seen := map[string]bool{"UTC": true}
	for _, path := range []string{"/usr/share/zoneinfo/zone1970.tab", "/usr/share/zoneinfo/zone.tab"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) < 3 {
				continue
			}
			zone := strings.TrimSpace(parts[2])
			if zone != "" {
				seen[zone] = true
			}
		}
	}
	if len(seen) == 1 {
		for _, zone := range []string{
			"Africa/Johannesburg",
			"America/Argentina/Buenos_Aires",
			"America/Chicago",
			"America/Los_Angeles",
			"America/New_York",
			"America/Sao_Paulo",
			"Asia/Dubai",
			"Asia/Hong_Kong",
			"Asia/Singapore",
			"Asia/Tokyo",
			"Australia/Sydney",
			"Europe/London",
			"Europe/Paris",
			"Pacific/Auckland",
		} {
			seen[zone] = true
		}
	}
	zones := make([]string, 0, len(seen))
	for zone := range seen {
		zones = append(zones, zone)
	}
	sort.Strings(zones)
	for i, zone := range zones {
		if zone == "UTC" {
			copy(zones[1:i+1], zones[0:i])
			zones[0] = "UTC"
			break
		}
	}
	return zones
}

func largestPolicyPermissionCount(policies []UserPolicy) int {
	maxCount := len(SystemDefaultCapabilityKeys())
	for _, policy := range policies {
		if count := len(policy.EnabledCapabilities); count > maxCount {
			maxCount = count
		}
	}
	return maxCount
}

func dashboardKPILines(count int, singular, plural []string) []string {
	if count == 1 {
		return singular
	}
	return plural
}

func countLabel(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func formatDashboardKPIValue(value int) string {
	type unit struct {
		threshold float64
		suffix    string
	}
	units := []unit{
		{threshold: 1_000_000_000, suffix: "B"},
		{threshold: 1_000_000, suffix: "M"},
		{threshold: 1_000, suffix: "k"},
	}
	negative := value < 0
	absValue := value
	if absValue < 0 {
		absValue = -absValue
	}
	for _, candidate := range units {
		if float64(absValue) >= candidate.threshold {
			scaled := float64(absValue) / candidate.threshold
			text := fmt.Sprintf("%.1f", scaled)
			text = strings.TrimSuffix(strings.TrimSuffix(text, "0"), ".")
			if negative {
				return "-" + text + candidate.suffix
			}
			return text + candidate.suffix
		}
	}
	return fmt.Sprintf("%d", value)
}

func dashboardKPI(value int, singular, plural []string) map[string]any {
	return map[string]any{
		"Value": formatDashboardKPIValue(value),
		"Lines": dashboardKPILines(value, singular, plural),
	}
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	if r.URL.Path == "/admin" {
		http.Redirect(w, r, "/admin/", http.StatusFound)
		return
	}
	if r.URL.Path != "/" && r.URL.Path != "/org-admin" && r.URL.Path != "/admin/" && r.URL.Path != "/admin/accounts-policy" && r.URL.Path != "/settings" && r.URL.Path != "/settings/2fa/enroll" && r.URL.Path != "/agents" && r.URL.Path != "/policies" && r.URL.Path != "/logs" {
		writeError(w, http.StatusNotFound, "not_found", "page not found")
		return
	}
	session, user, err := a.sessionAndUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	if user == nil || session == nil {
		_ = loginTemplate.Execute(w, map[string]any{"AppName": a.cfg.AppName})
		return
	}
	if !session.SecondFactorVerified {
		http.Redirect(w, r, "/auth/2fa", http.StatusFound)
		return
	}
	mustEnrollTwoFactor, err := a.userMustCompleteOrganizationTwoFactor(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "organization_policy_error", err.Error())
		return
	}
	if mustEnrollTwoFactor && !pathAllowsOrganizationTwoFactorEnrollment(r.URL.Path) {
		http.Redirect(w, r, twoFactorEnrollmentURLForRequest(r), http.StatusFound)
		return
	}
	canAccessOrganizationAdmin, err := a.canAccessOrganizationAdmin(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "organization_admin_error", err.Error())
		return
	}
	if (r.URL.Path == "/org-admin" || r.URL.Path == "/admin/") && !canAccessOrganizationAdmin {
		writeError(w, http.StatusNotFound, "not_found", "page not found")
		return
	}
	if err := a.ensureUserBackendAPIToken(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	settings, err := a.store.GetUserSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_error", err.Error())
		return
	}
	conns, _ := a.store.ListGmailConnections(user.ID)
	showSettings := r.URL.Path == "/settings"
	showTwoFactorEnroll := r.URL.Path == "/settings/2fa/enroll"
	showAgents := r.URL.Path == "/agents"
	showPolicies := r.URL.Path == "/policies"
	showLogs := r.URL.Path == "/logs"
	showOrgAdmin := r.URL.Path == "/org-admin" || r.URL.Path == "/admin/"
	selectedAgentID := strings.TrimSpace(r.URL.Query().Get("agent"))
	selectedWorkspace := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if showSettings || showTwoFactorEnroll || showAgents || showPolicies || showLogs || showOrgAdmin {
		selectedWorkspace = ""
	}
	showDashboard := selectedWorkspace == "" && !showSettings && !showTwoFactorEnroll && !showAgents && !showPolicies && !showLogs && !showOrgAdmin
	activeSection := activeSectionForRequest(r.URL.Path, selectedWorkspace)
	pageTitle := pageTitleForSection(activeSection)
	pageSubtitle := pageSubtitleForSection(activeSection)
	if showTwoFactorEnroll {
		pageTitle = "Two-factor enrollment"
		pageSubtitle = "Complete authenticator setup before continuing."
	}
	var selectedConn *GmailConnection
	workspaceError := ""
	if selectedWorkspace != "" {
		selectedConn, _ = a.store.ResolveGmailConnection(user.ID, selectedWorkspace)
		if selectedConn == nil {
			workspaceError = "Workspace not found."
		}
	}
	workspaceView := map[string]any{}
	driveFolders := []AllowedDriveFolder{}
	if selectedConn != nil {
		authConnected := workspaceHasStoredOAuth(selectedConn)
		connectionStatus := "Disconnected"
		if authConnected {
			connectionStatus = "Active"
		}
		workspaceView = map[string]any{
			"Email":            selectedConn.MailboxEmail,
			"Name":             selectedConn.FriendlyName,
			"Selector":         selectedConn.MailboxEmail,
			"Scopes":           friendlyWorkspaceScopes(selectedConn.Scopes),
			"ConnectedSince":   formatUserTime(selectedConn.CreatedAt, settings.Timezone),
			"ConnectionStatus": connectionStatus,
			"AuthConnected":    authConnected,
		}
		driveFolders, _ = a.store.ListDriveFolderRefs(user.ID, selectedConn.MailboxEmail)
	}
	workspaceNav := make([]map[string]any, 0, len(conns))
	for _, conn := range conns {
		workspaceNav = append(workspaceNav, map[string]any{
			"Email":  conn.MailboxEmail,
			"Name":   conn.FriendlyName,
			"Active": selectedConn != nil && conn.MailboxEmail == selectedConn.MailboxEmail,
		})
	}
	agents := a.agentAccessViews(user.ID, conns, selectedAgentID, settings.Timezone)
	selectedAgents := selectedAgentViews(agents, selectedAgentID)
	if showAgents && selectedAgentID != "" && len(selectedAgents) == 0 && r.URL.Query().Get("agents_error") == "" {
		// Keep the page useful if a stale URL points to a deleted or unknown agent.
		selectedAgentID = ""
	}
	selectedAgentName := ""
	if len(selectedAgents) == 1 {
		selectedAgentName = fmt.Sprint(selectedAgents[0]["Name"])
	}
	driveFolderCount, _ := a.store.CountVisibleDriveFolders(user.ID)
	agentRequestsLast7Days, _ := a.store.CountAgentRequestLogsSince(user.ID, nowUTC().Add(-7*24*time.Hour))
	policies, _ := a.store.ListUserPolicies(user.ID)
	largestPolicyPermissions := largestPolicyPermissionCount(policies)
	loggingSettings, err := a.store.GetUserLoggingSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "logging_settings_error", err.Error())
		return
	}
	loggingSettingsView := *loggingSettings
	loggingSettingsView.RetentionDays = effectiveLoggingRetentionDays(user, loggingSettingsView.RetentionDays)
	loggingRetentionUnit := "days"
	if loggingSettingsView.RetentionDays == 1 {
		loggingRetentionUnit = "day"
	}
	twoFactorSettings, err := a.store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_error", err.Error())
		return
	}
	organizationTwoFactorRequired, err := a.organizationRequiresUserTwoFactor(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "organization_policy_error", err.Error())
		return
	}
	if organizationTwoFactorRequired && (twoFactorSettings == nil || (!twoFactorSettings.Enabled && strings.TrimSpace(twoFactorSettings.PendingSecretEnc) == "")) {
		if err := a.ensureUserTwoFactorPendingSecret(user); err != nil {
			writeError(w, http.StatusInternalServerError, "two_factor_error", err.Error())
			return
		}
		twoFactorSettings, err = a.store.GetUserTwoFactorSettings(user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "two_factor_error", err.Error())
			return
		}
	}
	organizationPolicy, err := a.organizationAccountsPolicyForUser(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "organization_policy_error", err.Error())
		return
	}
	twoFactorCanReset := twoFactorSettings != nil && twoFactorSettings.Enabled
	if organizationTwoFactorRequired {
		twoFactorCanReset = false
	} else if organizationPolicy != nil && !organizationPolicy.AllowUserTwoFactorReset {
		twoFactorCanReset = false
	}
	twoFactorStatus := "Disabled"
	twoFactorPending := false
	twoFactorManualSecret := ""
	twoFactorPendingSetAt := ""
	twoFactorBackURL := safeRelativeRedirectPath(r.URL.Query().Get("back"))
	if twoFactorBackURL == "" {
		twoFactorBackURL = "/"
	}
	if twoFactorSettings != nil {
		twoFactorPending = !twoFactorSettings.Enabled && strings.TrimSpace(twoFactorSettings.PendingSecretEnc) != ""
		if twoFactorSettings.Enabled {
			twoFactorStatus = "Enabled"
		} else if twoFactorPending {
			twoFactorStatus = "Setup in progress"
		}
		if twoFactorPending {
			twoFactorManualSecret, _ = a.crypto.Decrypt(twoFactorSettings.PendingSecretEnc)
			twoFactorPendingSetAt = formatUserTime(twoFactorSettings.PendingSecretSetAt, settings.Timezone)
		}
	}
	logRangeStart, logRangeEnd := logRetentionBounds(settings.Timezone, loggingSettingsView.RetentionDays)
	organizationAdminView := map[string]any{}
	if showOrgAdmin {
		organizationAdminView, err = a.organizationAdminView(user, settings.Timezone)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "organization_admin_error", err.Error())
			return
		}
	}
	renderDashboardResponse(w, r, map[string]any{
		"AppName":             a.cfg.AppName,
		"User":                user,
		"CSRFToken":           a.csrfTokenFromRequest(r),
		"ActiveSection":       activeSection,
		"AdminMode":           activeSection == "admin",
		"PageTitle":           pageTitle,
		"PageSubtitle":        pageSubtitle,
		"ShowDashboard":       showDashboard,
		"ShowOrgAdmin":        showOrgAdmin,
		"ShowSettings":        showSettings,
		"ShowTwoFactorEnroll": showTwoFactorEnroll,
		"ShowAgents":          showAgents,
		"ShowPolicies":        showPolicies,
		"ShowLogs":            showLogs,
		"DashboardKPIs": []map[string]any{
			dashboardKPI(len(conns), []string{"Workspace Connected"}, []string{"Workspaces Connected"}),
			dashboardKPI(driveFolderCount, []string{"Visible Drive Folder"}, []string{"Visible Drive Folders"}),
			dashboardKPI(len(agents), []string{"Agent Configured"}, []string{"Agents Configured"}),
			dashboardKPI(largestPolicyPermissions, []string{"Permissions", "(Largest Policy)"}, []string{"Permissions", "(Largest Policy)"}),
			dashboardKPI(agentRequestsLast7Days, []string{"Agent Request", "(Last 7 Days)"}, []string{"Agent Requests", "(Last 7 Days)"}),
		},
		"Settings": map[string]any{
			"Timezone":               settings.Timezone,
			"Timezones":              timezoneOptions(settings.Timezone),
			"CurrentTime":            formatUserTime(nowUTC(), settings.Timezone),
			"SessionTimeoutHours":    sessionTimeoutHours(settings),
			"SessionStartedAt":       formatUserTime(session.CreatedAt, settings.Timezone),
			"SessionExpiresAt":       formatUserTime(session.ExpiresAt, settings.Timezone),
			"SessionExpiresAtUnixMS": session.ExpiresAt.UnixMilli(),
		},
		"SettingsError": r.URL.Query().Get("settings_error"),
		"SettingsSaved": r.URL.Query().Get("settings_saved") == "1",
		"TwoFactor": map[string]any{
			"Enabled":         twoFactorSettings != nil && twoFactorSettings.Enabled,
			"Pending":         twoFactorPending,
			"Status":          twoFactorStatus,
			"ManualSecret":    twoFactorManualSecret,
			"AppName":         a.twoFactorAppName(),
			"AccountEmail":    user.Email,
			"QRCodeURL":       "/settings/2fa/qr.png",
			"Error":           r.URL.Query().Get("two_factor_error"),
			"PendingSetAt":    twoFactorPendingSetAt,
			"OrgManaged":      organizationTwoFactorRequired,
			"CanStart":        !organizationTwoFactorRequired && !twoFactorPending && (twoFactorSettings == nil || !twoFactorSettings.Enabled),
			"CanCancel":       !organizationTwoFactorRequired,
			"CanReset":        twoFactorCanReset,
			"HasEnrollment":   twoFactorSettings != nil && twoFactorSettings.Enabled,
			"HasPendingSetup": twoFactorPending,
			"BackURL":         twoFactorBackURL,
		},
		"Agents":                     agents,
		"SelectedAgents":             selectedAgents,
		"SelectedAgentID":            selectedAgentID,
		"SelectedAgentName":          selectedAgentName,
		"IsOrganizationCustomer":     isOrganizationCustomer(user),
		"CanAccessOrganizationAdmin": canAccessOrganizationAdmin,
		"AgentWorkspaceOptions":      a.agentWorkspaceOptions(conns),
		"AgentsError":                r.URL.Query().Get("agents_error"),
		"AgentsSaved":                r.URL.Query().Get("agents_saved") == "1",
		"AgentsDeleted":              r.URL.Query().Get("agents_deleted") == "1",
		"OrganizationAdmin":          organizationAdminView,
		"OrganizationAdminError":     r.URL.Query().Get("org_admin_error"),
		"LoggingSettings":            &loggingSettingsView,
		"LoggingRetentionUnit":       loggingRetentionUnit,
		"LogRangeStart":              logRangeStart.Format(time.RFC3339Nano),
		"LogRangeEnd":                logRangeEnd.Format(time.RFC3339Nano),
		"LogRangeStartDate":          formatLogDateKey(logRangeStart, settings.Timezone),
		"LogRangeEndDate":            formatLogDateKey(logRangeEnd, settings.Timezone),
		"PolicyEditor":               a.policyEditorView(user.ID, r),
		"Workspaces":                 workspaceNav,
		"WorkspaceConnected":         selectedConn != nil,
		"Workspace":                  workspaceView,
		"WorkspaceError":             workspaceError,
		"WorkspaceAccountError":      r.URL.Query().Get("workspace_error"),
		"DriveFolders":               driveFolders,
		"FolderError":                r.URL.Query().Get("folder_error"),
	})
}
func (a *App) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	session, user, err := a.sessionAndUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	if session == nil || user == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if !session.SecondFactorVerified {
		http.Redirect(w, r, "/auth/2fa", http.StatusFound)
		return
	}
	if !a.requireSessionCSRF(w, r) {
		return
	}
	timezone := strings.TrimSpace(r.FormValue("timezone"))
	previous, _ := a.store.GetUserSettings(user.ID)
	if err := a.store.SaveUserTimezone(user.ID, timezone); err != nil {
		if wantsJSON(r) {
			writeError(w, http.StatusBadRequest, "settings_error", err.Error())
			return
		}
		http.Redirect(w, r, "/settings?settings_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	previousTimezone := ""
	if previous != nil {
		previousTimezone = previous.Timezone
	}
	a.logUserAudit(r, user, "user_settings_updated", "timezone", "Timezone", map[string]any{
		"field":          "timezone",
		"previous_value": previousTimezone,
		"new_value":      timezone,
		"source":         requestSource(r),
	})
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                     "ok",
			"message":                    "Settings saved.",
			"timezone":                   timezone,
			"current_time":               formatUserTime(nowUTC(), timezone),
			"session_started_at":         formatUserTime(session.CreatedAt, timezone),
			"session_expires_at":         formatUserTime(session.ExpiresAt, timezone),
			"session_expires_at_unix_ms": session.ExpiresAt.UnixMilli(),
		})
		return
	}
	http.Redirect(w, r, "/settings?settings_saved=1", http.StatusFound)
}

func (a *App) handleSessionTimeoutUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	session, user, err := a.sessionAndUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	if session == nil || user == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if !session.SecondFactorVerified {
		http.Redirect(w, r, "/auth/2fa", http.StatusFound)
		return
	}
	if !a.requireSessionCSRF(w, r) {
		return
	}
	rawValue := strings.TrimSpace(r.FormValue("session_timeout_hours"))
	sessionTimeoutHoursValue, err := strconv.Atoi(rawValue)
	if err != nil {
		err = fmt.Errorf("session timeout must be a whole number of hours")
	}
	if err == nil && sessionTimeoutHoursValue < 1 {
		err = fmt.Errorf("session timeout must be at least 1 hour")
	}
	if err == nil && sessionTimeoutHoursValue > maxUserSessionTimeoutHours {
		err = fmt.Errorf("session timeout cannot exceed %d hours", maxUserSessionTimeoutHours)
	}
	if err != nil {
		if wantsJSON(r) {
			writeError(w, http.StatusBadRequest, "settings_error", err.Error())
			return
		}
		http.Redirect(w, r, "/settings?settings_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	previous, _ := a.store.GetUserSettings(user.ID)
	if err := a.store.SaveUserSessionTimeoutHours(user.ID, sessionTimeoutHoursValue); err != nil {
		if wantsJSON(r) {
			writeError(w, http.StatusBadRequest, "settings_error", err.Error())
			return
		}
		http.Redirect(w, r, "/settings?settings_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	updatedSettings, err := a.store.GetUserSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_error", err.Error())
		return
	}
	expiresAt := sessionExpiryFromStart(session.CreatedAt, updatedSettings, a.cfg.SessionTTL)
	if err := a.store.UpdateSessionExpiry(session.TokenHash, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	if cookie, cookieErr := r.Cookie(a.cfg.SessionCookieName); cookieErr == nil && strings.TrimSpace(cookie.Value) != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     a.cfg.SessionCookieName,
			Value:    cookie.Value,
			Path:     "/",
			HttpOnly: true,
			Secure:   a.cfg.CookieSecure,
			SameSite: http.SameSiteLaxMode,
			Expires:  expiresAt,
		})
	}
	previousTimeout := defaultUserSessionTimeoutHours
	if previous != nil {
		previousTimeout = sessionTimeoutHours(previous)
	}
	a.logUserAudit(r, user, "user_settings_updated", "session_timeout_hours", "User Interface Session Timeout", map[string]any{
		"field":           "session_timeout_hours",
		"previous_value":  previousTimeout,
		"new_value":       sessionTimeoutHoursValue,
		"session_started": session.CreatedAt.Format(time.RFC3339Nano),
		"session_expires": expiresAt.Format(time.RFC3339Nano),
		"source":          requestSource(r),
	})
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                     "ok",
			"session_timeout_hours":      sessionTimeoutHoursValue,
			"session_started_at":         formatUserTime(session.CreatedAt, updatedSettings.Timezone),
			"session_expires_at":         formatUserTime(expiresAt, updatedSettings.Timezone),
			"session_expires_at_unix_ms": expiresAt.UnixMilli(),
		})
		return
	}
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func requestSource(r *http.Request) string {
	if wantsJSON(r) {
		return "api"
	}
	return "ui"
}

func maxLoggingRetentionDays(user *User) int {
	if user != nil && user.IsAdmin {
		return 365
	}
	return 7
}

func effectiveLoggingRetentionDays(user *User, retentionDays int) int {
	if user == nil || !user.IsAdmin {
		return 7
	}
	maxRetention := maxLoggingRetentionDays(user)
	if retentionDays < 1 || retentionDays > maxRetention {
		return maxRetention
	}
	return retentionDays
}
