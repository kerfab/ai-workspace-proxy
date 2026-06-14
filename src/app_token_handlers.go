package main

import (
	"encoding/json"
	"net/http"
	"sort"
)

const (
	agentWorkspaceAPIConfigFilename = "agents-workspace-api-access.config.json"
	userBackendAPIConfigFilename    = "user-backend-api-access.config.json"
	agentWorkspaceAPIConfigPath     = "config/" + agentWorkspaceAPIConfigFilename
)

func (a *App) rotateProxyToken(userID string) error {
	rawToken, err := RandomToken("ptk_", 24)
	if err != nil {
		return err
	}
	enc, err := a.crypto.Encrypt(rawToken)
	if err != nil {
		return err
	}
	hint := rawToken[:8] + "..." + rawToken[len(rawToken)-6:]
	return a.store.SaveProxyToken(userID, enc, hint)
}

func (a *App) rotateUserBackendAPIToken(userID string) error {
	rawToken, err := RandomToken("ubk_", 24)
	if err != nil {
		return err
	}
	enc, err := a.crypto.Encrypt(rawToken)
	if err != nil {
		return err
	}
	hint := rawToken[:8] + "..." + rawToken[len(rawToken)-6:]
	return a.store.SaveUserBackendAPIToken(userID, enc, hint)
}

func (a *App) handleDownloadAgentWorkspaceAPIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	rec, err := a.store.GetProxyTokenRecord(user.ID)
	if err != nil || rec == nil {
		writeError(w, http.StatusNotFound, "token_error", "agent Workspace API key not found")
		return
	}
	raw, err := a.crypto.Decrypt(rec["token_enc"])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	config, err := a.agentWorkspaceAPIConfig(user.ID, raw, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "config_error", err.Error())
		return
	}
	writeDownloadJSON(w, agentWorkspaceAPIConfigFilename, config)
}

func (a *App) handleDownloadUserBackendAPIConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	rec, err := a.store.GetUserBackendAPITokenRecord(user.ID)
	if err != nil || rec == nil {
		writeError(w, http.StatusNotFound, "token_error", "user backend API token not found")
		return
	}
	raw, err := a.crypto.Decrypt(rec["token_enc"])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	writeDownloadJSON(w, userBackendAPIConfigFilename, map[string]any{
		"config_type":            "user_backend_api_access",
		"proxy_url":              a.cfg.BaseURL,
		"user_backend_api_token": raw,
	})
}

func (a *App) agentWorkspaceAPIConfig(userID, rawToken, skillPlatform string) (map[string]any, error) {
	conns, err := a.store.ListGmailConnections(userID)
	if err != nil {
		return nil, err
	}
	sort.Slice(conns, func(i, j int) bool {
		return conns[i].MailboxEmail < conns[j].MailboxEmail
	})
	workspaces := make([]map[string]string, 0, len(conns))
	for _, conn := range conns {
		workspaces = append(workspaces, map[string]string{
			"email": conn.MailboxEmail,
			"name":  conn.FriendlyName,
		})
	}
	config := map[string]any{
		"config_type": "agents_workspace_api_access",
		"proxy_url":   a.cfg.BaseURL,
		"proxy_token": rawToken,
		"workspaces":  workspaces,
	}
	if skillPlatform != "" {
		config["skill_platform"] = skillPlatform
	}
	return config, nil
}

func writeDownloadJSON(w http.ResponseWriter, filename string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
}

func (a *App) handleRotateAgentWorkspaceAPIKey(w http.ResponseWriter, r *http.Request) {
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
	if err := a.rotateProxyToken(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) handleRotateUserBackendAPIKey(w http.ResponseWriter, r *http.Request) {
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
	if err := a.rotateUserBackendAPIToken(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) ensureProxyToken(userID string) error {
	rec, err := a.store.GetProxyTokenRecord(userID)
	if err != nil {
		return err
	}
	if rec != nil {
		return nil
	}
	return a.rotateProxyToken(userID)
}

func (a *App) ensureUserBackendAPIToken(userID string) error {
	rec, err := a.store.GetUserBackendAPITokenRecord(userID)
	if err != nil {
		return err
	}
	if rec != nil {
		return nil
	}
	return a.rotateUserBackendAPIToken(userID)
}
