// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var uiTemplateFS embed.FS

func mustParseUITemplate(path string) *template.Template {
	return template.Must(template.ParseFS(uiTemplateFS, path))
}

func mustParseDashboardTemplate(path string) *template.Template {
	t := template.Must(template.New("dash").ParseFS(uiTemplateFS, "templates/*.html"))
	source, err := uiTemplateFS.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return template.Must(t.Parse(string(source)))
}

var (
	loginTemplate              = mustParseUITemplate("templates/login.html")
	oauthErrorTemplate         = mustParseUITemplate("templates/oauth-error.html")
	twoFactorChallengeTemplate = mustParseUITemplate("templates/twofactor.html")

	dashboardTemplate = mustParseDashboardTemplate("templates/app.html")
)
