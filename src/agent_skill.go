package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

//go:embed agent_skill_assets/scripts agent_skill_assets/snippets agent_skill_assets/templates
var agentSkillAssets embed.FS

const agentSkillDownloadTokenTTL = 10 * time.Minute

type agentSkillPlatform string

const (
	agentSkillPlatformGeneric  agentSkillPlatform = "generic"
	agentSkillPlatformOpenClaw agentSkillPlatform = "openclaw"
)

var (
	errAgentSkillDownloadTokenInvalid = errors.New("invalid download token")
	errAgentSkillDownloadTokenExpired = errors.New("download token has expired")
	errAgentSkillPlatformInvalid      = errors.New("invalid skill platform")
)

type agentSkillWorkspace struct {
	Email        string
	Name         string
	PolicyID     string
	PolicyName   string
	Capabilities []string
}

func (a *App) handleCreateAgentSkillInstallToken(w http.ResponseWriter, r *http.Request) {
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
	platform, ok := parseAgentSkillPlatform(r.FormValue("platform"))
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_platform", "select Generic or OpenClaw")
		return
	}
	rawToken, err := RandomToken("askl_", 32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	expiresAt := nowUTC().Add(agentSkillDownloadTokenTTL)
	if err := a.store.SaveAgentSkillDownloadToken(user.ID, SHA256Hex(rawToken), string(platform), expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "token_error", err.Error())
		return
	}
	settings, err := a.store.GetUserSettings(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_error", err.Error())
		return
	}
	downloadURL := a.cfg.BaseURL + "/api/agent-skill/download?token=" + url.QueryEscape(rawToken)
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"expires_at":         expiresAt.Format(time.RFC3339),
		"expires_at_display": formatUserTime(expiresAt, settings.Timezone),
		"download_url":       downloadURL,
		"platform":           string(platform),
		"platform_label":     agentSkillPlatformLabel(platform),
		"install_note":       agentSkillInstallNote(platform),
		"post_install_note":  agentSkillPostInstallNote(platform),
		"install_command":    agentSkillInstallCommand(platform, downloadURL),
	})
}

