package main

import (
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

func friendlyWorkspaceScopes(scopes string) []string {
	labels := map[string]string{
		"https://www.googleapis.com/auth/calendar":      "Google Calendar access",
		"https://www.googleapis.com/auth/drive":         "Google Drive access",
		"https://www.googleapis.com/auth/documents":     "Google Docs access",
		"https://www.googleapis.com/auth/gmail.labels":  "Gmail label management",
		"https://www.googleapis.com/auth/gmail.modify":  "Gmail email and draft management",
		"https://www.googleapis.com/auth/presentations": "Google Slides access",
		"https://www.googleapis.com/auth/spreadsheets":  "Google Sheets access",
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
func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	if r.URL.Path != "/" && r.URL.Path != "/settings" && r.URL.Path != "/policies" {
		writeError(w, http.StatusNotFound, "not_found", "page not found")
		return
	}
	user, _ := a.currentUserFromSession(r)
	if user == nil {
		_ = loginTemplate.Execute(w, map[string]any{"AppName": a.cfg.AppName})
		return
	}
	if err := a.ensureProxyToken(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
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
	showPolicies := r.URL.Path == "/policies"
	selectedWorkspace := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if showSettings || showPolicies {
		selectedWorkspace = ""
	}
	showDashboard := selectedWorkspace == "" && !showSettings && !showPolicies
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
		workspaceView = map[string]any{
			"Email":            selectedConn.MailboxEmail,
			"Name":             selectedConn.FriendlyName,
			"Selector":         selectedConn.MailboxEmail,
			"PolicyID":         selectedConn.PolicyID,
			"PolicyOptions":    a.policyOptionsForUser(user.ID, selectedConn.PolicyID),
			"Scopes":           friendlyWorkspaceScopes(selectedConn.Scopes),
			"ConnectedSince":   formatUserTime(selectedConn.CreatedAt, settings.Timezone),
			"ConnectionStatus": "Active",
			"ProxyTokenStatus": "Long-lived",
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
	_ = dashboardTemplate.Execute(w, map[string]any{
		"AppName":       a.cfg.AppName,
		"User":          user,
		"CSRFToken":     a.csrfTokenFromRequest(r),
		"ShowDashboard": showDashboard,
		"ShowSettings":  showSettings,
		"ShowPolicies":  showPolicies,
		"Settings": map[string]any{
			"Timezone":    settings.Timezone,
			"Timezones":   timezoneOptions(settings.Timezone),
			"CurrentTime": formatUserTime(nowUTC(), settings.Timezone),
		},
		"SettingsError":        r.URL.Query().Get("settings_error"),
		"SettingsSaved":        r.URL.Query().Get("settings_saved") == "1",
		"PolicyEditor":         a.policyEditorView(user.ID, r),
		"Workspaces":           workspaceNav,
		"WorkspaceConnected":   selectedConn != nil,
		"Workspace":            workspaceView,
		"WorkspaceError":       workspaceError,
		"WorkspacePolicyError": r.URL.Query().Get("workspace_policy_error"),
		"WorkspacePolicySaved": r.URL.Query().Get("workspace_policy_saved") == "1",
		"DriveFolders":         driveFolders,
		"FolderError":          r.URL.Query().Get("folder_error"),
	})
}
func (a *App) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	if !a.requireSessionCSRF(w, r) {
		return
	}
	timezone := strings.TrimSpace(r.FormValue("timezone"))
	if err := a.store.SaveUserTimezone(user.ID, timezone); err != nil {
		http.Redirect(w, r, "/settings?settings_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/settings?settings_saved=1", http.StatusFound)
}
