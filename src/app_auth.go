package main

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	loginStatePurpose     = "proxy_login"
	workspaceStatePurpose = "workspace_connect"
)

func (a *App) workspaceScopes() []string {
	return []string{
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/gmail.labels",
		"https://www.googleapis.com/auth/contacts.readonly",
		"https://www.googleapis.com/auth/contacts.other.readonly",
		"https://www.googleapis.com/auth/directory.readonly",
		"https://www.googleapis.com/auth/calendar",
		"https://www.googleapis.com/auth/drive",
		"https://www.googleapis.com/auth/documents",
		"https://www.googleapis.com/auth/spreadsheets",
		"https://www.googleapis.com/auth/presentations",
	}
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
	if err := a.ensureUserBackendAPIToken(user.ID); err != nil {
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
	if !a.requireSessionCSRF(w, r) {
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
	profile, err := a.fetchGmailProfile(tokenResp.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "workspace_error", err.Error())
		return
	}
	accountEmail := normalizeEmail(profile.EmailAddress)
	existing, _ := a.store.GetGmailConnection(user.ID, accountEmail)
	refreshToken := tokenResp.RefreshToken
	if refreshToken == "" && existing != nil {
		refreshToken, _ = a.crypto.Decrypt(existing.RefreshTokenEnc)
	}
	if refreshToken == "" {
		writeError(w, http.StatusBadGateway, "oauth_error", "Google did not return a refresh token")
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
	friendlyName := accountEmail
	if existing != nil && strings.TrimSpace(existing.FriendlyName) != "" {
		friendlyName = existing.FriendlyName
	}
	conn := &GmailConnection{
		UserID:          user.ID,
		MailboxEmail:    accountEmail,
		FriendlyName:    friendlyName,
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
