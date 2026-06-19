// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"context"
	"embed"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"
)

//go:embed static
var staticAssets embed.FS

type App struct {
	cfg               *Config
	store             *Store
	crypto            *Crypto
	client            *http.Client
	lookupTXT         func(string) ([]string, error)
	lookupNS          func(string) ([]*net.NS, error)
	lookupIPAddr      func(context.Context, string) ([]net.IPAddr, error)
	lookupTXTAtServer func(context.Context, string, string) ([]string, error)
}

func NewApp(cfg *Config, store *Store, crypto *Crypto) *App {
	if store != nil && cfg != nil {
		store.SetOrganizationAdminVerificationKey(cfg.EncryptionKey)
	}
	return &App{
		cfg:               cfg,
		store:             store,
		crypto:            crypto,
		client:            &http.Client{Timeout: cfg.HTTPClientTimeout},
		lookupTXT:         net.LookupTXT,
		lookupNS:          net.LookupNS,
		lookupIPAddr:      net.DefaultResolver.LookupIPAddr,
		lookupTXTAtServer: lookupTXTAtServer,
	}
}

func lookupTXTAtServer(ctx context.Context, serverAddr, host string) ([]string, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, serverAddr)
		},
	}
	return resolver.LookupTXT(ctx, host)
}
func (a *App) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleIndex)
	mux.HandleFunc("/favicon.ico", a.handleFavicon)
	mux.HandleFunc("/admin", a.handleIndex)
	mux.HandleFunc("/admin/", a.handleIndex)
	mux.HandleFunc("/org-admin", a.handleIndex)
	mux.HandleFunc("/org-admin/domain/validate", a.handleOrganizationDomainValidate)
	mux.HandleFunc("/admin/domain/validate", a.handleOrganizationDomainValidate)
	mux.HandleFunc("/auth/google/login", a.handleGoogleLogin)
	mux.HandleFunc("/auth/google/callback", a.handleGoogleCallback)
	mux.HandleFunc("/auth/2fa", a.handleTwoFactorChallenge)
	mux.HandleFunc("/logout", a.handleLogout)
	mux.HandleFunc("/settings", a.handleIndex)
	mux.HandleFunc("/settings/2fa/enroll", a.handleIndex)
	mux.HandleFunc("/settings/update", a.handleSettingsUpdate)
	mux.HandleFunc("/settings/session-timeout", a.handleSessionTimeoutUpdate)
	mux.HandleFunc("/settings/2fa/start", a.handleTwoFactorSetupStart)
	mux.HandleFunc("/settings/2fa/confirm", a.handleTwoFactorSetupConfirm)
	mux.HandleFunc("/settings/2fa/cancel", a.handleTwoFactorSetupCancel)
	mux.HandleFunc("/settings/2fa/reset", a.handleTwoFactorReset)
	mux.HandleFunc("/settings/2fa/qr.png", a.handleTwoFactorQRCode)
	mux.HandleFunc("/agents", a.handleIndex)
	mux.HandleFunc("/agents/create", a.handleAgentCreate)
	mux.HandleFunc("/agents/update", a.handleAgentUpdate)
	mux.HandleFunc("/agents/grants", a.handleAgentGrantsUpdate)
	mux.HandleFunc("/agents/drive-folders/grants", a.handleAgentDriveFolderGrantsUpdate)
	mux.HandleFunc("/agents/rotate", a.handleAgentRotate)
	mux.HandleFunc("/agents/toggle", a.handleAgentToggle)
	mux.HandleFunc("/agents/skill-warning/dismiss", a.handleAgentSkillWarningDismiss)
	mux.HandleFunc("/agents/firewall/toggle", a.handleAgentFirewallToggle)
	mux.HandleFunc("/agents/firewall/add", a.handleAgentFirewallAdd)
	mux.HandleFunc("/agents/firewall/delete", a.handleAgentFirewallDelete)
	mux.HandleFunc("/agents/delete", a.handleAgentDelete)
	mux.HandleFunc("/logs", a.handleIndex)
	mux.HandleFunc("/api/logs/request", a.handleRequestLogsAPI)
	mux.HandleFunc("/api/logs/request/columns", a.handleRequestLogColumnsAPI)
	mux.HandleFunc("/api/logs/request/export", a.handleRequestLogExportAPI)
	mux.HandleFunc("/api/logs/request/view-settings", a.handleRequestLogViewSettingsAPI)
	mux.HandleFunc("/policies", a.handleIndex)
	mux.HandleFunc("/policies/save", a.handlePolicySave)
	mux.HandleFunc("/policies/delete", a.handlePolicyDelete)
	mux.HandleFunc("/policies/default", a.handlePolicyDefault)
	mux.HandleFunc("/auth/workspace/connect", a.handleWorkspaceConnect)
	mux.HandleFunc("/auth/workspace/refresh", a.handleWorkspaceAuthRefresh)
	mux.HandleFunc("/auth/workspace/reauth", a.handleWorkspaceReauth)
	mux.HandleFunc("/auth/workspace/callback", a.handleWorkspaceCallback)
	mux.HandleFunc("/auth/workspace/disconnect", a.handleWorkspaceDisconnect)
	mux.HandleFunc("/workspace/delete", a.handleWorkspaceDelete)
	mux.HandleFunc("/workspace/order", a.handleWorkspaceOrder)
	mux.HandleFunc("/workspace/account/update", a.handleWorkspaceAccountUpdate)
	mux.HandleFunc("/workspace/drive-folders/add", a.handleAddDriveFolder)
	mux.HandleFunc("/workspace/drive-folders/", a.handleDriveFolderRoutes)
	mux.HandleFunc("/api/drive-folders/tree", a.handleDriveFolderTreeAPI)
	mux.HandleFunc("/api/drive-folders/tree/refresh", a.handleRefreshDriveFolderTreeAPI)
	mux.HandleFunc("/api/agent-skill/install-token", a.handleCreateAgentSkillInstallToken)
	mux.HandleFunc("/api/agent-skill/download", a.handleDownloadAgentSkill)
	mux.HandleFunc("/api/config/agents-workspace-api-access/download", a.handleDownloadAgentWorkspaceAPIConfig)
	mux.HandleFunc("/api/config/user-backend-api-access/download", a.handleDownloadUserBackendAPIConfig)
	mux.HandleFunc("/api/config/user-backend-api-access/rotate", a.handleRotateUserBackendAPIKey)
	mux.HandleFunc("/api/user/workspaces", a.handleUserWorkspacesAPI)
	mux.HandleFunc("/api/user/drive-folders", a.handleUserDriveFoldersAPI)
	mux.HandleFunc("/api/user/agents", a.handleUserAgentsAPI)
	mux.HandleFunc("/api/user/agents/", a.handleUserAgentAPI)
	mux.HandleFunc("/api/user/policies", a.handleUserPoliciesAPI)
	mux.HandleFunc("/api/user/policies/", a.handleUserPolicyAPI)
	mux.HandleFunc("/admin/users", a.handleAdminUsers)
	mux.HandleFunc("/admin/users/", a.handleAdminUserRoutes)
	mux.HandleFunc("/admin/accounts-policy", a.handleAdminAccountsPolicy)
	mux.HandleFunc("/gmail.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/people.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/calendar.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/drive.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/docs.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/sheets.googleapis.com/", a.handleProxyRelay)
	mux.HandleFunc("/slides.googleapis.com/", a.handleProxyRelay)
	mux.Handle("/static/", a.staticFileHandler())
	return mux
}

func (a *App) handleFavicon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) staticFileHandler() http.Handler {
	staticFS, err := fs.Sub(staticAssets, "static")
	if err != nil {
		staticFS = staticAssets
	}
	fileServer := http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			w.Header().Set("Cache-Control", "public, max-age=300")
		case strings.HasSuffix(r.URL.Path, ".js"):
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			w.Header().Set("Cache-Control", "public, max-age=300")
		case strings.HasSuffix(r.URL.Path, ".png") ||
			strings.HasSuffix(r.URL.Path, ".jpg") ||
			strings.HasSuffix(r.URL.Path, ".jpeg") ||
			strings.HasSuffix(r.URL.Path, ".gif") ||
			strings.HasSuffix(r.URL.Path, ".webp") ||
			strings.HasSuffix(r.URL.Path, ".svg") ||
			strings.HasSuffix(r.URL.Path, ".ico"):
			w.Header().Set("Cache-Control", "public, max-age=86400")
		default:
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		fileServer.ServeHTTP(w, r)
	})
}
