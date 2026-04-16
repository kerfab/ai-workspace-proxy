package main

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	loginStatePurpose     = "proxy_login"
	workspaceStatePurpose = "workspace_connect"
)

type App struct {
	cfg       *Config
	store     *Store
	crypto    *Crypto
	policy    *PolicyEngine
	deniedLog *DeniedLogger
	client    *http.Client
}

type GoogleTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	IDToken      string `json:"id_token"`
}

type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

type GmailProfile struct {
	EmailAddress string `json:"emailAddress"`
}

type relayTarget struct {
	serviceLabel   string
	upstreamBase   string
	normalizedPath string
}

func NewApp(cfg *Config, store *Store, crypto *Crypto, policy *PolicyEngine, deniedLog *DeniedLogger) *App {
	return &App{
		cfg:       cfg,
		store:     store,
		crypto:    crypto,
		policy:    policy,
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
	mux.HandleFunc("/auth/workspace/connect", a.handleWorkspaceConnect)
	mux.HandleFunc("/auth/workspace/callback", a.handleWorkspaceCallback)
	mux.HandleFunc("/auth/workspace/disconnect", a.handleWorkspaceDisconnect)
	mux.HandleFunc("/auth/gmail/connect", a.handleWorkspaceConnect)
	mux.HandleFunc("/auth/gmail/callback", a.handleWorkspaceCallback)
	mux.HandleFunc("/auth/gmail/disconnect", a.handleWorkspaceDisconnect)
	mux.HandleFunc("/workspace/drive-folders/add", a.handleAddDriveFolder)
	mux.HandleFunc("/workspace/drive-folders/", a.handleDriveFolderRoutes)
	mux.HandleFunc("/api/token/reveal", a.handleRevealToken)
	mux.HandleFunc("/api/token/rotate", a.handleRotateToken)
	mux.HandleFunc("/admin/users", a.handleAdminUsers)
	mux.HandleFunc("/admin/users/", a.handleAdminUserRoutes)
	mux.HandleFunc("/gmail.googleapis.com/", a.handleProxyRelay)
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

func (a *App) workspaceScopes() []string {
	return []string{
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/gmail.labels",
		"https://www.googleapis.com/auth/calendar.readonly",
		"https://www.googleapis.com/auth/drive",
		"https://www.googleapis.com/auth/documents",
		"https://www.googleapis.com/auth/spreadsheets",
		"https://www.googleapis.com/auth/presentations",
	}
}

func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	user, _ := a.currentUserFromSession(r)
	if user == nil {
		_ = loginTemplate.Execute(w, map[string]any{"AppName": a.cfg.AppName})
		return
	}
	conn, _ := a.store.GetGmailConnection(user.ID)
	rec, _ := a.store.GetProxyTokenRecord(user.ID)
	hint := "Token available"
	if rec != nil && rec["token_hint"] != "" {
		hint = rec["token_hint"]
	}
	connected := conn != nil
	workspaceView := map[string]any{}
	driveFolders, _ := a.store.ListDriveFolderRefs(user.ID)
	if connected {
		workspaceView = map[string]any{
			"AccountEmail":     conn.MailboxEmail,
			"Scopes":           conn.Scopes,
			"ConnectedSince":   conn.CreatedAt.Format(time.RFC3339),
			"ConnectionStatus": "Active",
			"ProxyTokenStatus": "Long-lived",
		}
	}
	_ = dashboardTemplate.Execute(w, map[string]any{
		"AppName":            a.cfg.AppName,
		"BaseURL":            a.cfg.BaseURL,
		"User":               user,
		"TokenHint":          hint,
		"WorkspaceConnected": connected,
		"Workspace":          workspaceView,
		"DriveFolders":       driveFolders,
		"FolderError":        r.URL.Query().Get("folder_error"),
	})
}

func (a *App) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := RandomToken("st_", 16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "state_error", err.Error())
		return
	}
	if err := a.store.SaveOAuthState(SHA256Hex(state), loginStatePurpose, "", nowUTC().Add(15*time.Minute)); err != nil {
		writeError(w, http.StatusInternalServerError, "state_error", err.Error())
		return
	}
	q := url.Values{}
	q.Set("client_id", a.cfg.WorkspaceClientID)
	q.Set("redirect_uri", a.cfg.LoginRedirectURI())
	q.Set("response_type", "code")
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("access_type", "online")
	http.Redirect(w, r, googleAuthURL+"?"+q.Encode(), http.StatusFound)
}

