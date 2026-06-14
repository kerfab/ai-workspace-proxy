package main

import (
	"net/http"
	"strings"
)

func (a *App) handleUserWorkspacesAPI(w http.ResponseWriter, r *http.Request) {
	user := a.requireUserBackendAPIUser(w, r)
	if user == nil {
		return
	}
	if r.URL.Path != "/api/user/workspaces" {
		writeError(w, http.StatusNotFound, "not_found", "workspace endpoint not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	workspaceSelector := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if workspaceSelector != "" {
		conn, err := a.store.ResolveGmailConnection(user.ID, workspaceSelector)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		if conn == nil {
			writeError(w, http.StatusNotFound, "workspace_not_found", "workspace not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"workspace": a.workspaceAPIView(user.ID, conn)})
		return
	}
	conns, err := a.store.ListGmailConnections(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	workspaces := make([]map[string]any, 0, len(conns))
	for i := range conns {
		workspaces = append(workspaces, a.workspaceAPIView(user.ID, &conns[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaces": workspaces})
}

func (a *App) workspaceAPIView(userID string, conn *GmailConnection) map[string]any {
	policyID := strings.TrimSpace(conn.PolicyID)
	if policyID == "" {
		policyID = systemPolicyID
	}
	return map[string]any{
		"email":       conn.MailboxEmail,
		"name":        conn.FriendlyName,
		"policy_id":   policyID,
		"policy_name": a.policyDisplayNameForUser(userID, policyID),
	}
}
