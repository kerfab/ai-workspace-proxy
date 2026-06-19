// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func renderDashboardResponse(w http.ResponseWriter, r *http.Request, data map[string]any) {
	if r.Header.Get("X-AIWP-Partial-Navigation") != "1" {
		_ = dashboardTemplate.Execute(w, data)
		return
	}
	var buf bytes.Buffer
	if err := dashboardTemplate.Execute(&buf, data); err != nil {
		writeError(w, http.StatusInternalServerError, "render_error", err.Error())
		return
	}
	html := buf.String()
	payload := map[string]any{
		"active_section": data["ActiveSection"],
		"admin_mode":     data["AdminMode"],
		"page_title":     data["PageTitle"],
		"topbar_crumb":   extractHTMLFragment(html, "topbar-crumb"),
		"sidebar":        extractHTMLFragment(html, "sidebar"),
		"page_header":    extractHTMLFragment(html, "main-page-header"),
		"page_content":   extractHTMLFragment(html, "main-page-content"),
	}
	for key, value := range payload {
		if strings.TrimSpace(fmt.Sprint(value)) == "" {
			writeError(w, http.StatusInternalServerError, "partial_render_error", "missing partial navigation fragment: "+key)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(payload)
}

func extractHTMLFragment(html, name string) string {
	startMarker := `<template data-aiwp-fragment="` + name + `:start"></template>`
	endMarker := `<template data-aiwp-fragment="` + name + `:end"></template>`
	start := strings.Index(html, startMarker)
	if start < 0 {
		return ""
	}
	start += len(startMarker)
	end := strings.Index(html[start:], endMarker)
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(html[start : start+end])
}
