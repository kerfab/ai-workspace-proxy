// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"net/http"
	"strings"
	"time"
)

func (a *App) handleUserDriveFoldersAPI(w http.ResponseWriter, r *http.Request) {
	user := a.requireUserBackendAPIUser(w, r)
	if user == nil {
		return
	}
	if r.URL.Path != "/api/user/drive-folders" {
		writeError(w, http.StatusNotFound, "not_found", "Drive folder endpoint not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	workspaceSelector := strings.TrimSpace(r.URL.Query().Get("workspace"))
	if workspaceSelector == "" {
		writeError(w, http.StatusBadRequest, "workspace_required", "workspace query parameter is required")
		return
	}
	conn, err := a.store.ResolveGmailConnection(user.ID, workspaceSelector)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	if conn == nil {
		writeError(w, http.StatusNotFound, "workspace_not_found", "workspace not found")
		return
	}
	folders, err := a.store.ListDriveFolderRefs(user.ID, conn.MailboxEmail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	views := make([]map[string]any, 0, len(folders))
	for i := range folders {
		views = append(views, driveFolderAPIRefView(&folders[i]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"workspace":     a.workspaceAPIView(user.ID, conn),
		"drive_folders": views,
	})
}

func driveFolderAPIRefView(folder *AllowedDriveFolder) map[string]any {
	return map[string]any{
		"id":              folder.ID,
		"workspace_email": folder.MailboxEmail,
		"reference_name":  folder.ReferenceName,
		"reference_key":   folder.ReferenceKey,
		"folder_id":       folder.FolderID,
		"folder_name":     folder.FolderName,
		"created_at":      folder.CreatedAt.Format(time.RFC3339),
		"updated_at":      folder.UpdatedAt.Format(time.RFC3339),
	}
}
