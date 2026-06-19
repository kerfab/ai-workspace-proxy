// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

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
		if wantsJSON(r) {
			writeError(w, http.StatusBadRequest, "workspace_error", err.Error())
			return
		}
		http.Redirect(w, r, "/?workspace_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	if err := a.store.DisconnectGmailConnectionOAuth(user.ID, conn.MailboxEmail); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	a.logWorkspaceAudit(r, user, conn.MailboxEmail, "workspace_auth_disconnected", conn.MailboxEmail, conn.FriendlyName, map[string]any{})
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":             "ok",
			"message":            "Workspace disconnected.",
			"workspace":          conn.MailboxEmail,
			"connection_status":  "Disconnected",
			"auth_connected":     false,
			"refresh_auth_label": "Refresh Google auth & scopes",
			"connect_label":      "Reconnect workspace",
		})
		return
	}
	http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail), http.StatusFound)
}

func (a *App) handleWorkspaceDelete(w http.ResponseWriter, r *http.Request) {
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
		if wantsJSON(r) {
			writeError(w, http.StatusBadRequest, "workspace_error", err.Error())
			return
		}
		http.Redirect(w, r, "/?workspace_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	if err := a.store.DeleteGmailConnection(user.ID, conn.MailboxEmail); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	a.logWorkspaceAudit(r, user, conn.MailboxEmail, "workspace_settings_deleted", conn.MailboxEmail, conn.FriendlyName, map[string]any{})
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"message":   "Workspace settings deleted.",
			"workspace": conn.MailboxEmail,
		})
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
		if wantsJSON(r) {
			writeError(w, http.StatusBadRequest, "workspace_error", err.Error())
			return
		}
		http.Redirect(w, r, "/?workspace_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	friendlyName := strings.TrimSpace(r.FormValue("friendly_name"))
	if err := a.store.UpdateGmailConnectionFriendlyName(user.ID, conn.MailboxEmail, friendlyName); err != nil {
		if wantsJSON(r) {
			writeError(w, http.StatusBadRequest, "workspace_error", err.Error())
			return
		}
		http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&workspace_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	a.logWorkspaceAudit(r, user, conn.MailboxEmail, "workspace_friendly_name_updated", conn.MailboxEmail, friendlyName, map[string]any{
		"old_name": conn.FriendlyName,
		"new_name": friendlyName,
	})
	agentIDs, _ := a.store.AgentIDsForWorkspace(user.ID, conn.MailboxEmail)
	skillUpdateReason := "Workspace \"" + conn.FriendlyName + "\" was renamed."
	_ = a.store.MarkAgentsSkillStale(user.ID, agentIDs, skillUpdateReason)
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                 "ok",
			"message":                "Workspace friendly name saved.",
			"workspace":              conn.MailboxEmail,
			"friendly_name":          friendlyName,
			"skill_update_agent_ids": agentIDs,
			"skill_update_reasons":   []string{skillUpdateReason},
		})
		return
	}
	http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail), http.StatusFound)
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
	a.logWorkspaceAudit(r, user, "", "workspace_order_updated", user.ID, "Workspace order", map[string]any{"workspace_order": r.Form["workspace"]})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}
