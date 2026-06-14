package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (a *App) handleAddDriveFolder(w http.ResponseWriter, r *http.Request) {
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
	conn, err := a.resolveWorkspaceFromRequest(user.ID, r)
	if err != nil {
		http.Redirect(w, r, "/?folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	accessToken, err := a.getValidWorkspaceAccessToken(conn)
	if err != nil {
		http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	folder, err := a.buildDriveFolderRefFromRequest(user.ID, conn.MailboxEmail, "", accessToken, r)
	if err != nil {
		http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	if existing, _ := a.store.FindDriveFolderRefByKey(user.ID, conn.MailboxEmail, folder.ReferenceKey); existing != nil {
		http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape("Reference Name already exists."), http.StatusFound)
		return
	}
	if err := a.store.CreateDriveFolderRef(folder); err != nil {
		http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
		return
	}
	if err := a.refreshDriveFolderTree(user.ID, accessToken, folder); err != nil {
		_ = a.store.DeleteDriveFolderRef(user.ID, conn.MailboxEmail, folder.ID)
		http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape("Unable to cache subfolder tree: "+err.Error()), http.StatusFound)
		return
	}
	http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail), http.StatusFound)
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
	if !a.requireSessionCSRF(w, r) {
		return
	}
	conn, err := a.resolveWorkspaceFromRequest(user.ID, r)
	if err != nil {
		http.Redirect(w, r, "/?folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
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
	current, err := a.store.FindDriveFolderRefByID(user.ID, conn.MailboxEmail, id)
	if err != nil || current == nil {
		http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape("Allowed folder not found."), http.StatusFound)
		return
	}
	switch action {
	case "update":
		accessToken, err := a.getValidWorkspaceAccessToken(conn)
		if err != nil {
			http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
			return
		}
		folder, err := a.buildDriveFolderRefFromRequest(user.ID, conn.MailboxEmail, id, accessToken, r)
		if err != nil {
			http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
			return
		}
		if existing, _ := a.store.FindDriveFolderRefByKey(user.ID, conn.MailboxEmail, folder.ReferenceKey); existing != nil && existing.ID != id {
			http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape("Reference Name already exists."), http.StatusFound)
			return
		}
		if err := a.store.UpdateDriveFolderRef(folder); err != nil {
			http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
			return
		}
		if err := a.refreshDriveFolderTree(user.ID, accessToken, folder); err != nil {
			http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape("Unable to refresh subfolder tree: "+err.Error()), http.StatusFound)
			return
		}
	case "refresh":
		accessToken, err := a.getValidWorkspaceAccessToken(conn)
		if err != nil {
			http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
			return
		}
		if err := a.refreshDriveFolderTree(user.ID, accessToken, current); err != nil {
			http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape("Unable to refresh subfolder tree: "+err.Error()), http.StatusFound)
			return
		}
	case "delete":
		if err := a.store.DeleteDriveFolderRef(user.ID, conn.MailboxEmail, id); err != nil {
			http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail)+"&folder_error="+url.QueryEscape(err.Error()), http.StatusFound)
			return
		}
	default:
		writeError(w, http.StatusNotFound, "not_found", "folder action not found")
		return
	}
	http.Redirect(w, r, "/?workspace="+url.QueryEscape(conn.MailboxEmail), http.StatusFound)
}
func (a *App) buildDriveFolderRefFromRequest(userID, mailboxEmail, existingID, accessToken string, r *http.Request) (*AllowedDriveFolder, error) {
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
	meta, err := a.fetchDriveFileMetadata(accessToken, folderID, resourceKey)
	if err != nil {
		var driveErr *driveAPIError
		if errors.As(err, &driveErr) && driveErr.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("Unable to access folder with Workspace account %s. Make sure the folder is shared with that account, or select the Workspace account that can open the folder link", mailboxEmail)
		}
		return nil, fmt.Errorf("Unable to access folder: %w", err)
	}
	if meta.MimeType != mimeTypeFolder {
		return nil, fmt.Errorf("The provided link does not point to a Google Drive folder")
	}
	return &AllowedDriveFolder{ID: existingID, UserID: userID, MailboxEmail: normalizeEmail(mailboxEmail), ReferenceName: referenceName, ReferenceKey: normalizeReferenceKey(referenceName), FolderURL: folderLink, FolderID: folderID, FolderName: meta.Name, ResourceKey: resourceKey, AllowDocs: allowDocs, AllowSheets: allowSheets, AllowSlides: allowSlides, AllowDriveFiles: allowDrive}, nil
}
func (a *App) handleDriveFolderTreeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	user, err := a.currentUserFromProxyBearer(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_error", err.Error())
		return
	}
	if user == nil || user.IsSuspended {
		writeError(w, http.StatusUnauthorized, "auth_error", "invalid or suspended user")
		return
	}
	_ = a.store.TouchProxyTokenUsage(user.ID)
	conn, err := a.resolveWorkspaceFromQuery(user.ID, r)
	if err != nil {
		writeError(w, http.StatusForbidden, "workspace_not_found", err.Error())
		return
	}
	accessToken, err := a.getValidWorkspaceAccessToken(conn)
	if err != nil {
		writeError(w, http.StatusBadGateway, "workspace_token_error", err.Error())
		return
	}

	refName := r.URL.Query().Get("driveRef")
	folders, selectedRef, err := a.resolveReferenceFolders(user.ID, conn.MailboxEmail, refName)
	if err != nil {
		writeError(w, http.StatusForbidden, "request_denied", err.Error())
		return
	}
	if selectedRef != nil {
		folders = []AllowedDriveFolder{*selectedRef}
	}

	payloadFolders := []map[string]any{}
	for i := range folders {
		if err := a.ensureDriveFolderTreeCache(user.ID, accessToken, &folders[i]); err != nil {
			writeError(w, http.StatusBadGateway, "drive_tree_error", err.Error())
			return
		}
		entries, err := a.store.ListDriveFolderTree(user.ID, folders[i].ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		tree := []map[string]any{}
		refreshedAt := ""
		for _, entry := range entries {
			if refreshedAt == "" || entry.UpdatedAt.After(parseTime(refreshedAt)) {
				refreshedAt = entry.UpdatedAt.Format(time.RFC3339)
			}
			tree = append(tree, map[string]any{
				"folderId":       entry.FolderID,
				"parentFolderId": entry.ParentFolderID,
				"name":           entry.FolderName,
				"path":           entry.Path,
				"depth":          entry.Depth,
				"resourceKey":    entry.ResourceKey,
			})
		}
		payloadFolders = append(payloadFolders, map[string]any{
			"referenceName": folders[i].ReferenceName,
			"referenceKey":  folders[i].ReferenceKey,
			"rootFolderId":  folders[i].FolderID,
			"rootName":      folders[i].FolderName,
			"refreshedAt":   refreshedAt,
			"folders":       tree,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"driveFolders": payloadFolders})
}
func (a *App) handleRefreshDriveFolderTreeAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}
	user, err := a.currentUserFromProxyBearer(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_error", err.Error())
		return
	}
	if user == nil || user.IsSuspended {
		writeError(w, http.StatusUnauthorized, "auth_error", "invalid or suspended user")
		return
	}
	_ = a.store.TouchProxyTokenUsage(user.ID)
	conn, err := a.resolveWorkspaceFromQuery(user.ID, r)
	if err != nil {
		writeError(w, http.StatusForbidden, "workspace_not_found", err.Error())
		return
	}
	accessToken, err := a.getValidWorkspaceAccessToken(conn)
	if err != nil {
		writeError(w, http.StatusBadGateway, "workspace_token_error", err.Error())
		return
	}

	refName := r.URL.Query().Get("driveRef")
	folders, selectedRef, err := a.resolveReferenceFolders(user.ID, conn.MailboxEmail, refName)
	if err != nil {
		writeError(w, http.StatusForbidden, "request_denied", err.Error())
		return
	}
	if selectedRef != nil {
		folders = []AllowedDriveFolder{*selectedRef}
	}

	refreshed := []map[string]any{}
	for i := range folders {
		if err := a.refreshDriveFolderTree(user.ID, accessToken, &folders[i]); err != nil {
			writeError(w, http.StatusBadGateway, "drive_tree_error", err.Error())
			return
		}
		entries, err := a.store.ListDriveFolderTree(user.ID, folders[i].ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", err.Error())
			return
		}
		refreshedAt := ""
		if len(entries) > 0 {
			refreshedAt = entries[0].UpdatedAt.Format(time.RFC3339)
		}
		refreshed = append(refreshed, map[string]any{
			"referenceName": folders[i].ReferenceName,
			"referenceKey":  folders[i].ReferenceKey,
			"rootFolderId":  folders[i].FolderID,
			"rootName":      folders[i].FolderName,
			"folderCount":   len(entries),
			"refreshedAt":   refreshedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "driveFolders": refreshed})
}