func (a *App) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		writeError(w, http.StatusBadRequest, "oauth_error", "missing code or state")
		return
	}
	_, ok, err := a.store.ConsumeOAuthState(SHA256Hex(state), loginStatePurpose)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "oauth_error", err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusBadRequest, "oauth_error", "invalid or expired state")
		return
	}
	tokenResp, err := a.exchangeCodeForToken(code, a.cfg.WorkspaceClientID, a.cfg.WorkspaceClientSecret, a.cfg.LoginRedirectURI())
	if err != nil {
		writeError(w, http.StatusBadGateway, "oauth_error", err.Error())
		return
	}
	userInfo, err := a.fetchUserInfo(tokenResp.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "userinfo_error", err.Error())
		return
	}
	email := normalizeEmail(userInfo.Email)
	if !userInfo.EmailVerified {
		writeError(w, http.StatusForbidden, "login_denied", "Google account email is not verified")
		return
	}
	if !a.cfg.IsEmailAllowed(email) {
		writeError(w, http.StatusForbidden, "login_denied", "email domain is not allowed")
		return
	}
	isAdmin := a.cfg.AdminEmails[email]
	user, err := a.store.CreateOrUpdateUser(email, userInfo.Name, userInfo.Picture, isAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user_error", err.Error())
		return
	}
	if user.IsSuspended {
		writeError(w, http.StatusForbidden, "user_suspended", "user is suspended")
		return
	}
	if err := a.ensureProxyToken(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	sessionRaw, err := RandomToken("pst_", 24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	if err := a.store.CreateSession(user.ID, SHA256Hex(sessionRaw), nowUTC().Add(a.cfg.SessionTTL)); err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     a.cfg.SessionCookieName,
		Value:    sessionRaw,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  nowUTC().Add(a.cfg.SessionTTL),
	})
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	if cookie, err := r.Cookie(a.cfg.SessionCookieName); err == nil {
		_ = a.store.DeleteSessionByTokenHash(SHA256Hex(cookie.Value))
		http.SetCookie(w, &http.Cookie{
			Name:     a.cfg.SessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   a.cfg.CookieSecure,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
		})
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) handleWorkspaceConnect(w http.ResponseWriter, r *http.Request) {
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	state, err := RandomToken("st_", 16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "state_error", err.Error())
		return
	}
	if err := a.store.SaveOAuthState(SHA256Hex(state), workspaceStatePurpose, user.ID, nowUTC().Add(15*time.Minute)); err != nil {
		writeError(w, http.StatusInternalServerError, "state_error", err.Error())
		return
	}
	q := url.Values{}
	q.Set("client_id", a.cfg.WorkspaceClientID)
	q.Set("redirect_uri", a.cfg.WorkspaceRedirectURI())
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(a.workspaceScopes(), " "))
	q.Set("state", state)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	http.Redirect(w, r, googleAuthURL+"?"+q.Encode(), http.StatusFound)
}

