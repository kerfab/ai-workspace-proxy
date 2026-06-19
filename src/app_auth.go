// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	loginStatePurpose             = "proxy_login"
	loginOAuthStateCookieName     = "aiwp_login_oauth_state"
	loginOAuthStateTTL            = 15 * time.Minute
	workspaceAfterLoginCookieName = "aiwp_after_login_redirect"
	workspaceOAuthStateCookieName = "aiwp_workspace_oauth_state"
	workspaceReauthTokenTTL       = 365 * 24 * time.Hour
	workspaceOAuthStateTTL        = 15 * time.Minute
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

func safeRelativeRedirectPath(raw string) string {
	raw = strings.TrimSpace(raw)
	switch {
	case raw == "":
		return ""
	case !strings.HasPrefix(raw, "/"):
		return ""
	case strings.HasPrefix(raw, "//"):
		return ""
	default:
		return raw
	}
}

func (a *App) setAfterLoginRedirectCookie(w http.ResponseWriter, path string) {
	path = safeRelativeRedirectPath(path)
	if path == "" {
		a.clearAfterLoginRedirectCookie(w)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     workspaceAfterLoginCookieName,
		Value:    url.QueryEscape(path),
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((15 * time.Minute).Seconds()),
		Expires:  nowUTC().Add(15 * time.Minute),
	})
}

func (a *App) clearAfterLoginRedirectCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     workspaceAfterLoginCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func (a *App) setOAuthStateCookie(w http.ResponseWriter, name, stateHash string, ttl time.Duration) {
	if strings.TrimSpace(name) == "" || strings.TrimSpace(stateHash) == "" || ttl <= 0 {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    stateHash,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(ttl.Seconds()),
		Expires:  nowUTC().Add(ttl),
	})
}

func (a *App) clearOAuthStateCookie(w http.ResponseWriter, name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func oauthStateCookieMatches(r *http.Request, name, stateHash string) bool {
	if r == nil || strings.TrimSpace(name) == "" || strings.TrimSpace(stateHash) == "" {
		return false
	}
	cookie, err := r.Cookie(name)
	if err != nil {
		return false
	}
	return strings.TrimSpace(cookie.Value) == strings.TrimSpace(stateHash)
}

func encodeWorkspaceOAuthStateCookieValue(stateHash, userID, mailboxEmail string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strings.Join([]string{
		strings.TrimSpace(stateHash),
		strings.TrimSpace(userID),
		normalizeEmail(mailboxEmail),
	}, "\n")))
}

func decodeWorkspaceOAuthStateCookieValue(raw string) (stateHash, userID, mailboxEmail string, ok bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return "", "", "", false
	}
	parts := strings.Split(string(decoded), "\n")
	if len(parts) != 3 {
		return "", "", "", false
	}
	stateHash = strings.TrimSpace(parts[0])
	userID = strings.TrimSpace(parts[1])
	mailboxEmail = normalizeEmail(parts[2])
	return stateHash, userID, mailboxEmail, stateHash != "" && userID != ""
}

func workspaceOAuthStateCookie(r *http.Request) (stateHash, userID, mailboxEmail string, ok bool) {
	if r == nil {
		return "", "", "", false
	}
	cookie, err := r.Cookie(workspaceOAuthStateCookieName)
	if err != nil {
		return "", "", "", false
	}
	return decodeWorkspaceOAuthStateCookieValue(cookie.Value)
}

func afterLoginRedirectPath(r *http.Request) string {
	cookie, err := r.Cookie(workspaceAfterLoginCookieName)
	if err != nil {
		return ""
	}
	value, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		return ""
	}
	return safeRelativeRedirectPath(value)
}

