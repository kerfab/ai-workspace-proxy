package main

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	mimeTypeFolder = "application/vnd.google-apps.folder"
	mimeTypeDoc    = "application/vnd.google-apps.document"
	mimeTypeSheet  = "application/vnd.google-apps.spreadsheet"
	mimeTypeSlide  = "application/vnd.google-apps.presentation"
)

type driveFileMetadata struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	MimeType    string   `json:"mimeType"`
	Parents     []string `json:"parents"`
	ResourceKey string   `json:"resourceKey"`
	Trashed     bool     `json:"trashed"`
}
type driveAPIError struct {
	StatusCode int
	Body       string
}

func (e *driveAPIError) Error() string {
	return fmt.Sprintf("drive metadata returned %d: %s", e.StatusCode, e.Body)
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
	if resourceKey == "" {
		resourceKey = u.Query().Get("resourceKey")
	}
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
func drivePolicyKindFromMime(mime string) string {
	mime = strings.TrimSpace(mime)
	switch mime {
	case "", "application/octet-stream":
		return "drive"
	case mimeTypeDoc:
		return "docs"
	case mimeTypeSheet:
		return "sheets"
	case mimeTypeSlide:
		return "slides"
	default:
		if strings.HasPrefix(mime, "application/vnd.google-apps.") {
			return "google_native"
		}
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
func buildFolderQueryClause(folderIDs []string) string {
	parts := make([]string, 0, len(folderIDs))
	seen := map[string]bool{}
	for _, folderID := range folderIDs {
		if folderID == "" || seen[folderID] {
			continue
		}
		seen[folderID] = true
		parts = append(parts, fmt.Sprintf("'%s' in parents", folderID))
	}
	return strings.Join(parts, " or ")
}