func (a *App) handleWorkspaceCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		writeError(w, http.StatusBadRequest, "oauth_error", "missing code or state")
		return
	}
	userID, ok, err := a.store.ConsumeOAuthState(SHA256Hex(state), workspaceStatePurpose)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "oauth_error", err.Error())
		return
	}
	if !ok || userID == "" {
		writeError(w, http.StatusBadRequest, "oauth_error", "invalid or expired state")
		return
	}
	user, err := a.store.FindUserByID(userID)
	if err != nil || user == nil {
		writeError(w, http.StatusUnauthorized, "user_error", "user not found")
		return
	}
	tokenResp, err := a.exchangeCodeForToken(code, a.cfg.WorkspaceClientID, a.cfg.WorkspaceClientSecret, a.cfg.WorkspaceRedirectURI())
	if err != nil {
		writeError(w, http.StatusBadGateway, "oauth_error", err.Error())
		return
	}
	existing, _ := a.store.GetGmailConnection(user.ID)
	refreshToken := tokenResp.RefreshToken
	if refreshToken == "" && existing != nil {
		refreshToken, _ = a.crypto.Decrypt(existing.RefreshTokenEnc)
	}
	if refreshToken == "" {
		writeError(w, http.StatusBadGateway, "oauth_error", "Google did not return a refresh token")
		return
	}
	profile, err := a.fetchGmailProfile(tokenResp.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "workspace_error", err.Error())
		return
	}
	accountEmail := normalizeEmail(profile.EmailAddress)
	if accountEmail != normalizeEmail(user.Email) {
		writeError(w, http.StatusForbidden, "workspace_connect_denied", "connected Google Workspace account must match the signed-in proxy user email")
		return
	}
	accessEnc, err := a.crypto.Encrypt(tokenResp.AccessToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "crypto_error", err.Error())
		return
	}
	refreshEnc, err := a.crypto.Encrypt(refreshToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "crypto_error", err.Error())
		return
	}
	scope := tokenResp.Scope
	if scope == "" {
		scope = strings.Join(a.workspaceScopes(), " ")
	}
	conn := &GmailConnection{
		UserID:          user.ID,
		MailboxEmail:    accountEmail,
		Scopes:          scope,
		AccessTokenEnc:  accessEnc,
		RefreshTokenEnc: refreshEnc,
		TokenExpiry:     nowUTC().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}
	if err := a.store.SaveGmailConnection(conn); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) handleWorkspaceDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	if err := a.store.DeleteGmailConnection(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) handleAddDriveFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	count, err := a.store.CountDriveFolderRefs(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if count >= 5 {
		http.Redirect(w, r, "/?folder_error="+url.QueryEscape("You can configure up to 5 allowed Drive folders."), http.StatusFound)
		return
	}
	folder, err := a.buildDriveFolderRefFromRequest(user.ID, "", r)
	if err != nil {
		http.Redirect(w, r, "/?folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	if existing, _ := a.store.FindDriveFolderRefByKey(user.ID, folder.ReferenceKey); existing != nil {
		http.Redirect(w, r, "/?folder_error="+url.QueryEscape("Reference Name already exists."), http.StatusFound)
		return
	}
	if err := a.store.CreateDriveFolderRef(folder); err != nil {
		http.Redirect(w, r, "/?folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) handleDriveFolderRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/workspace/drive-folders/")
	trimmed = strings.Trim(trimmed, "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "not_found", "folder route not found")
		return
	}
	id, action := parts[0], parts[1]
	current, err := a.store.FindDriveFolderRefByID(user.ID, id)
	if err != nil || current == nil {
		http.Redirect(w, r, "/?folder_error="+url.QueryEscape("Allowed folder not found."), http.StatusFound)
		return
	}
	switch action {
	case "update":
		folder, err := a.buildDriveFolderRefFromRequest(user.ID, id, r)
		if err != nil {
			http.Redirect(w, r, "/?folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
			return
		}
		if existing, _ := a.store.FindDriveFolderRefByKey(user.ID, folder.ReferenceKey); existing != nil && existing.ID != id {
			http.Redirect(w, r, "/?folder_error="+url.QueryEscape("Reference Name already exists."), http.StatusFound)
			return
		}
		if err := a.store.UpdateDriveFolderRef(folder); err != nil {
			http.Redirect(w, r, "/?folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
			return
		}
	case "delete":
		if err := a.store.DeleteDriveFolderRef(user.ID, id); err != nil {
			http.Redirect(w, r, "/?folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
			return
		}
	default:
		writeError(w, http.StatusNotFound, "not_found", "folder action not found")
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) buildDriveFolderRefFromRequest(userID, existingID string, r *http.Request) (*AllowedDriveFolder, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("invalid form: %w", err)
	}
	referenceName := strings.TrimSpace(r.FormValue("reference_name"))
	if referenceName == "" {
		return nil, fmt.Errorf("Reference Name is required")
	}
	folderLink := strings.TrimSpace(r.FormValue("folder_link"))
	if folderLink == "" {
		return nil, fmt.Errorf("Folder link is required")
	}
	allowDocs := r.FormValue("allow_docs") != ""
	allowSheets := r.FormValue("allow_sheets") != ""
	allowSlides := r.FormValue("allow_slides") != ""
	allowDrive := r.FormValue("allow_drive_files") != ""
	if !allowDocs && !allowSheets && !allowSlides && !allowDrive {
		allowDocs, allowSheets, allowSlides, allowDrive = true, true, true, true
	}
	folderID, resourceKey, err := parseDriveFolderLink(folderLink)
	if err != nil {
		return nil, err
	}
	conn, err := a.store.GetGmailConnection(userID)
	if err != nil || conn == nil {
		return nil, fmt.Errorf("Connect Google Workspace before configuring allowed Drive folders")
	}
	accessToken, err := a.getValidWorkspaceAccessToken(conn)
	if err != nil {
		return nil, err
	}
	meta, err := a.fetchDriveFileMetadata(accessToken, folderID, resourceKey)
	if err != nil {
		return nil, fmt.Errorf("Unable to access folder: %w", err)
	}
	if meta.MimeType != mimeTypeFolder {
		return nil, fmt.Errorf("The provided link does not point to a Google Drive folder")
	}
	return &AllowedDriveFolder{ID: existingID, UserID: userID, ReferenceName: referenceName, ReferenceKey: normalizeReferenceKey(referenceName), FolderURL: folderLink, FolderID: folderID, FolderName: meta.Name, ResourceKey: resourceKey, AllowDocs: allowDocs, AllowSheets: allowSheets, AllowSlides: allowSlides, AllowDriveFiles: allowDrive}, nil
}

func (a *App) handleRevealToken(w http.ResponseWriter, r *http.Request) {
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	rec, err := a.store.GetProxyTokenRecord(user.ID)
	if err != nil || rec == nil {
		writeError(w, http.StatusNotFound, "token_error", "proxy token not found")
		return
	}
	raw, err := a.crypto.Decrypt(rec["token_enc"])
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": raw})
}

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

func (a *App) handleRotateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	if err := a.rotateProxyToken(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *App) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	admin := a.requireAdminUser(w, r)
	if admin == nil {
		return
	}
	users, err := a.store.ListUsers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	_ = adminUsersTemplate.Execute(w, map[string]any{
		"AppName":       a.cfg.AppName,
		"Admin":         admin,
		"Users":         users,
		"PolicyPath":    a.cfg.PolicyPath,
		"DeniedLogPath": a.cfg.DeniedLogPath,
	})
}

func (a *App) handleAdminUserRoutes(w http.ResponseWriter, r *http.Request) {
	admin := a.requireAdminUser(w, r)
	if admin == nil {
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/admin/users/")
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		writeError(w, http.StatusNotFound, "not_found", "user route not found")
		return
	}
	parts := strings.Split(trimmed, "/")
	userID := parts[0]

	if len(parts) == 1 && r.Method == http.MethodGet {
		target, err := a.store.FindUserByID(userID)
		if err != nil || target == nil {
			writeError(w, http.StatusNotFound, "user_error", "user not found")
			return
		}
		stats, err := a.store.GetUserDailyStats(userID, 30)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		_ = adminUserTemplate.Execute(w, map[string]any{
			"AppName": a.cfg.AppName,
			"Admin":   admin,
			"User":    target,
			"Stats":   stats,
		})
		return
	}

	if len(parts) == 2 && r.Method == http.MethodPost {
		action := parts[1]
		switch action {
		case "suspend":
			if err := a.store.SetUserSuspended(userID, true); err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
		case "unsuspend":
			if err := a.store.SetUserSuspended(userID, false); err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
		case "delete":
			if err := a.store.DeleteUser(userID); err != nil {
				writeError(w, http.StatusInternalServerError, "db_error", err.Error())
				return
			}
		default:
			writeError(w, http.StatusNotFound, "not_found", "admin action not found")
			return
		}
		http.Redirect(w, r, "/admin/users", http.StatusFound)
		return
	}

	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "unsupported admin route")
}

func (a *App) handleProxyRelay(w http.ResponseWriter, r *http.Request) {
	proxyToken := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if proxyToken == "" {
		writeError(w, http.StatusUnauthorized, "auth_required", "missing Bearer token")
		return
	}
	user, err := a.store.FindUserByProxyToken(a.crypto, proxyToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_error", err.Error())
		return
	}
	if user == nil || user.IsSuspended {
		writeError(w, http.StatusUnauthorized, "auth_error", "invalid or suspended user")
		return
	}
	_ = a.store.TouchProxyTokenUsage(user.ID)

	conn, err := a.store.GetGmailConnection(user.ID)
	if err != nil || conn == nil {
		writeError(w, http.StatusForbidden, "workspace_not_connected", "user has not connected Google Workspace")
		return
	}

	target, err := normalizeProxyTarget(r.URL.EscapedPath(), conn.MailboxEmail)
	if err != nil {
		_ = a.store.IncrementDailyStat(user.ID, false)
		a.logDeniedRequest(r, user, conn.MailboxEmail, nil, err.Error())
		writeError(w, http.StatusForbidden, "request_denied", err.Error())
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, a.cfg.MaxRequestBodyBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "body_error", err.Error())
		return
	}
	if int64(len(body)) > a.cfg.MaxRequestBodyBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "request body too large")
		return
	}

	accessToken, err := a.getValidWorkspaceAccessToken(conn)
	if err != nil {
		writeError(w, http.StatusBadGateway, "workspace_token_error", err.Error())
		return
	}

	body, rawQuery, postMoveFolderID, err := a.rewriteWorkspaceRequest(user.ID, accessToken, r.Method, target, body, r.URL.Query())
	if err != nil {
		_ = a.store.IncrementDailyStat(user.ID, false)
		a.logDeniedRequest(r, user, conn.MailboxEmail, body, err.Error())
		writeError(w, http.StatusForbidden, "request_denied", err.Error())
		return
	}

	allowed, reason, err := a.policy.Evaluate(r.Method, target.normalizedPath, body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "policy_error", err.Error())
		return
	}
	if !allowed {
		_ = a.store.IncrementDailyStat(user.ID, false)
		a.logDeniedRequest(r, user, conn.MailboxEmail, body, reason)
		writeError(w, http.StatusForbidden, "request_denied", reason)
		return
	}

	upstreamURL := target.upstreamBase + target.normalizedPath
	if rawQuery != "" {
		upstreamURL += "?" + rawQuery
	}
	upstreamReq, err := http.NewRequest(r.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upstream_error", err.Error())
		return
	}
	copyWhitelistedRequestHeaders(upstreamReq.Header, r.Header)
	upstreamReq.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(upstreamReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if postMoveFolderID != "" && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := a.postCreateMoveToFolder(accessToken, target.serviceLabel, respBody, postMoveFolderID); err != nil {
			writeError(w, http.StatusBadGateway, "workspace_move_error", err.Error())
			return
		}
	}

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
	_ = a.store.IncrementDailyStat(user.ID, true)
}

