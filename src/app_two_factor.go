// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"fmt"
	"image/png"
	"net/http"
	"net/url"
	"strings"

	"github.com/boombuler/barcode"
	barcodeqr "github.com/boombuler/barcode/qr"
	"github.com/pquerna/otp/totp"
)

const (
	twoFactorDigits = 6
	twoFactorPeriod = 30
)

func (a *App) twoFactorAppName() string {
	name := strings.TrimSpace(a.cfg.AppName)
	if name == "" {
		return defaultAppName
	}
	return name
}

func (a *App) ensureUserTwoFactorPendingSecret(user *User) error {
	if user == nil {
		return fmt.Errorf("user is required")
	}
	settings, err := a.store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		return err
	}
	if settings != nil && (settings.Enabled || strings.TrimSpace(settings.PendingSecretEnc) != "") {
		return nil
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      a.twoFactorAppName(),
		AccountName: user.Email,
	})
	if err != nil {
		return fmt.Errorf("unable to start 2FA enrollment")
	}
	secretEnc, err := a.crypto.Encrypt(key.Secret())
	if err != nil {
		return err
	}
	return a.store.SaveUserTwoFactorPendingSecret(user.ID, secretEnc)
}

func buildTwoFactorOTPAuthURL(appName, accountEmail, secret string) string {
	appName = strings.TrimSpace(appName)
	accountEmail = normalizeEmail(accountEmail)
	label := url.PathEscape(appName + ":" + accountEmail)
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", appName)
	query.Set("algorithm", "SHA1")
	query.Set("digits", fmt.Sprintf("%d", twoFactorDigits))
	query.Set("period", fmt.Sprintf("%d", twoFactorPeriod))
	return "otpauth://totp/" + label + "?" + query.Encode()
}

func (a *App) handleTwoFactorChallenge(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.renderTwoFactorChallengePage(w, r, "")
	case http.MethodPost:
		a.handleTwoFactorChallengeVerify(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET or POST required")
	}
}

func (a *App) renderTwoFactorChallengePage(w http.ResponseWriter, r *http.Request, errorMessage string) {
	session, user, err := a.currentPendingSecondFactorUserFromSession(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	if session == nil || user == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	_ = twoFactorChallengeTemplate.Execute(w, map[string]any{
		"AppName":      a.cfg.AppName,
		"User":         user,
		"CSRFToken":    a.csrfTokenFromRequest(r),
		"ErrorMessage": errorMessage,
	})
}

func (a *App) handleTwoFactorChallengeVerify(w http.ResponseWriter, r *http.Request) {
	session, user, err := a.currentPendingSecondFactorUserFromSession(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	if session == nil || user == nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if !a.requireSessionCSRF(w, r) {
		return
	}
	settings, err := a.store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_error", err.Error())
		return
	}
	if settings == nil || !settings.Enabled || strings.TrimSpace(settings.SecretEnc) == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	secret, err := a.crypto.Decrypt(settings.SecretEnc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_error", err.Error())
		return
	}
	code := strings.TrimSpace(r.FormValue("code"))
	if !totp.Validate(code, secret) {
		a.logUserAudit(r, user, "two_factor_challenge_failed", user.ID, user.Email, map[string]any{
			"source": requestSource(r),
		})
		a.renderTwoFactorChallengePage(w, r, "Invalid authentication code. Please try again.")
		return
	}
	if err := a.store.UpdateSessionSecondFactorVerified(session.TokenHash, true); err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", err.Error())
		return
	}
	a.logUserAudit(r, user, "user_signed_in", user.ID, user.Email, map[string]any{
		"method":      "google_oauth_totp",
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

func (a *App) handleTwoFactorSetupStart(w http.ResponseWriter, r *http.Request) {
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
	settings, err := a.store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_error", err.Error())
		return
	}
	if settings != nil && settings.Enabled {
		http.Redirect(w, r, "/settings?two_factor_error="+url.QueryEscape("Reset the current 2FA enrollment before starting a new one."), http.StatusFound)
		return
	}
	if err := a.ensureUserTwoFactorPendingSecret(user); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	back := safeRelativeRedirectPath(r.FormValue("back"))
	if back == "" {
		back = "/"
	}
	http.Redirect(w, r, "/settings/2fa/enroll?back="+url.QueryEscape(back), http.StatusFound)
}

func (a *App) handleTwoFactorSetupConfirm(w http.ResponseWriter, r *http.Request) {
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
	back := safeRelativeRedirectPath(r.FormValue("back"))
	if back == "" {
		back = "/"
	}
	settings, err := a.store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_error", err.Error())
		return
	}
	if settings == nil || strings.TrimSpace(settings.PendingSecretEnc) == "" {
		http.Redirect(w, r, "/settings/2fa/enroll?back="+url.QueryEscape(back)+"&two_factor_error="+url.QueryEscape("Start a new 2FA enrollment before confirming a code."), http.StatusFound)
		return
	}
	secret, err := a.crypto.Decrypt(settings.PendingSecretEnc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "crypto_error", err.Error())
		return
	}
	code := strings.TrimSpace(r.FormValue("code"))
	if !totp.Validate(code, secret) {
		http.Redirect(w, r, "/settings/2fa/enroll?back="+url.QueryEscape(back)+"&two_factor_error="+url.QueryEscape("Invalid authentication code. Please try again."), http.StatusFound)
		return
	}
	if err := a.store.EnableUserTwoFactor(user.ID, settings.PendingSecretEnc); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	a.logUserAudit(r, user, "two_factor_enabled", user.ID, user.Email, map[string]any{
		"source": requestSource(r),
	})
	http.Redirect(w, r, back, http.StatusFound)
}