func (a *App) handleDownloadAgentSkill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "GET required")
		return
	}
	user, platform, err := a.userForAgentSkillDownload(r)
	if err != nil {
		if errors.Is(err, errAgentSkillDownloadTokenExpired) {
			writeError(w, http.StatusUnauthorized, "download_token_expired", "download token has expired; generate a new agent skill install command from the dashboard")
			return
		}
		if errors.Is(err, errAgentSkillDownloadTokenInvalid) {
			writeError(w, http.StatusUnauthorized, "download_token_invalid", "download token is invalid or already used")
			return
		}
		if errors.Is(err, errAgentSkillPlatformInvalid) {
			writeError(w, http.StatusBadRequest, "invalid_platform", "platform query parameter must be generic or openclaw")
			return
		}
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
	data, err := a.buildAgentSkillZip(user.ID, raw, platform)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "agent_skill_error", err.Error())
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="ai-workspace-proxy-%s-skill.zip"`, platform))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (a *App) userForAgentSkillDownload(r *http.Request) (*User, agentSkillPlatform, error) {
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		userID, platformValue, ok, expired, err := a.store.ConsumeAgentSkillDownloadToken(SHA256Hex(token))
		if err != nil {
			return nil, "", err
		}
		if expired {
			return nil, "", errAgentSkillDownloadTokenExpired
		}
		if !ok {
			return nil, "", errAgentSkillDownloadTokenInvalid
		}
		platform, ok := parseAgentSkillPlatform(platformValue)
		if !ok {
			return nil, "", fmt.Errorf("stored skill platform is invalid")
		}
		user, err := a.store.FindUserByID(userID)
		return user, platform, err
	}
	platform, ok := parseAgentSkillPlatform(r.URL.Query().Get("platform"))
	if !ok {
		return nil, "", errAgentSkillPlatformInvalid
	}
	user, err := a.currentUserFromProxyBearer(r)
	return user, platform, err
}

func agentSkillInstallCommand(platform agentSkillPlatform, downloadURL string) string {
	zipPath := fmt.Sprintf("/tmp/ai-workspace-proxy-%s-skill.zip", platform)
	stagingDir := fmt.Sprintf("/tmp/ai-workspace-proxy-%s-skill", platform)
	return strings.Join([]string{
		"AIWP_ZIP=" + shellQuote(zipPath),
		"AIWP_STAGING=" + shellQuote(stagingDir),
		"AIWP_DOWNLOAD_URL=" + shellQuote(downloadURL),
		"rm -rf \"$AIWP_STAGING\" \"$AIWP_ZIP\" &&",
		"if command -v curl >/dev/null 2>&1; then",
		"  curl -fL \"$AIWP_DOWNLOAD_URL\" -o \"$AIWP_ZIP\"",
		"elif command -v wget >/dev/null 2>&1; then",
		"  wget -O \"$AIWP_ZIP\" \"$AIWP_DOWNLOAD_URL\"",
		"else",
		"  echo \"ERROR: curl or wget is required.\" >&2",
		"  exit 1",
		"fi &&",
		"mkdir -p \"$AIWP_STAGING\" &&",
		"unzip -q \"$AIWP_ZIP\" -d \"$AIWP_STAGING\" &&",
		"sh \"$AIWP_STAGING/scripts/install_skill.sh\" &&",
		"rm -rf \"$AIWP_STAGING\" \"$AIWP_ZIP\"",
	}, "\n")
}

func parseAgentSkillPlatform(value string) (agentSkillPlatform, bool) {
	switch agentSkillPlatform(strings.ToLower(strings.TrimSpace(value))) {
	case agentSkillPlatformGeneric:
		return agentSkillPlatformGeneric, true
	case agentSkillPlatformOpenClaw:
		return agentSkillPlatformOpenClaw, true
	default:
		return "", false
	}
}

func agentSkillPlatformLabel(platform agentSkillPlatform) string {
	switch platform {
	case agentSkillPlatformOpenClaw:
		return "OpenClaw"
	default:
		return "Generic"
	}
}

func agentSkillInstallNote(platform agentSkillPlatform) string {
	if platform == agentSkillPlatformOpenClaw {
		return "The installer uses $HOME/.openclaw/workspace/skills when available. If it cannot find the OpenClaw skills folder, it prompts for the full path."
	}
	return "The installer prompts for the folder where this agent skill should be installed. Set AI_WORKSPACE_PROXY_SKILL_DIR before running the command to skip the prompt."
}

func agentSkillPostInstallNote(platform agentSkillPlatform) string {
	if platform == agentSkillPlatformOpenClaw {
		return "The bootstrap rewrites local Markdown paths to the installed skill folder. Start a new OpenClaw session or restart OpenClaw if the skill list does not refresh."
	}
	return "The bootstrap rewrites local Markdown paths to the installed skill folder. Configure your AI agent platform to read the installed SKILL.md file."
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (a *App) proxyTokenForUser(userID string) (string, error) {
	rec, err := a.store.GetProxyTokenRecord(userID)
	if err != nil {
		return "", err
	}
	if rec == nil {
		return "", fmt.Errorf("agent Workspace API key not found")
	}
	return a.crypto.Decrypt(rec["token_enc"])
}

func (a *App) buildAgentSkillZip(userID, proxyToken string, platform agentSkillPlatform) ([]byte, error) {
	workspaces, err := a.agentSkillWorkspaces(userID)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	rootSkill, err := buildAgentRootSkillMarkdown(workspaces)
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
	installScript, err := agentSkillAssets.ReadFile(agentSkillInstallScriptPath(platform))
	if err != nil {
		return nil, err
	}
	if err := zipWrite(zw, "scripts/install_skill.sh", installScript); err != nil {
		return nil, err
	}
	bootstrapScript, err := agentSkillAssets.ReadFile("agent_skill_assets/scripts/bootstrap_skill.sh")
	if err != nil {
		return nil, err
	}
	if err := zipWrite(zw, "scripts/bootstrap_skill.sh", bootstrapScript); err != nil {
		return nil, err
	}
	configData := map[string]any{
		"config_type":    "agents_workspace_api_access",
		"proxy_url":      a.cfg.BaseURL,
		"proxy_token":    proxyToken,
		"skill_platform": string(platform),
		"workspaces":     agentSkillConfigWorkspaces(workspaces),
	}
	config, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := zipWrite(zw, agentWorkspaceAPIConfigPath, append(config, '\n')); err != nil {
		return nil, err
	}
	for _, workspace := range workspaces {
		dir := "skills/" + safeSkillPathSegment(workspace.Email)
		if err := zipAgentServiceSkill(zw, dir+"/GMAIL.md", workspace, "agent_skill_assets/snippets/guidance/GMAIL.md", "agent_skill_assets/snippets/GMAIL.md"); err != nil {
			return nil, err
		}
		if err := zipAgentServiceSkill(zw, dir+"/CONTACTS.md", workspace, "agent_skill_assets/snippets/guidance/CONTACTS.md", "agent_skill_assets/snippets/CONTACTS.md"); err != nil {
			return nil, err
		}
		driveIndex, err := buildAgentWorkspaceIndexMarkdown(workspace, "agent_skill_assets/snippets/index/DRIVE.md")
		if err != nil {
			return nil, err
		}
		if err := zipWrite(zw, dir+"/DRIVE.md", []byte(driveIndex)); err != nil {
			return nil, err
		}
		if err := zipAgentServiceSkill(zw, dir+"/drive/FILES.md", workspace, "agent_skill_assets/snippets/guidance/drive/FILES.md", "agent_skill_assets/snippets/drive/FILES.md"); err != nil {
			return nil, err
		}
		if err := zipAgentServiceSkill(zw, dir+"/drive/DOCS.md", workspace, "agent_skill_assets/snippets/guidance/drive/DOCS.md", "agent_skill_assets/snippets/drive/DOCS.md"); err != nil {
			return nil, err
		}
		if err := zipAgentServiceSkill(zw, dir+"/drive/SHEETS.md", workspace, "agent_skill_assets/snippets/guidance/drive/SHEETS.md", "agent_skill_assets/snippets/drive/SHEETS.md"); err != nil {
			return nil, err
		}
		if err := zipAgentServiceSkill(zw, dir+"/drive/SLIDES.md", workspace, "agent_skill_assets/snippets/guidance/drive/SLIDES.md", "agent_skill_assets/snippets/drive/SLIDES.md"); err != nil {
			return nil, err
		}
		if err := zipAgentServiceSkill(zw, dir+"/CALENDAR.md", workspace, "agent_skill_assets/snippets/guidance/CALENDAR.md", "agent_skill_assets/snippets/CALENDAR.md"); err != nil {
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

func agentSkillInstallScriptPath(platform agentSkillPlatform) string {
	if platform == agentSkillPlatformOpenClaw {
		return "agent_skill_assets/scripts/install_openclaw.sh"
	}
	return "agent_skill_assets/scripts/install_generic.sh"
}

func buildAgentRootSkillMarkdown(workspaces []agentSkillWorkspace) (string, error) {
	var workspaceSelection string
	var err error
	if len(workspaces) == 1 {
		workspaceSelection, err = renderAgentWorkspaceSnippet("agent_skill_assets/snippets/root/workspace-selection-one.md", workspaces[0])
	} else {
		workspaceSelection, err = renderAgentTemplate("agent_skill_assets/snippets/root/workspace-selection-multiple.md", nil)
	}
	if err != nil {
		return "", err
	}
	var workspaceList string
	if len(workspaces) == 0 {
		workspaceList, err = renderAgentTemplate("agent_skill_assets/snippets/root/workspace-list-empty.md", nil)
		if err != nil {
			return "", err
		}
		return renderAgentRootTemplate(strings.TrimSpace(workspaceSelection), strings.TrimSpace(workspaceList))
	}
	lines := make([]string, 0, len(workspaces))
	for _, workspace := range workspaces {
		line, err := renderAgentWorkspaceSnippet("agent_skill_assets/snippets/root/workspace-list-entry.md", workspace)
		if err != nil {
			return "", err
		}
		lines = append(lines, strings.TrimSpace(line))
	}
	workspaceList = strings.Join(lines, "\n")
	return renderAgentRootTemplate(strings.TrimSpace(workspaceSelection), workspaceList)
}

func renderAgentRootTemplate(workspaceSelection, workspaceList string) (string, error) {
	workspaceSelectionBlock, err := renderAgentTemplate("agent_skill_assets/snippets/root/workspace-selection.md", map[string]string{
		"WORKSPACE_SELECTION": workspaceSelection,
		"WORKSPACE_LIST":      workspaceList,
	})
	if err != nil {
		return "", err
	}
	frontmatter, err := agentAssetString("agent_skill_assets/snippets/root/frontmatter.md")
	if err != nil {
		return "", err
	}
	intro, err := agentAssetString("agent_skill_assets/snippets/root/intro.md")
	if err != nil {
		return "", err
	}
	usageModes, err := agentAssetString("agent_skill_assets/snippets/root/usage-modes.md")
	if err != nil {
		return "", err
	}
	routing, err := agentAssetString("agent_skill_assets/snippets/root/routing.md")
	if err != nil {
		return "", err
	}
	filesToRead, err := agentAssetString("agent_skill_assets/snippets/root/files-to-read.md")
	if err != nil {
		return "", err
	}
	secrets, err := agentAssetString("agent_skill_assets/snippets/root/secrets.md")
	if err != nil {
		return "", err
	}
	rules, err := agentAssetString("agent_skill_assets/snippets/root/rules.md")
	if err != nil {
		return "", err
	}
	update, err := agentAssetString("agent_skill_assets/snippets/root/update.md")
	if err != nil {
		return "", err
	}
	return renderAgentTemplate("agent_skill_assets/templates/SKILL_ROOT.md", map[string]string{
		"ROOT_FRONTMATTER":         frontmatter,
		"ROOT_INTRO":               intro,
		"ROOT_USAGE_MODES":         usageModes,
		"ROOT_ROUTING":             routing,
		"ROOT_FILES_TO_READ":       filesToRead,
		"ROOT_SECRETS":             secrets,
		"ROOT_WORKSPACE_SELECTION": strings.TrimSpace(workspaceSelectionBlock),
		"ROOT_RULES":               rules,
		"ROOT_UPDATE":              update,
	})
}

func buildAgentWorkspaceIndexMarkdown(workspace agentSkillWorkspace, indexSnippetPath string) (string, error) {
	indexBody, err := renderAgentWorkspaceSnippet(indexSnippetPath, workspace)
	if err != nil {
		return "", err
	}
	header, err := renderAgentWorkspaceHeader(workspace)
	if err != nil {
		return "", err
	}
	return renderAgentTemplate("agent_skill_assets/templates/WORKSPACE_INDEX.md", map[string]string{
		"INDEX_HEADER":     firstAgentMarkdownBlock(indexBody),
		"WORKSPACE_HEADER": strings.TrimSpace(header),
		"INDEX_BODY":       strings.TrimSpace(withoutFirstAgentMarkdownBlock(indexBody)),
	})
}

func zipAgentServiceSkill(zw *zip.Writer, zipPath string, workspace agentSkillWorkspace, guidancePath, snippetPath string) error {
	content, err := buildAgentServiceSkillMarkdown(workspace, guidancePath, snippetPath)
	if err != nil {
		return err
	}
	return zipWrite(zw, zipPath, []byte(content))
}

func buildAgentServiceSkillMarkdown(workspace agentSkillWorkspace, guidancePath, snippetPath string) (string, error) {
	sections, err := parseAgentCapabilitySnippetFile(snippetPath)
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
		note, err := renderAgentTemplate("agent_skill_assets/snippets/service/POLICY-FILTER-NOTE.md", nil)
		if err != nil {
			return "", err
		}
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, strings.ReplaceAll(sections[key], "WORKSPACE", workspace.Name))
		}
		body = strings.TrimSpace(note) + "\n\n" + strings.Join(parts, "\n\n")
	}
	guidance, err := renderAgentWorkspaceSnippet(guidancePath, workspace)
	if err != nil {
		return "", err
	}
	header := firstAgentMarkdownBlock(guidance)
	guidanceBody := strings.TrimSpace(withoutFirstAgentMarkdownBlock(guidance))
	workspaceHeader, err := renderAgentWorkspaceHeader(workspace)
	if err != nil {
		return "", err
	}
	return renderAgentTemplate("agent_skill_assets/templates/SERVICE_DOC.md", map[string]string{
		"SERVICE_HEADER":   header,
		"WORKSPACE_HEADER": strings.TrimSpace(workspaceHeader),
		"SERVICE_GUIDANCE": guidanceBody,
		"SERVICE_BODY":     body,
	})
}

func renderAgentWorkspaceHeader(workspace agentSkillWorkspace) (string, error) {
	return renderAgentWorkspaceSnippet("agent_skill_assets/snippets/common/workspace-header.md", workspace)
}

func renderAgentWorkspaceSnippet(path string, workspace agentSkillWorkspace) (string, error) {
	return renderAgentTemplate(path, map[string]string{
		"WORKSPACE_NAME":  workspace.Name,
		"WORKSPACE_EMAIL": workspace.Email,
		"POLICY_NAME":     workspace.PolicyName,
	})
}

func renderAgentTemplate(path string, values map[string]string) (string, error) {
	out, err := agentAssetString(path)
	if err != nil {
		return "", err
	}
	for key, value := range values {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
	}
	return out, nil
}

func agentAssetString(path string) (string, error) {
	raw, err := agentSkillAssets.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func firstAgentMarkdownBlock(markdown string) string {
	markdown = strings.TrimSpace(markdown)
	parts := strings.SplitN(markdown, "\n\n", 2)
	return strings.TrimSpace(parts[0])
}

func withoutFirstAgentMarkdownBlock(markdown string) string {
	markdown = strings.TrimSpace(markdown)
	parts := strings.SplitN(markdown, "\n\n", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func parseAgentCapabilitySnippetFile(path string) (map[string]string, error) {
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