func (a *App) currentUserFromSession(r *http.Request) (*User, error) {
	cookie, err := r.Cookie(a.cfg.SessionCookieName)
	if err != nil {
		return nil, nil
	}
	session, err := a.store.FindSessionByTokenHash(SHA256Hex(cookie.Value))
	if err != nil || session == nil {
		return nil, err
	}
	if session.ExpiresAt.Before(nowUTC()) {
		_ = a.store.DeleteSessionByTokenHash(session.TokenHash)
		return nil, nil
	}
	user, err := a.store.FindUserByID(session.UserID)
	if err != nil || user == nil || user.IsSuspended {
		return nil, err
	}
	return user, nil
}

func (a *App) requireSessionUser(w http.ResponseWriter, r *http.Request) *User {
	user, err := a.currentUserFromSession(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return nil
	}
	if user == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return nil
	}
	return user
}

func (a *App) requireAdminUser(w http.ResponseWriter, r *http.Request) *User {
	user := a.requireSessionUser(w, r)
	if user == nil {
		return nil
	}
	if !user.IsAdmin {
		writeError(w, http.StatusForbidden, "admin_required", "admin privileges required")
		return nil
	}
	return user
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

func (a *App) exchangeCodeForToken(code, clientID, clientSecret, redirectURI string) (*GoogleTokenResponse, error) {
	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	return a.postFormToken(form)
}

func (a *App) refreshToken(refreshToken, clientID, clientSecret string) (*GoogleTokenResponse, error) {
	form := url.Values{}
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "refresh_token")
	return a.postFormToken(form)
}

