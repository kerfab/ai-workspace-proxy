// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func extractPrimaryIDFromPath(path, prefix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
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
func (a *App) rewriteWorkspaceRequest(userID, agentID, mailboxEmail, accessToken, method string, target relayTarget, body []byte, query url.Values) ([]byte, string, string, error) {
	refName := query.Get("driveRef")
	drivePath := query.Get("drivePath")
	driveFolderID := query.Get("driveFolderId")
	query.Del("driveRef")
	query.Del("drivePath")
	query.Del("driveFolderId")
	if target.serviceLabel == "people" {
		return rewritePeopleRequest(method, target.normalizedPath, body, query)
	}
	selectedFolders := []AllowedDriveFolder{}
	var selectedRef *AllowedDriveFolder
	if target.serviceLabel == "drive" || target.serviceLabel == "docs" || target.serviceLabel == "sheets" || target.serviceLabel == "slides" {
		var err error
		selectedFolders, selectedRef, err = a.resolveAgentReferenceFolders(userID, agentID, mailboxEmail, refName)
		if err != nil {
			return body, "", "", err
		}
	}
	switch target.serviceLabel {
	case "drive":
		return a.rewriteDriveRequest(userID, accessToken, method, target.normalizedPath, body, query, selectedFolders, selectedRef, drivePath, driveFolderID)
	case "docs":
		return a.rewriteStructuredFileRequest(userID, accessToken, target.serviceLabel, method, target.normalizedPath, body, query, selectedFolders, selectedRef, drivePath, driveFolderID)
	case "sheets":
		return a.rewriteStructuredFileRequest(userID, accessToken, target.serviceLabel, method, target.normalizedPath, body, query, selectedFolders, selectedRef, drivePath, driveFolderID)
	case "slides":
		return a.rewriteStructuredFileRequest(userID, accessToken, target.serviceLabel, method, target.normalizedPath, body, query, selectedFolders, selectedRef, drivePath, driveFolderID)
	default:
		return body, query.Encode(), "", nil
	}
}
func (a *App) rewriteDriveRequest(userID, accessToken, method, path string, body []byte, query url.Values, folders []AllowedDriveFolder, selectedRef *AllowedDriveFolder, drivePath, driveFolderID string) ([]byte, string, string, error) {
	query.Set("supportsAllDrives", "true")
	if path == "/drive/v3/files" && method == http.MethodGet {
		folderIDs, err := a.resolveDriveSearchFolderIDs(userID, accessToken, folders, selectedRef, drivePath, driveFolderID)
		if err != nil {
			return body, "", "", err
		}
		clause := buildFolderQueryClause(folderIDs)
		if clause == "" {
			return body, "", "", fmt.Errorf("no cached Drive folders are available for search")
		}
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
		targetFolderID, err := a.resolveDriveTargetFolderID(userID, accessToken, selectedRef, drivePath, driveFolderID)
		if err != nil {
			return body, "", "", err
		}
		m, err := parseJSONMap(body)
		if err != nil {
			return body, "", "", err
		}
		m["parents"] = []string{targetFolderID}
		delete(m, "trashed")
		return mustJSON(m), query.Encode(), "", nil
	}
	if strings.HasPrefix(path, "/upload/drive/v3/files/") {
		fileID := extractPrimaryIDFromPath(path, "/upload/drive/v3/files/")
		_, _, err := a.ensureFileInAllowedFolders(userID, accessToken, fileID, folders)
		if err != nil {
			return body, "", "", err
		}
		return body, query.Encode(), "", nil
	}
	if strings.HasPrefix(path, "/drive/v3/files/") {
		fileID := extractPrimaryIDFromPath(path, "/drive/v3/files/")
		_, _, err := a.ensureFileInAllowedFolders(userID, accessToken, fileID, folders)
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
		return body, query.Encode(), "", nil
	}
	return body, query.Encode(), "", nil
}
func (a *App) rewriteStructuredFileRequest(userID, accessToken, service, method, path string, body []byte, query url.Values, folders []AllowedDriveFolder, selectedRef *AllowedDriveFolder, drivePath, driveFolderID string) ([]byte, string, string, error) {
	kind := service
	createPath := map[string]string{"docs": "/v1/documents", "sheets": "/v4/spreadsheets", "slides": "/v1/presentations"}[service]
	if path == createPath && method == http.MethodPost {
		if selectedRef == nil {
			return body, "", "", fmt.Errorf("%s create requires driveRef with an allowed Reference Name", strings.Title(service))
		}
		targetFolderID, err := a.resolveDriveTargetFolderID(userID, accessToken, selectedRef, drivePath, driveFolderID)
		if err != nil {
			return body, "", "", err
		}
		return body, query.Encode(), targetFolderID, nil
	}
	fileID := extractStructuredFileID(service, path)
	if fileID == "" {
		return body, query.Encode(), "", nil
	}
	meta, _, err := a.ensureFileInAllowedFolders(userID, accessToken, fileID, folders)
	if err != nil {
		return body, "", "", err
	}
	if detectFileKindFromMime(meta.MimeType) != kind {
		return body, "", "", fmt.Errorf("target file is not a %s file", kind)
	}
	return body, query.Encode(), "", nil
}
func extractStructuredFileID(service, path string) string {
	switch service {
	case "docs":
		if strings.HasPrefix(path, "/v1/documents/") {
			return firstStructuredFilePathID(strings.TrimPrefix(path, "/v1/documents/"))
		}
	case "sheets":
		if strings.HasPrefix(path, "/v4/spreadsheets/") {
			return firstStructuredFilePathID(strings.TrimPrefix(path, "/v4/spreadsheets/"))
		}
	case "slides":
		if strings.HasPrefix(path, "/v1/presentations/") {
			return firstStructuredFilePathID(strings.TrimPrefix(path, "/v1/presentations/"))
		}
	}
	return ""
}

func firstStructuredFilePathID(rest string) string {
	id := strings.Split(rest, "/")[0]
	if decoded, err := url.PathUnescape(id); err == nil {
		id = decoded
	}
	return strings.Split(id, ":")[0]
}

func (a *App) ensureFileInAllowedFolders(userID, accessToken, fileID string, folders []AllowedDriveFolder) (*driveFileMetadata, *AllowedDriveFolder, error) {
	meta, err := a.fetchDriveFileMetadata(accessToken, fileID, "")
	if err != nil {
		return nil, nil, err
	}
	if meta.Trashed {
		return nil, nil, fmt.Errorf("file is trashed")
	}
	if folder := matchFileToAllowedFolder(userID, meta, folders, a.store); folder != nil {
		return meta, folder, nil
	}
	for i := range folders {
		if err := a.refreshDriveFolderTree(userID, accessToken, &folders[i]); err != nil {
			return nil, nil, err
		}
	}
	if folder := matchFileToAllowedFolder(userID, meta, folders, a.store); folder != nil {
		return meta, folder, nil
	}
	return nil, nil, fmt.Errorf("file is outside the allowed Drive folders")
}
func matchFileToAllowedFolder(userID string, meta *driveFileMetadata, folders []AllowedDriveFolder, store *Store) *AllowedDriveFolder {
	for _, folder := range folders {
		if meta.ID == folder.FolderID {
			copy := folder
			return &copy
		}
		for _, parent := range meta.Parents {
			if parent == folder.FolderID {
				copy := folder
				return &copy
			}
			entry, err := store.FindDriveFolderTreeByFolderID(userID, folder.ID, parent)
			if err == nil && entry != nil {
				copy := folder
				return &copy
			}
		}
	}
	return nil
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
