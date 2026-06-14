package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func (a *App) fetchDriveFileMetadata(accessToken, fileID, resourceKey string) (*driveFileMetadata, error) {
	q := url.Values{}
	q.Set("fields", "id,name,mimeType,parents,resourceKey,trashed")
	q.Set("supportsAllDrives", "true")
	if resourceKey != "" {
		q.Set("resourceKey", resourceKey)
	}
	req, err := http.NewRequest(http.MethodGet, driveAPIBase+"/drive/v3/files/"+fileID+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &driveAPIError{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	var out driveFileMetadata
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
func (a *App) listDriveChildFolders(accessToken, parentID string) ([]driveFileMetadata, error) {
	out := []driveFileMetadata{}
	pageToken := ""
	for {
		q := url.Values{}
		q.Set("q", fmt.Sprintf("'%s' in parents and mimeType = '%s' and trashed = false", parentID, mimeTypeFolder))
		q.Set("fields", "nextPageToken,files(id,name,mimeType,parents,resourceKey,trashed)")
		q.Set("pageSize", "1000")
		q.Set("supportsAllDrives", "true")
		q.Set("includeItemsFromAllDrives", "true")
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		req, err := http.NewRequest(http.MethodGet, driveAPIBase+"/drive/v3/files?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, err
		}
		raw, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("drive child folder listing returned %d: %s", resp.StatusCode, string(raw))
		}
		var payload struct {
			Files         []driveFileMetadata `json:"files"`
			NextPageToken string              `json:"nextPageToken"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		out = append(out, payload.Files...)
		if payload.NextPageToken == "" {
			return out, nil
		}
		pageToken = payload.NextPageToken
	}
}

func (a *App) fetchDriveCommentAuthorMe(accessToken, fileID, commentID, replyID string) (bool, error) {
	q := url.Values{}
	q.Set("fields", "author/me")
	var path string
	if replyID == "" {
		path = fmt.Sprintf("/drive/v3/files/%s/comments/%s", url.PathEscape(fileID), url.PathEscape(commentID))
	} else {
		path = fmt.Sprintf("/drive/v3/files/%s/comments/%s/replies/%s", url.PathEscape(fileID), url.PathEscape(commentID), url.PathEscape(replyID))
	}
	req, err := http.NewRequest(http.MethodGet, driveAPIBase+path+"?"+q.Encode(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, &driveAPIError{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	var payload struct {
		Author struct {
			Me bool `json:"me"`
		} `json:"author"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false, err
	}
	return payload.Author.Me, nil
}