func (a *App) redirectWorkspaceCallbackError(w http.ResponseWriter, r *http.Request, mailboxEmail, message string) {
	target := "/"
	if mailboxEmail = normalizeEmail(mailboxEmail); mailboxEmail != "" {
		target = "/?workspace=" + url.QueryEscape(mailboxEmail)
	}
	if strings.TrimSpace(message) != "" {
		sep := "?"
		if strings.Contains(target, "?") {
			sep = "&"
		}
		target += sep + "workspace_error=" + url.QueryEscape(message)
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (a *App) writeLoginOAuthError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if wantsJSON(r) {
		writeError(w, status, "oauth_error", message)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_ = oauthErrorTemplate.Execute(w, map[string]any{
		"AppName":     a.cfg.AppName,
		"Title":       "Google sign-in could not be completed",
		"Message":     message,
		"Explanation": "The proxy could not complete the Google sign-in callback securely, so this sign-in attempt was stopped.",
		"PossibleCauses": []string{
			"The sign-in window was left open too long before completion, so the temporary sign-in state expired.",
			"The callback page was reloaded or opened more than once after Google redirected back to the proxy.",
			"Your browser lost the temporary sign-in state or blocked the cookie used to track this sign-in attempt.",
		},
	})
}

func (a *App) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	a.setAfterLoginRedirectCookie(w, r.URL.Query().Get("next"))
	state, err := RandomToken("st_", 16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "state_error", err.Error())
		return
	}
	stateHash := SHA256Hex(state)
	if err := a.store.SaveOAuthState(stateHash, loginStatePurpose, "", nowUTC().Add(loginOAuthStateTTL)); err != nil {
		writeError(w, http.StatusInternalServerError, "state_error", err.Error())
		return
	}
	a.setOAuthStateCookie(w, loginOAuthStateCookieName, stateHash, loginOAuthStateTTL)
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
		a.writeLoginOAuthError(w, r, http.StatusBadRequest, "Missing Google sign-in code or state.")
		return
	}
	stateHash := SHA256Hex(state)
	cookieStateOK := oauthStateCookieMatches(r, loginOAuthStateCookieName, stateHash)
	a.clearOAuthStateCookie(w, loginOAuthStateCookieName)
	_, ok, err := a.store.ConsumeOAuthState(stateHash, loginStatePurpose)
	if err != nil {
		a.writeLoginOAuthError(w, r, http.StatusInternalServerError, firstErr(err, "Unable to validate the Google sign-in state.").Error())
		return
	}
	if !ok && !cookieStateOK {
		if user, sessionErr := a.currentUserFromSession(r); sessionErr == nil && user != nil {
			redirectTo := afterLoginRedirectPath(r)
			a.clearAfterLoginRedirectCookie(w)
			if redirectTo == "" {
				redirectTo = "/"
			}
			http.Redirect(w, r, redirectTo, http.StatusFound)
			return
		}
		a.writeLoginOAuthError(w, r, http.StatusBadRequest, "The Google sign-in state was invalid or expired.")
		return
	}
	tokenResp, err := a.exchangeCodeForToken(code, a.cfg.WorkspaceClientID, a.cfg.WorkspaceClientSecret, a.cfg.LoginRedirectURI())
	if err != nil {
		a.writeLoginOAuthError(w, r, http.StatusBadGateway, firstErr(err, "Google sign-in token exchange failed.").Error())
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
	if err := a.ensureUserBackendAPIToken(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	userSettings, err := a.store.GetUserSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_error", err.Error())
		return
	}
	twoFactorSettings, err := a.store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_error", err.Error())
		return
	}
	requireSecondFactor := twoFactorSettings != nil && twoFactorSettings.Enabled && strings.TrimSpace(twoFactorSettings.SecretEnc) != ""
	sessionRaw, err := RandomToken("pst_", 24)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	sessionStartedAt := nowUTC()
	sessionExpiresAt := sessionExpiryFromStart(sessionStartedAt, userSettings, a.cfg.SessionTTL)
	if err := a.store.CreateSessionWithSecondFactor(user.ID, SHA256Hex(sessionRaw), sessionExpiresAt, !requireSecondFactor); err != nil {
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
		Expires:  sessionExpiresAt,
	})
	if requireSecondFactor {
		http.Redirect(w, r, "/auth/2fa", http.StatusFound)
		return
	}
	a.logUserAudit(r, user, "user_signed_in", user.ID, user.Email, map[string]any{
		"method":      "google_oauth",
		"remote_addr": r.RemoteAddr,
		"user_agent":  r.UserAgent(),
	})
	redirectTo := afterLoginRedirectPath(r)
	a.clearAfterLoginRedirectCookie(w)
	if redirectTo == "" {
		redirectTo = "/"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	if !a.requireSessionCSRF(w, r) {
		return
	}
	user, err := a.currentUserFromSession(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	if cookie, err := r.Cookie(a.cfg.SessionCookieName); err == nil {
		if user != nil {
			a.logUserAudit(r, user, "user_signed_out", user.ID, user.Email, map[string]any{
				"method":      "browser",
				"remote_addr": r.RemoteAddr,
				"user_agent":  r.UserAgent(),
			})
		}
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
	a.startWorkspaceOAuthRedirect(w, r, user.ID, "")
}

func (a *App) handleWorkspaceAuthRefresh(w http.ResponseWriter, r *http.Request) {
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	workspace := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if workspace == "" {
		workspace = strings.TrimSpace(r.FormValue("workspace"))
	}
	if workspace == "" {
		writeError(w, http.StatusBadRequest, "workspace_error", "workspace is required")
		return
	}
	conn, err := a.store.ResolveGmailConnection(user.ID, workspace)
	if err != nil {
		writeError(w, http.StatusBadRequest, "workspace_error", err.Error())
		return
	}
	if conn == nil {
		writeError(w, http.StatusNotFound, "workspace_not_found", "workspace not found")
		return
	}
	a.startWorkspaceOAuthRedirect(w, r, user.ID, conn.MailboxEmail)
}

func (a *App) startWorkspaceOAuthRedirect(w http.ResponseWriter, r *http.Request, userID, mailboxEmail string) {
	state, err := RandomToken("st_", 16)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "state_error", err.Error())
		return
	}
	stateHash := SHA256Hex(state)
	if err := a.store.SaveWorkspaceOAuthState(stateHash, userID, mailboxEmail, nowUTC().Add(workspaceOAuthStateTTL)); err != nil {
		writeError(w, http.StatusInternalServerError, "state_error", err.Error())
		return
	}
	a.setOAuthStateCookie(w, workspaceOAuthStateCookieName, encodeWorkspaceOAuthStateCookieValue(stateHash, userID, mailboxEmail), workspaceOAuthStateTTL)
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

func (a *App) handleWorkspaceReauth(w http.ResponseWriter, r *http.Request) {
	rawToken := strings.TrimSpace(r.URL.Query().Get("token"))
	if rawToken == "" {
		writeError(w, http.StatusBadRequest, "reauth_error", "missing reauthorization token")
		return
	}
	tokenHash := SHA256Hex(rawToken)
	userID, mailboxEmail, expired, ok, err := a.store.GetWorkspaceReauthToken(tokenHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reauth_error", err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusBadRequest, "reauth_error", "reauthorization link is invalid or has already been used")
		return
	}
	if expired {
		writeError(w, http.StatusBadRequest, "reauth_error", "reauthorization link has expired")
		return
	}
	user, err := a.currentUserFromSession(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	if user == nil {
		a.setAfterLoginRedirectCookie(w, r.URL.RequestURI())
		http.Redirect(w, r, "/auth/google/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
		return
	}
	if user.ID != userID {
		writeError(w, http.StatusForbidden, "reauth_error", "this reauthorization link belongs to another proxy user account")
		return
	}
	conn, err := a.store.ResolveGmailConnection(user.ID, mailboxEmail)
	if err != nil {
		writeError(w, http.StatusBadRequest, "workspace_error", err.Error())
		return
	}
	if conn == nil {
		writeError(w, http.StatusNotFound, "workspace_not_found", "workspace settings were not found on the proxy")
		return
	}
	a.startWorkspaceOAuthRedirect(w, r, user.ID, mailboxEmail)
}

func (a *App) handleWorkspaceCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		writeError(w, http.StatusBadRequest, "oauth_error", "missing code or state")
		return
	}
	stateHash := SHA256Hex(state)
	cookieStateHash, cookieUserID, cookieMailbox, cookieStateOK := workspaceOAuthStateCookie(r)
	cookieStateOK = cookieStateOK && cookieStateHash == stateHash
	a.clearOAuthStateCookie(w, workspaceOAuthStateCookieName)
	userID, expectedMailbox, ok, err := a.store.ConsumeWorkspaceOAuthState(stateHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "oauth_error", err.Error())
		return
	}
	if (!ok || userID == "") && !cookieStateOK {
		if user, sessionErr := a.currentUserFromSession(r); sessionErr == nil && user != nil {
			a.redirectWorkspaceCallbackError(w, r, "", "Google authorization response could not be validated. Please try again.")
			return
		}
		writeError(w, http.StatusBadRequest, "oauth_error", "invalid or expired state")
		return
	}
	if (!ok || userID == "") && cookieStateOK {
		userID = cookieUserID
		expectedMailbox = cookieMailbox
		ok = true
	}
	if userID == "" {
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
		a.redirectWorkspaceCallbackError(w, r, expectedMailbox, err.Error())
		return
	}
	profile, err := a.fetchGmailProfile(tokenResp.AccessToken)
	if err != nil {
		a.redirectWorkspaceCallbackError(w, r, expectedMailbox, err.Error())
		return
	}
	accountEmail := normalizeEmail(profile.EmailAddress)
	if expectedMailbox != "" && accountEmail != expectedMailbox {
		a.redirectWorkspaceCallbackError(w, r, expectedMailbox, fmt.Sprintf("Google returned %s, but this saved Workspace expects %s. The existing Google authorization for this Workspace was kept unchanged. Please select the matching Google account and try again.", accountEmail, expectedMailbox))
		return
	}
	if err := a.validateWorkspaceConnectionAgainstOrganizationPolicies(user, accountEmail); err != nil {
		a.redirectWorkspaceCallbackError(w, r, expectedMailbox, err.Error())
		return
	}
	existing, _ := a.store.GetGmailConnection(user.ID, accountEmail)
	if expectedMailbox != "" && existing == nil {
		a.redirectWorkspaceCallbackError(w, r, expectedMailbox, "Workspace settings were not found on the proxy.")
		return
	}
	refreshToken := tokenResp.RefreshToken
	if refreshToken == "" && existing != nil && strings.TrimSpace(existing.RefreshTokenEnc) != "" {
		refreshToken, _ = a.crypto.Decrypt(existing.RefreshTokenEnc)
	}
	if refreshToken == "" {
		a.redirectWorkspaceCallbackError(w, r, expectedMailbox, "Google did not return a refresh token")
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
	wasConnected := existing != nil && strings.TrimSpace(existing.RefreshTokenEnc) != ""
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
	action := "workspace_connected"
	if expectedMailbox != "" {
		if wasConnected {
			action = "workspace_auth_refreshed"
		} else {
			action = "workspace_reconnected"
		}
	}
	a.logWorkspaceAudit(r, user, accountEmail, action, accountEmail, friendlyName, map[string]any{
		"scopes": scope,
	})
	http.Redirect(w, r, "/?workspace="+url.QueryEscape(accountEmail), http.StatusFound)
}