func (a *App) postFormToken(form url.Values) (*GoogleTokenResponse, error) {
	req, err := http.NewRequest(http.MethodPost, googleTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(raw))
	}
	var out GoogleTokenResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *App) fetchUserInfo(accessToken string) (*GoogleUserInfo, error) {
	req, err := http.NewRequest(http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(raw))
	}
	var out GoogleUserInfo
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *App) fetchGmailProfile(accessToken string) (*GmailProfile, error) {
	req, err := http.NewRequest(http.MethodGet, gmailProfileURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gmail profile endpoint returned %d: %s", resp.StatusCode, string(raw))
	}
	var out GmailProfile
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (a *App) getValidWorkspaceAccessToken(conn *GmailConnection) (string, error) {
	if conn.AccessTokenEnc != "" && conn.TokenExpiry.After(nowUTC().Add(1*time.Minute)) {
		return a.crypto.Decrypt(conn.AccessTokenEnc)
	}
	refreshToken, err := a.crypto.Decrypt(conn.RefreshTokenEnc)
	if err != nil {
		return "", err
	}
	refreshed, err := a.refreshToken(refreshToken, a.cfg.WorkspaceClientID, a.cfg.WorkspaceClientSecret)
	if err != nil {
		return "", err
	}
	accessEnc, err := a.crypto.Encrypt(refreshed.AccessToken)
	if err != nil {
		return "", err
	}
	conn.AccessTokenEnc = accessEnc
	conn.TokenExpiry = nowUTC().Add(time.Duration(refreshed.ExpiresIn) * time.Second)
	if refreshed.Scope != "" {
		conn.Scopes = refreshed.Scope
	}
	if err := a.store.SaveGmailConnection(conn); err != nil {
		return "", err
	}
	return refreshed.AccessToken, nil
}

const (
	mimeTypeFolder         = "application/vnd.google-apps.folder"
	mimeTypeDoc            = "application/vnd.google-apps.document"
	mimeTypeSheet          = "application/vnd.google-apps.spreadsheet"
	mimeTypeSlide          = "application/vnd.google-apps.presentation"
	maxDriveFoldersPerUser = 5
)

type driveFileMetadata struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	MimeType    string   `json:"mimeType"`
	Parents     []string `json:"parents"`
	ResourceKey string   `json:"resourceKey"`
	Trashed     bool     `json:"trashed"`
}

func normalizeReferenceKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func parseDriveFolderLink(raw string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", fmt.Errorf("invalid folder link")
	}
	resourceKey := u.Query().Get("resourcekey")
	re := regexp.MustCompile(`/folders/([a-zA-Z0-9_-]+)`)
	if m := re.FindStringSubmatch(u.Path); len(m) == 2 {
		return m[1], resourceKey, nil
	}
	if id := u.Query().Get("id"); id != "" {
		return id, resourceKey, nil
	}
	return "", "", fmt.Errorf("unable to extract folder id from link")
}

func allowedKindsFromFolder(f *AllowedDriveFolder) map[string]bool {
	return map[string]bool{"docs": f.AllowDocs, "sheets": f.AllowSheets, "slides": f.AllowSlides, "drive": f.AllowDriveFiles}
}

func detectFileKindFromMime(mime string) string {
	switch mime {
	case mimeTypeDoc:
		return "docs"
	case mimeTypeSheet:
		return "sheets"
	case mimeTypeSlide:
		return "slides"
	default:
		return "drive"
	}
}

func kindAllowedInFolder(f *AllowedDriveFolder, kind string) bool {
	switch kind {
	case "docs":
		return f.AllowDocs
	case "sheets":
		return f.AllowSheets
	case "slides":
		return f.AllowSlides
	default:
		return f.AllowDriveFiles
	}
}