func (a *App) handleTwoFactorSetupCancel(w http.ResponseWriter, r *http.Request) {
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
	requiredByOrganization, err := a.organizationRequiresUserTwoFactor(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "organization_policy_error", err.Error())
		return
	}
	if requiredByOrganization {
		back := safeRelativeRedirectPath(r.FormValue("back"))
		if back == "" {
			back = "/"
		}
		http.Redirect(w, r, "/settings/2fa/enroll?back="+url.QueryEscape(back)+"&two_factor_error="+url.QueryEscape("Two-factor authentication is required by your organization and enrollment cannot be cancelled."), http.StatusFound)
		return
	}
	if err := a.store.CancelUserTwoFactorPendingSecret(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	back := safeRelativeRedirectPath(r.FormValue("back"))
	if back == "" {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusFound)
}

func (a *App) handleTwoFactorReset(w http.ResponseWriter, r *http.Request) {
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
	requiredByOrganization, err := a.organizationRequiresUserTwoFactor(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "organization_policy_error", err.Error())
		return
	}
	if requiredByOrganization {
		http.Redirect(w, r, "/settings?two_factor_error="+url.QueryEscape("Two-factor authentication is managed by your organization and cannot be disabled from your account settings."), http.StatusFound)
		return
	}
	policy, err := a.organizationAccountsPolicyForUser(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "organization_policy_error", err.Error())
		return
	}
	if policy != nil && !policy.AllowUserTwoFactorReset {
		http.Redirect(w, r, "/settings?two_factor_error="+url.QueryEscape("Your organization requires an administrator to reset two-factor authentication."), http.StatusFound)
		return
	}
	if err := a.store.ResetUserTwoFactor(user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	a.logUserAudit(r, user, "two_factor_reset", user.ID, user.Email, map[string]any{
		"source": requestSource(r),
	})
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func (a *App) handleTwoFactorQRCode(w http.ResponseWriter, r *http.Request) {
	user := a.requireSessionUser(w, r)
	if user == nil {
		return
	}
	settings, err := a.store.GetUserTwoFactorSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_error", err.Error())
		return
	}
	if settings == nil || strings.TrimSpace(settings.PendingSecretEnc) == "" {
		writeError(w, http.StatusNotFound, "two_factor_not_found", "2FA enrollment QR code is not available")
		return
	}
	secret, err := a.crypto.Decrypt(settings.PendingSecretEnc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "crypto_error", err.Error())
		return
	}
	otpauthURL := buildTwoFactorOTPAuthURL(a.twoFactorAppName(), user.Email, secret)
	code, err := barcodeqr.Encode(otpauthURL, barcodeqr.M, barcodeqr.Auto)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_error", "unable to generate QR code")
		return
	}
	scaled, err := barcode.Scale(code, 256, 256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "two_factor_error", "unable to scale QR code")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_ = png.Encode(w, scaled)
}
