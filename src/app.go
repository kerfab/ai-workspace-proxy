package main

import "net/http"

type App struct {
	cfg       *Config
	store     *Store
	crypto    *Crypto
	deniedLog *DeniedLogger
	client    *http.Client
}

func NewApp(cfg *Config, store *Store, crypto *Crypto, deniedLog *DeniedLogger) *App {
	return &App{
		cfg:       cfg,
		store:     store,
		crypto:    crypto,
		deniedLog: deniedLog,
		client:    &http.Client{Timeout: cfg.HTTPClientTimeout},
	}
}
func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/auth/google/login", a.handleGoogleLogin)
	mux.HandleFunc("/auth/google/callback", a.handleGoogleCallback)
	mux.HandleFunc("/logout", a.handleLogout)
	mux.HandleFunc("/settings", a.handleIndex)
	mux.HandleFunc("/settings/update", a.handleSettingsUpdate)
	mux.HandleFunc("/policies", a.handleIndex)
	mux.HandleFunc("/policies/save", a.handlePolicySave)
	mux.HandleFunc("/policies/delete", a.handlePolicyDelete)
	mux.HandleFunc("/policies/default", a.handlePolicyDefault)
	mux.HandleFunc("/auth/workspace/connect", a.handleWorkspaceConnect)
	mux.HandleFunc("/auth/workspace/callback", a.handleWorkspaceCallback)
	mux.HandleFunc("/auth/workspace/disconnect", a.handleWorkspaceDisconnect)
	mux.HandleFunc("/workspace/order", a.handleWorkspaceOrder)
	mux.HandleFunc("/workspace/account/update", a.handleWorkspaceAccountUpdate)
	mux.HandleFunc("/workspace/policy/update", a.handleWorkspacePolicyUpdate)
	mux.HandleFunc("/workspace/drive-folders/add", a.handleAddDriveFolder)
	mux.HandleFunc("/workspace/drive-folders/", a.handleDriveFolderRoutes)
	mux.HandleFunc("/api/drive-folders/tree", a.handleDriveFolderTreeAPI)
	mux.HandleFunc("/api/drive-folders/tree/refresh", a.handleRefreshDriveFolderTreeAPI)
	mux.HandleFunc("/api/agent-skill/install-token", a.handleCreateAgentSkillInstallToken)
	mux.HandleFunc("/api/agent-skill/download", a.handleDownloadAgentSkill)
	mux.HandleFunc("/api/config/agents-workspace-api-access/download", a.handleDownloadAgentWorkspaceAPIConfig)
	mux.HandleFunc("/api/config/agents-workspace-api-access/rotate", a.handleRotateAgentWorkspaceAPIKey)
	mux.HandleFunc("/api/config/user-backend-api-access/download", a.handleDownloadUserBackendAPIConfig)
	mux.HandleFunc("/api/config/user-backend-api-access/rotate", a.handleRotateUserBackendAPIKey)
	mux.HandleFunc("/api/user/workspaces", a.handleUserWorkspacesAPI)
	mux.HandleFunc("/api/user/drive-folders", a.handleUserDriveFoldersAPI)
	mux.HandleFunc("/api/user/policies", a.handleUserPoliciesAPI)
	mux.HandleFunc("/api/user/policies/", a.handleUserPolicyAPI)
	mux.HandleFunc("/admin/users", a.handleAdminUsers)
	mux.HandleFunc("/admin/users/", a.handleAdminUserRoutes)
	mux.HandleFunc("/gmail.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/people.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/calendar.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/drive.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/docs.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/sheets.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/slides.googleapis.com/", a.handleProxyRelay)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = a.store.DeleteExpiredSessions()
		mux.ServeHTTP(w, r)
	})
}
