package main

import (
	"net/http"
	"strings"
)

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
		"CSRFToken":     a.csrfTokenFromRequest(r),
		"Users":         users,
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
		settings, err := a.store.GetUserSettings(admin.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "settings_error", err.Error())
			return
		}
		_ = adminUserTemplate.Execute(w, map[string]any{
			"AppName":            a.cfg.AppName,
			"Admin":              admin,
			"CSRFToken":          a.csrfTokenFromRequest(r),
			"User":               target,
			"UserCreatedAtLocal": formatUserTime(target.CreatedAt, settings.Timezone),
			"Stats":              stats,
		})
		return
	}

	if len(parts) == 2 && r.Method == http.MethodPost {
		if !a.requireSessionCSRF(w, r) {
			return
		}
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