func buildFolderQueryClause(folders []AllowedDriveFolder) string {
	parts := make([]string, 0, len(folders))
	for _, f := range folders {
		parts = append(parts, fmt.Sprintf("'%s' in parents", f.FolderID))
	}
	return strings.Join(parts, " or ")
}

func (a *App) fetchDriveFileMetadata(accessToken, fileID, resourceKey string) (*driveFileMetadata, error) {
	q := url.Values{}
	q.Set("fields", "id,name,mimeType,parents,resourceKey,trashed")
	q.Set("supportsAllDrives", "true")
	if resourceKey != "" {
		q.Set("resourceKey", resourceKey)
	}
	req, err := http.NewRequest(http.MethodGet, driveAPIBase+"/drive/v3/files/"+fileID+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("drive metadata returned %d: %s", resp.StatusCode, string(raw))
	}
	var out driveFileMetadata
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func extractPrimaryIDFromPath(path, prefix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func (a *App) resolveReferenceFolders(userID string, refName string) ([]AllowedDriveFolder, *AllowedDriveFolder, error) {
	folders, err := a.store.ListDriveFolderRefs(userID)
	if err != nil {
		return nil, nil, err
	}
	if len(folders) == 0 {
		return nil, nil, fmt.Errorf("no allowed Drive folders configured")
	}
	if refName == "" {
		return folders, nil, nil
	}
	for _, f := range folders {
		if strings.EqualFold(f.ReferenceName, refName) || f.ReferenceKey == normalizeReferenceKey(refName) {
			copy := f
			return []AllowedDriveFolder{f}, &copy, nil
		}
	}
	return nil, nil, fmt.Errorf("unknown Drive Reference Name")
}

func parseJSONMap(body []byte) (map[string]any, error) {
	if len(body) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("request body must be JSON")
	}
	return out, nil
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (a *App) rewriteWorkspaceRequest(userID, accessToken, method string, target relayTarget, body []byte, query url.Values) ([]byte, string, string, error) {
	refName := query.Get("driveRef")
	query.Del("driveRef")
	selectedFolders, selectedRef, err := a.resolveReferenceFolders(userID, refName)
	if err != nil && target.serviceLabel != "gmail" && target.serviceLabel != "calendar" {
		return body, "", "", err
	}
	switch target.serviceLabel {
	case "drive":
		return a.rewriteDriveRequest(accessToken, method, target.normalizedPath, body, query, selectedFolders, selectedRef)
	case "docs":
		return a.rewriteStructuredFileRequest(accessToken, target.serviceLabel, method, target.normalizedPath, body, query, selectedFolders, selectedRef)
	case "sheets":
		return a.rewriteStructuredFileRequest(accessToken, target.serviceLabel, method, target.normalizedPath, body, query, selectedFolders, selectedRef)
	case "slides":
		return a.rewriteStructuredFileRequest(accessToken, target.serviceLabel, method, target.normalizedPath, body, query, selectedFolders, selectedRef)
	default:
		return body, query.Encode(), "", nil
	}
}

func (a *App) rewriteDriveRequest(accessToken, method, path string, body []byte, query url.Values, folders []AllowedDriveFolder, selectedRef *AllowedDriveFolder) ([]byte, string, string, error) {
	query.Set("supportsAllDrives", "true")
	if path == "/drive/v3/files" && method == http.MethodGet {
		clause := buildFolderQueryClause(folders)
		existingQ := strings.TrimSpace(query.Get("q"))
		if existingQ != "" {
			query.Set("q", fmt.Sprintf("(%s) and (%s)", existingQ, clause))
		} else {
			query.Set("q", clause)
		}
		query.Set("includeItemsFromAllDrives", "true")
		return body, query.Encode(), "", nil
	}
	if path == "/drive/v3/files" && method == http.MethodPost {
		if selectedRef == nil {
			return body, "", "", fmt.Errorf("Drive create requires driveRef with an allowed Reference Name")
		}
		m, err := parseJSONMap(body)
		if err != nil {
			return body, "", "", err
		}
		kind := detectFileKindFromMime(fmt.Sprintf("%v", m["mimeType"]))
		if !kindAllowedInFolder(selectedRef, kind) {
			return body, "", "", fmt.Errorf("selected folder does not allow %s files", kind)
		}
		m["parents"] = []string{selectedRef.FolderID}
		delete(m, "trashed")
		return mustJSON(m), query.Encode(), "", nil
	}
	if strings.HasPrefix(path, "/drive/v3/files/") {
		fileID := extractPrimaryIDFromPath(path, "/drive/v3/files/")
		meta, folder, err := a.ensureFileInAllowedFolders(accessToken, fileID, folders)
		if err != nil {
			return body, "", "", err
		}
		if method == http.MethodPatch || method == http.MethodPut {
			if query.Get("addParents") != "" || query.Get("removeParents") != "" {
				return body, "", "", fmt.Errorf("moving files between folders is not allowed through the proxy")
			}
			m, err := parseJSONMap(body)
			if err == nil {
				if v, ok := m["trashed"]; ok && fmt.Sprintf("%v", v) == "true" {
					return body, "", "", fmt.Errorf("trashing files is not allowed")
				}
				return mustJSON(m), query.Encode(), "", nil
			}
		}
		if !kindAllowedInFolder(folder, detectFileKindFromMime(meta.MimeType)) {
			return body, "", "", fmt.Errorf("selected file type is not allowed in its configured folder")
		}
		return body, query.Encode(), "", nil
	}
	return body, query.Encode(), "", nil
}

func (a *App) rewriteStructuredFileRequest(accessToken, service, method, path string, body []byte, query url.Values, folders []AllowedDriveFolder, selectedRef *AllowedDriveFolder) ([]byte, string, string, error) {
	kind := service
	createPath := map[string]string{"docs": "/v1/documents", "sheets": "/v4/spreadsheets", "slides": "/v1/presentations"}[service]
	if path == createPath && method == http.MethodPost {
		if selectedRef == nil {
			return body, "", "", fmt.Errorf("%s create requires driveRef with an allowed Reference Name", strings.Title(service))
		}
		if !kindAllowedInFolder(selectedRef, kind) {
			return body, "", "", fmt.Errorf("selected folder does not allow %s files", kind)
		}
		return body, query.Encode(), selectedRef.FolderID, nil
	}
	fileID := extractStructuredFileID(service, path)
	if fileID == "" {
		return body, query.Encode(), "", nil
	}
	meta, folder, err := a.ensureFileInAllowedFolders(accessToken, fileID, folders)
	if err != nil {
		return body, "", "", err
	}
	if detectFileKindFromMime(meta.MimeType) != kind {
		return body, "", "", fmt.Errorf("target file is not a %s file", kind)
	}
	if !kindAllowedInFolder(folder, kind) {
		return body, "", "", fmt.Errorf("selected file type is not allowed in its configured folder")
	}
	return body, query.Encode(), "", nil
}

func extractStructuredFileID(service, path string) string {
	switch service {
	case "docs":
		if strings.HasPrefix(path, "/v1/documents/") {
			return strings.Split(strings.TrimPrefix(path, "/v1/documents/"), ":")[0]
		}
	case "sheets":
		if strings.HasPrefix(path, "/v4/spreadsheets/") {
			return strings.Split(strings.TrimPrefix(path, "/v4/spreadsheets/"), "/")[0]
		}
	case "slides":
		if strings.HasPrefix(path, "/v1/presentations/") {
			return strings.Split(strings.TrimPrefix(path, "/v1/presentations/"), "/")[0]
		}
	}
	return ""
}

func (a *App) ensureFileInAllowedFolders(accessToken, fileID string, folders []AllowedDriveFolder) (*driveFileMetadata, *AllowedDriveFolder, error) {
	meta, err := a.fetchDriveFileMetadata(accessToken, fileID, "")
	if err != nil {
		return nil, nil, err
	}
	if meta.Trashed {
		return nil, nil, fmt.Errorf("file is trashed")
	}
	for _, folder := range folders {
		for _, parent := range meta.Parents {
			if parent == folder.FolderID {
				copy := folder
				return meta, &copy, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("file is outside the allowed Drive folders")
}

func (a *App) postCreateMoveToFolder(accessToken, service string, responseBody []byte, folderID string) error {
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return err
	}
	var fileID string
	switch service {
	case "docs":
		fileID = fmt.Sprintf("%v", payload["documentId"])
	case "sheets":
		fileID = fmt.Sprintf("%v", payload["spreadsheetId"])
	case "slides":
		fileID = fmt.Sprintf("%v", payload["presentationId"])
	default:
		return nil
	}
	if fileID == "" || fileID == "<nil>" {
		return fmt.Errorf("could not determine created file id")
	}
	meta, err := a.fetchDriveFileMetadata(accessToken, fileID, "")
	if err != nil {
		return err
	}
	oldParents := strings.Join(meta.Parents, ",")
	q := url.Values{}
	q.Set("addParents", folderID)
	q.Set("removeParents", oldParents)
	q.Set("supportsAllDrives", "true")
	req, err := http.NewRequest(http.MethodPatch, driveAPIBase+"/drive/v3/files/"+fileID+"?"+q.Encode(), strings.NewReader(`{}`))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("move file returned %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}

func normalizeProxyTarget(rawPath, accountEmail string) (relayTarget, error) {
	switch {
	case strings.HasPrefix(rawPath, "/gmail.googleapis.com/"):
		normalizedPath, err := normalizeGmailPath(rawPath, accountEmail)
		if err != nil {
			return relayTarget{}, err
		}
		return relayTarget{serviceLabel: "gmail", upstreamBase: gmailAPIBase, normalizedPath: normalizedPath}, nil
	case strings.HasPrefix(rawPath, "/calendar.googleapis.com/"):
		path := strings.TrimPrefix(rawPath, "/calendar.googleapis.com")
		if !strings.HasPrefix(path, "/calendar/v3/") {
			return relayTarget{}, fmt.Errorf("only /calendar/v3/... paths are allowed")
		}
		return relayTarget{serviceLabel: "calendar", upstreamBase: calendarAPIBase, normalizedPath: path}, nil
	case strings.HasPrefix(rawPath, "/drive.googleapis.com/"):
		path := strings.TrimPrefix(rawPath, "/drive.googleapis.com")
		if !strings.HasPrefix(path, "/drive/v3/") {
			return relayTarget{}, fmt.Errorf("only /drive/v3/... paths are allowed")
		}
		return relayTarget{serviceLabel: "drive", upstreamBase: driveAPIBase, normalizedPath: path}, nil
	case strings.HasPrefix(rawPath, "/docs.googleapis.com/"):
		path := strings.TrimPrefix(rawPath, "/docs.googleapis.com")
		if !strings.HasPrefix(path, "/v1/documents") {
			return relayTarget{}, fmt.Errorf("only /v1/documents... paths are allowed")
		}
		return relayTarget{serviceLabel: "docs", upstreamBase: docsAPIBase, normalizedPath: path}, nil
	case strings.HasPrefix(rawPath, "/sheets.googleapis.com/"):
		path := strings.TrimPrefix(rawPath, "/sheets.googleapis.com")
		if !strings.HasPrefix(path, "/v4/spreadsheets") {
			return relayTarget{}, fmt.Errorf("only /v4/spreadsheets... paths are allowed")
		}
		return relayTarget{serviceLabel: "sheets", upstreamBase: sheetsAPIBase, normalizedPath: path}, nil
	case strings.HasPrefix(rawPath, "/slides.googleapis.com/"):
		path := strings.TrimPrefix(rawPath, "/slides.googleapis.com")
		if !strings.HasPrefix(path, "/v1/presentations") {
			return relayTarget{}, fmt.Errorf("only /v1/presentations... paths are allowed")
		}
		return relayTarget{serviceLabel: "slides", upstreamBase: slidesAPIBase, normalizedPath: path}, nil
	default:
		return relayTarget{}, fmt.Errorf("path must start with /gmail.googleapis.com/, /calendar.googleapis.com/, /drive.googleapis.com/, /docs.googleapis.com/, /sheets.googleapis.com/, or /slides.googleapis.com/")
	}
}

func normalizeGmailPath(rawPath, mailboxEmail string) (string, error) {
	if !strings.HasPrefix(rawPath, "/gmail.googleapis.com/") {
		return "", fmt.Errorf("path must start with /gmail.googleapis.com/")
	}
	path := strings.TrimPrefix(rawPath, "/gmail.googleapis.com")
	if !strings.HasPrefix(path, "/gmail/v1/users/") {
		return "", fmt.Errorf("only /gmail/v1/users/... paths are allowed")
	}
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		return "", fmt.Errorf("invalid Gmail API path")
	}
	userID := strings.ToLower(parts[4])
	mailboxEmail = strings.ToLower(strings.TrimSpace(mailboxEmail))
	if userID != "me" && userID != mailboxEmail {
		return "", fmt.Errorf("path may only target users/me or the connected account email")
	}
	parts[4] = "me"
	return strings.Join(parts, "/"), nil
}

func copyWhitelistedRequestHeaders(dst, src http.Header) {
	for _, key := range []string{"Accept", "Content-Type"} {
		for _, value := range src.Values(key) {
			dst.Add(key, value)
		}
	}
}

func copyResponseHeaders(dst, src http.Header) {
	for key, values := range src {
		if strings.EqualFold(key, "Transfer-Encoding") {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func (a *App) logDeniedRequest(r *http.Request, user *User, mailbox string, body []byte, reason string) {
	headers := map[string]string{}
	for key, values := range r.Header {
		if strings.EqualFold(key, "Authorization") || strings.EqualFold(key, "Cookie") {
			continue
		}
		headers[key] = strings.Join(values, ",")
	}
	_ = a.deniedLog.Log(DeniedLogEntry{
		UserID:     user.ID,
		Mailbox:    mailbox,
		Method:     r.Method,
		Path:       r.URL.Path,
		Query:      r.URL.RawQuery,
		Headers:    headers,
		Body:       sanitizeBody(body),
		Reason:     reason,
		RemoteAddr: r.RemoteAddr,
	})
}

func sanitizeBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	text := string(body)
	lower := strings.ToLower(text)
	if strings.Contains(lower, `"raw"`) {
		return `{"redacted":"gmail raw MIME content omitted"}`
	}
	if len(text) > 4096 {
		return text[:4096] + "...[truncated]"
	}
	return text
}

func subtleEqual(aVal, bVal string) bool {
	if len(aVal) != len(bVal) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(aVal), []byte(bVal)) == 1
}
