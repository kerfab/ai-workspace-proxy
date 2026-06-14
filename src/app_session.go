package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

func (a *App) currentUserFromProxyBearer(r *http.Request) (*User, error) {
	proxyToken := parseBearerToken(r.Header.Get("Authorization"))
	if proxyToken == "" {
		return nil, nil
	}
	return a.store.FindUserByProxyToken(a.crypto, proxyToken)
}
func (a *App) currentUserFromUserBackendAPIBearer(r *http.Request) (*User, error) {
	backendToken := parseBearerToken(r.Header.Get("Authorization"))
	if backendToken == "" {
		return nil, nil
	}
	return a.store.FindUserByUserBackendAPIToken(a.crypto, backendToken)
}
func (a *App) requireUserBackendAPIUser(w http.ResponseWriter, r *http.Request) *User {
	user, err := a.currentUserFromUserBackendAPIBearer(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_error", err.Error())
		return nil
	}
	if user == nil || user.IsSuspended {
		writeError(w, http.StatusUnauthorized, "privileged_auth_required", "valid end-user backend API bearer token required")
		return nil
	}
	_ = a.store.TouchUserBackendAPITokenUsage(user.ID)
	return user
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
func (a *App) csrfTokenFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(a.cfg.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	return a.csrfTokenFromSession(cookie.Value)
}
func (a *App) csrfTokenFromSession(sessionToken string) string {
	mac := hmac.New(sha256.New, a.cfg.EncryptionKey)
	_, _ = mac.Write([]byte("csrf:" + sessionToken))
	return "csrf_" + hex.EncodeToString(mac.Sum(nil))
}
func (a *App) requireSessionCSRF(w http.ResponseWriter, r *http.Request) bool {
	expected := a.csrfTokenFromRequest(r)
	provided := strings.TrimSpace(r.FormValue("csrf_token"))
	if expected == "" || provided == "" || !subtleEqual(expected, provided) {
		writeError(w, http.StatusForbidden, "csrf_denied", "invalid CSRF token")
		return false
	}
	return true
}
func (a *App) resolveWorkspaceFromRequest(userID string, r *http.Request) (*GmailConnection, error) {
	workspace := strings.TrimSpace(r.FormValue("workspace"))
	if workspace == "" {
		workspace = strings.TrimSpace(r.URL.Query().Get("workspace"))
	}
	if workspace == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	return a.store.ResolveGmailConnection(userID, workspace)
}
func (a *App) resolveWorkspaceFromQuery(userID string, r *http.Request) (*GmailConnection, error) {
	workspace := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if workspace == "" {
		return nil, fmt.Errorf("workspace is required")
	}
	return a.store.ResolveGmailConnection(userID, workspace)
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
func subtleEqual(aVal, bVal string) bool {
	if len(aVal) != len(bVal) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(aVal), []byte(bVal)) == 1
}
