package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

//go:embed agent_skill_assets/scripts agent_skill_assets/snippets agent_skill_assets/templates
var agentSkillAssets embed.FS

type agentSkillWorkspace struct {
	Email        string
	Name         string
	PolicyID     string
	PolicyName   string
	Capabilities []string
}

func (a *App) handleDownloadAgentSkill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	user, err := a.userForAgentSkillDownload(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "auth_error", err.Error())
		return
	}
	if user == nil || user.IsSuspended {
		writeError(w, http.StatusUnauthorized, "auth_error", "invalid or suspended user")
		return
	}
	raw, err := a.proxyTokenForUser(user.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "token_error", err.Error())
		return
	}
	data, err := a.buildAgentSkillZip(user.ID, raw)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "agent_skill_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="ai-workspace-proxy-skill.zip"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (a *App) userForAgentSkillDownload(r *http.Request) (*User, error) {
	if user, _ := a.currentUserFromSession(r); user != nil {
		return user, nil
	}
	return a.currentUserFromProxyBearer(r)
}

func (a *App) proxyTokenForUser(userID string) (string, error) {
	rec, err := a.store.GetProxyTokenRecord(userID)
	if err != nil {
		return "", err
	}
	if rec == nil {
		return "", fmt.Errorf("proxy token not found")
	}
	return a.crypto.Decrypt(rec["token_enc"])
}

