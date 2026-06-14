package main

import (
	"fmt"
	"strings"
)

func drivePathKey(path string) string {
	return strings.ToLower(strings.TrimSpace(path))
}
func normalizeDrivePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return "", nil
	}
	parts := []string{}
	for _, part := range strings.Split(raw, "/") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "." || part == ".." {
			return "", fmt.Errorf("drivePath cannot contain . or .. segments")
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "/"), nil
}
func joinDrivePath(parentPath, name string) string {
	if parentPath == "" {
		return name
	}
	return parentPath + "/" + name
}
func (a *App) refreshDriveFolderTree(userID, accessToken string, root *AllowedDriveFolder) error {
	if root == nil {
		return fmt.Errorf("allowed folder is required")
	}
	rootEntry := DriveFolderTreeEntry{
		UserID:      userID,
		RootRefID:   root.ID,
		FolderID:    root.FolderID,
		FolderName:  root.FolderName,
		Path:        "",
		PathKey:     "",
		Depth:       0,
		ResourceKey: root.ResourceKey,
	}
	entries := []DriveFolderTreeEntry{rootEntry}
	queue := []DriveFolderTreeEntry{rootEntry}
	visited := map[string]bool{root.FolderID: true}

	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		children, err := a.listDriveChildFolders(accessToken, parent.FolderID)
		if err != nil {
			return err
		}
		for _, child := range children {
			if child.ID == "" || visited[child.ID] || child.Trashed {
				continue
			}
			visited[child.ID] = true
			childEntry := DriveFolderTreeEntry{
				UserID:         userID,
				RootRefID:      root.ID,
				FolderID:       child.ID,
				ParentFolderID: parent.FolderID,
				FolderName:     child.Name,
				Path:           joinDrivePath(parent.Path, child.Name),
				Depth:          parent.Depth + 1,
				ResourceKey:    child.ResourceKey,
			}
			childEntry.PathKey = drivePathKey(childEntry.Path)
			entries = append(entries, childEntry)
			queue = append(queue, childEntry)
		}
	}
	return a.store.ReplaceDriveFolderTree(userID, root.ID, entries)
}
func (a *App) ensureDriveFolderTreeCache(userID, accessToken string, root *AllowedDriveFolder) error {
	entries, err := a.store.ListDriveFolderTree(userID, root.ID)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return nil
	}
	return a.refreshDriveFolderTree(userID, accessToken, root)
}
func collectDriveSubtreeFolderIDs(entries []DriveFolderTreeEntry, targetFolderID string) []string {
	children := map[string][]string{}
	known := map[string]bool{}
	for _, entry := range entries {
		known[entry.FolderID] = true
		children[entry.ParentFolderID] = append(children[entry.ParentFolderID], entry.FolderID)
	}
	if !known[targetFolderID] {
		return []string{targetFolderID}
	}
	out := []string{}
	queue := []string{targetFolderID}
	seen := map[string]bool{}
	for len(queue) > 0 {
		folderID := queue[0]
		queue = queue[1:]
		if folderID == "" || seen[folderID] {
			continue
		}
		seen[folderID] = true
		out = append(out, folderID)
		queue = append(queue, children[folderID]...)
	}
	return out
}
func (a *App) resolveDriveTargetFolderID(userID, accessToken string, selectedRef *AllowedDriveFolder, drivePath, driveFolderID string) (string, error) {
	if selectedRef == nil {
		return "", fmt.Errorf("driveRef is required when selecting a subfolder")
	}
	if drivePath != "" && driveFolderID != "" {
		return "", fmt.Errorf("use either drivePath or driveFolderId, not both")
	}
	if err := a.ensureDriveFolderTreeCache(userID, accessToken, selectedRef); err != nil {
		return "", err
	}
	if driveFolderID != "" {
		if driveFolderID == selectedRef.FolderID {
			return driveFolderID, nil
		}
		entry, err := a.store.FindDriveFolderTreeByFolderID(userID, selectedRef.ID, driveFolderID)
		if err != nil {
			return "", err
		}
		if entry == nil {
			if err := a.refreshDriveFolderTree(userID, accessToken, selectedRef); err != nil {
				return "", err
			}
			entry, err = a.store.FindDriveFolderTreeByFolderID(userID, selectedRef.ID, driveFolderID)
			if err != nil {
				return "", err
			}
		}
		if entry == nil {
			return "", fmt.Errorf("driveFolderId is outside the selected driveRef")
		}
		return driveFolderID, nil
	}
	normalizedPath, err := normalizeDrivePath(drivePath)
	if err != nil {
		return "", err
	}
	if normalizedPath == "" {
		return selectedRef.FolderID, nil
	}
	matches, err := a.store.FindDriveFolderTreeByPathKey(userID, selectedRef.ID, drivePathKey(normalizedPath))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		if err := a.refreshDriveFolderTree(userID, accessToken, selectedRef); err != nil {
			return "", err
		}
		matches, err = a.store.FindDriveFolderTreeByPathKey(userID, selectedRef.ID, drivePathKey(normalizedPath))
		if err != nil {
			return "", err
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("drivePath %q was not found under driveRef %q", normalizedPath, selectedRef.ReferenceName)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("drivePath %q is ambiguous under driveRef %q; use driveFolderId", normalizedPath, selectedRef.ReferenceName)
	}
	return matches[0].FolderID, nil
}
func (a *App) resolveDriveSearchFolderIDs(userID, accessToken string, folders []AllowedDriveFolder, selectedRef *AllowedDriveFolder, drivePath, driveFolderID string) ([]string, error) {
	if drivePath != "" || driveFolderID != "" {
		targetID, err := a.resolveDriveTargetFolderID(userID, accessToken, selectedRef, drivePath, driveFolderID)
		if err != nil {
			return nil, err
		}
		entries, err := a.store.ListDriveFolderTree(userID, selectedRef.ID)
		if err != nil {
			return nil, err
		}
		return collectDriveSubtreeFolderIDs(entries, targetID), nil
	}
	out := []string{}
	for i := range folders {
		if err := a.ensureDriveFolderTreeCache(userID, accessToken, &folders[i]); err != nil {
			return nil, err
		}
		entries, err := a.store.ListDriveFolderTree(userID, folders[i].ID)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			out = append(out, entry.FolderID)
		}
	}
	return out, nil
}
func (a *App) resolveReferenceFolders(userID, mailboxEmail string, refName string) ([]AllowedDriveFolder, *AllowedDriveFolder, error) {
	folders, err := a.store.ListDriveFolderRefs(userID, mailboxEmail)
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
