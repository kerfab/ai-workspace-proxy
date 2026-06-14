package main

import (
	"net/http"
	"net/url"
	"strings"
)

func (a *App) handleWorkspaceDisconnect(w http.ResponseWriter, r *http.Request) {
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
	conn, err := a.resolveWorkspaceFromRequest(user.ID, r)
	if err != nil {
		http.Redirect(w, r, "/?folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	if err := a.store.DeleteGmailConnection(user.ID, conn.MailboxEmail); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}
func (a *App) handleWorkspaceAccountUpdate(w http.ResponseWriter, r *http.Request) {
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
	conn, err := a.resolveWorkspaceFromRequest(user.ID, r)
	if err != nil {
		http.Redirect(w, r, "/?folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	friendlyName := strings.TrimSpace(r.FormValue("friendly_name"))
	if err := a.store.UpdateGmailConnectionFriendlyName(user.ID, conn.MailboxEmail, friendlyName); err != nil {
		http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail), http.StatusFound)
}
func (a *App) handleWorkspacePolicyUpdate(w http.ResponseWriter, r *http.Request) {
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
	conn, err := a.resolveWorkspaceFromRequest(user.ID, r)
	if err != nil {
		http.Redirect(w, r, "/?workspace_policy_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	policyID := strings.TrimSpace(r.FormValue("policy_id"))
	if err := a.store.UpdateGmailConnectionPolicy(user.ID, conn.MailboxEmail, policyID); err != nil {
		http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&workspace_policy_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&workspace_policy_saved=1", http.StatusFound)
}
func (a *App) handleWorkspaceOrder(w http.ResponseWriter, r *http.Request) {
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
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_form", err.Error())
		return
	}
	if err := a.store.SaveWorkspaceOrder(user.ID, r.Form["workspace"]); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