func (a *App) buildAgentSkillZip(userID, proxyToken string) ([]byte, error) {
	workspaces, err := a.agentSkillWorkspaces(userID)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	rootSkill, err := buildRootSkillMarkdown(workspaces)
	if err != nil {
		return nil, err
	}
	if err := zipWrite(zw, "SKILL.md", []byte(rootSkill)); err != nil {
		return nil, err
	}
	script, err := agentSkillAssets.ReadFile("agent_skill_assets/scripts/workspace_proxy_tool.py")
	if err != nil {
		return nil, err
	}
	if err := zipWrite(zw, "scripts/workspace_proxy_tool.py", script); err != nil {
		return nil, err
	}
	updateScript, err := agentSkillAssets.ReadFile("agent_skill_assets/scripts/update_skill.sh")
	if err != nil {
		return nil, err
	}
	if err := zipWrite(zw, "scripts/update_skill.sh", updateScript); err != nil {
		return nil, err
	}
	config, err := json.MarshalIndent(map[string]any{
		"proxy_url":   a.cfg.BaseURL,
		"proxy_token": proxyToken,
		"workspaces":  agentSkillConfigWorkspaces(workspaces),
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := zipWrite(zw, "config/config.json", append(config, '\n')); err != nil {
		return nil, err
	}
	for _, workspace := range workspaces {
		dir := "skills/" + safeSkillPathSegment(workspace.Email)
		gmailIndex, err := buildWorkspaceIndexMarkdown(workspace, "agent_skill_assets/snippets/index/GMAIL.md")
		if err != nil {
			return nil, err
		}
		if err := zipWrite(zw, dir+"/GMAIL.md", []byte(gmailIndex)); err != nil {
			return nil, err
		}
		if err := zipServiceSkill(zw, dir+"/gmail/READ-OPS.md", workspace, "agent_skill_assets/snippets/guidance/gmail/READ-OPS.md", "agent_skill_assets/snippets/gmail/READ-OPS.md"); err != nil {
			return nil, err
		}
		if err := zipServiceSkill(zw, dir+"/gmail/WRITE-OPS.md", workspace, "agent_skill_assets/snippets/guidance/gmail/WRITE-OPS.md", "agent_skill_assets/snippets/gmail/WRITE-OPS.md"); err != nil {
			return nil, err
		}
		driveIndex, err := buildWorkspaceIndexMarkdown(workspace, "agent_skill_assets/snippets/index/DRIVE.md")
		if err != nil {
			return nil, err
		}
		if err := zipWrite(zw, dir+"/DRIVE.md", []byte(driveIndex)); err != nil {
			return nil, err
		}
		if err := zipServiceSkill(zw, dir+"/drive/FILES.md", workspace, "agent_skill_assets/snippets/guidance/drive/FILES.md", "agent_skill_assets/snippets/drive/FILES.md"); err != nil {
			return nil, err
		}
		if err := zipServiceSkill(zw, dir+"/drive/DOCS.md", workspace, "agent_skill_assets/snippets/guidance/drive/DOCS.md", "agent_skill_assets/snippets/drive/DOCS.md"); err != nil {
			return nil, err
		}
		if err := zipServiceSkill(zw, dir+"/drive/SHEETS.md", workspace, "agent_skill_assets/snippets/guidance/drive/SHEETS.md", "agent_skill_assets/snippets/drive/SHEETS.md"); err != nil {
			return nil, err
		}
		if err := zipServiceSkill(zw, dir+"/drive/SLIDES.md", workspace, "agent_skill_assets/snippets/guidance/drive/SLIDES.md", "agent_skill_assets/snippets/drive/SLIDES.md"); err != nil {
			return nil, err
		}
		if err := zipServiceSkill(zw, dir+"/CALENDAR.md", workspace, "agent_skill_assets/snippets/guidance/CALENDAR.md", "agent_skill_assets/snippets/CALENDAR.md"); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *App) agentSkillWorkspaces(userID string) ([]agentSkillWorkspace, error) {
	conns, err := a.store.ListGmailConnections(userID)
	if err != nil {
		return nil, err
	}
	sort.Slice(conns, func(i, j int) bool {
		return conns[i].MailboxEmail < conns[j].MailboxEmail
	})
	out := make([]agentSkillWorkspace, 0, len(conns))
	for _, conn := range conns {
		capabilities, policyID, err := a.store.WorkspacePolicyCapabilities(userID, conn.MailboxEmail)
		if err != nil {
			return nil, err
		}
		out = append(out, agentSkillWorkspace{
			Email:        conn.MailboxEmail,
			Name:         conn.FriendlyName,
			PolicyID:     policyID,
			PolicyName:   a.policyDisplayNameForUser(userID, policyID),
			Capabilities: capabilities,
		})
	}
	return out, nil
}

func (a *App) policyDisplayNameForUser(userID, policyID string) string {
	policyID = strings.TrimSpace(policyID)
	if policyID == "" || policyID == systemPolicyID {
		return systemPolicyName
	}
	policy, err := a.store.GetUserPolicy(userID, policyID)
	if err != nil || policy == nil {
		return policyID
	}
	return policy.Name
}

func agentSkillConfigWorkspaces(workspaces []agentSkillWorkspace) []map[string]string {
	out := make([]map[string]string, 0, len(workspaces))
	for _, workspace := range workspaces {
		out = append(out, map[string]string{
			"email": workspace.Email,
			"name":  workspace.Name,
		})
	}
	return out
}

func buildRootSkillMarkdown(workspaces []agentSkillWorkspace) (string, error) {
	var workspaceSelection string
	var err error
	if len(workspaces) == 1 {
		workspaceSelection, err = renderWorkspaceSnippet("agent_skill_assets/snippets/root/workspace-selection-one.md", workspaces[0])
	} else {
		workspaceSelection, err = renderTemplate("agent_skill_assets/snippets/root/workspace-selection-multiple.md", nil)
	}
	if err != nil {
		return "", err
	}
	var workspaceList string
	if len(workspaces) == 0 {
		workspaceList, err = renderTemplate("agent_skill_assets/snippets/root/workspace-list-empty.md", nil)
		if err != nil {
			return "", err
		}
		return renderRootTemplate(strings.TrimSpace(workspaceSelection), strings.TrimSpace(workspaceList))
	}
	lines := make([]string, 0, len(workspaces))
	for _, workspace := range workspaces {
		line, err := renderWorkspaceSnippet("agent_skill_assets/snippets/root/workspace-list-entry.md", workspace)
		if err != nil {
			return "", err
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	workspaceList = strings.Join(lines, "\n")
	return renderRootTemplate(strings.TrimSpace(workspaceSelection), workspaceList)
}

func renderRootTemplate(workspaceSelection, workspaceList string) (string, error) {
	workspaceSelectionBlock, err := renderTemplate("agent_skill_assets/snippets/root/workspace-selection.md", map[string]string{
		"WORKSPACE_SELECTION": workspaceSelection,
		"WORKSPACE_LIST":      workspaceList,
	})
	if err != nil {
		return "", err
	}
	frontmatter, err := assetString("agent_skill_assets/snippets/root/frontmatter.md")
	if err != nil {
		return "", err
	}
	intro, err := assetString("agent_skill_assets/snippets/root/intro.md")
	if err != nil {
		return "", err
	}
	filesToRead, err := assetString("agent_skill_assets/snippets/root/files-to-read.md")
	if err != nil {
		return "", err
	}
	rules, err := assetString("agent_skill_assets/snippets/root/rules.md")
	if err != nil {
		return "", err
	}
	update, err := assetString("agent_skill_assets/snippets/root/update.md")
	if err != nil {
		return "", err
	}
	return renderTemplate("agent_skill_assets/templates/SKILL_ROOT.md", map[string]string{
		"ROOT_FRONTMATTER":         frontmatter,
		"ROOT_INTRO":               intro,
		"ROOT_FILES_TO_READ":       filesToRead,
		"ROOT_WORKSPACE_SELECTION": strings.TrimSpace(workspaceSelectionBlock),
		"ROOT_RULES":               rules,
		"ROOT_UPDATE":              update,
	})
}

func buildWorkspaceIndexMarkdown(workspace agentSkillWorkspace, indexSnippetPath string) (string, error) {
	indexBody, err := renderWorkspaceSnippet(indexSnippetPath, workspace)
	if err != nil {
		return "", err
	}
	header, err := renderWorkspaceHeader(workspace)
	if err != nil {
		return "", err
	}
	return renderTemplate("agent_skill_assets/templates/WORKSPACE_INDEX.md", map[string]string{
		"INDEX_HEADER":     firstMarkdownBlock(indexBody),
		"WORKSPACE_HEADER": strings.TrimSpace(header),
		"INDEX_BODY":       strings.TrimSpace(withoutFirstMarkdownBlock(indexBody)),
	})
}

func zipServiceSkill(zw *zip.Writer, zipPath string, workspace agentSkillWorkspace, guidancePath, snippetPath string) error {
	content, err := buildServiceSkillMarkdown(workspace, guidancePath, snippetPath)
	if err != nil {
		return err
	}
	return zipWrite(zw, zipPath, []byte(content))
}

func buildServiceSkillMarkdown(workspace agentSkillWorkspace, guidancePath, snippetPath string) (string, error) {
	sections, err := parseCapabilitySnippetFile(snippetPath)
	if err != nil {
		return "", err
	}
	allowed := sliceToSet(workspace.Capabilities)
	keys := make([]string, 0, len(sections))
	for key := range sections {
		if allowed[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	body := ""
	if len(keys) == 0 {
		raw, err := agentSkillAssets.ReadFile("agent_skill_assets/snippets/service/NO-OPS.md")
		if err != nil {
			return "", err
		}
		body = strings.TrimSpace(string(raw))
	} else {
		note, err := renderTemplate("agent_skill_assets/snippets/service/POLICY-FILTER-NOTE.md", nil)
		if err != nil {
			return "", err
		}
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, strings.ReplaceAll(sections[key], "WORKSPACE", workspace.Name))
		}
		body = strings.TrimSpace(note) + "\n\n" + strings.Join(parts, "\n\n")
	}
	guidance, err := renderWorkspaceSnippet(guidancePath, workspace)
	if err != nil {
		return "", err
	}
	header := firstMarkdownBlock(guidance)
	guidanceBody := strings.TrimSpace(withoutFirstMarkdownBlock(guidance))
	workspaceHeader, err := renderWorkspaceHeader(workspace)
	if err != nil {
		return "", err
	}
	return renderTemplate("agent_skill_assets/templates/SERVICE_DOC.md", map[string]string{
		"SERVICE_HEADER":   header,
		"WORKSPACE_HEADER": strings.TrimSpace(workspaceHeader),
		"SERVICE_GUIDANCE": guidanceBody,
		"SERVICE_BODY":     body,
	})
}

func renderWorkspaceHeader(workspace agentSkillWorkspace) (string, error) {
	return renderWorkspaceSnippet("agent_skill_assets/snippets/common/workspace-header.md", workspace)
}

func renderWorkspaceSnippet(path string, workspace agentSkillWorkspace) (string, error) {
	return renderTemplate(path, map[string]string{
		"WORKSPACE_NAME":  workspace.Name,
		"WORKSPACE_EMAIL": workspace.Email,
		"POLICY_NAME":     workspace.PolicyName,
	})
}

func renderTemplate(path string, values map[string]string) (string, error) {
	out, err := assetString(path)
	if err != nil {
		return "", err
	}
	for key, value := range values {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
	}
	return out, nil
}

func assetString(path string) (string, error) {
	raw, err := agentSkillAssets.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func firstMarkdownBlock(markdown string) string {
	markdown = strings.TrimSpace(markdown)
	parts := strings.SplitN(markdown, "\n\n", 2)
	return strings.TrimSpace(parts[0])
}

func withoutFirstMarkdownBlock(markdown string) string {
	markdown = strings.TrimSpace(markdown)
	parts := strings.SplitN(markdown, "\n\n", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func parseCapabilitySnippetFile(path string) (map[string]string, error) {
	raw, err := agentSkillAssets.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sections := map[string]string{}
	var currentKey string
	var current strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<!-- capability: ") && strings.HasSuffix(trimmed, " -->") {
			currentKey = strings.TrimSuffix(strings.TrimPrefix(trimmed, "<!-- capability: "), " -->")
			current.Reset()
			continue
		}
		if trimmed == "<!-- /capability -->" {
			if currentKey != "" {
				sections[currentKey] = strings.TrimSpace(current.String())
			}
			currentKey = ""
			current.Reset()
			continue
		}
		if currentKey != "" {
			current.WriteString(line)
			current.WriteByte('\n')
		}
	}
	return sections, nil
}

func zipWrite(zw *zip.Writer, name string, data []byte) error {
	f, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

func safeSkillPathSegment(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "\x00", "_")
	return replacer.Replace(value)
}
