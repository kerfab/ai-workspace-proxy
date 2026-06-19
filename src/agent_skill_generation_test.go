// Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
// Proprietary software. No use, copy, modification, distribution, disclosure,
// or reverse engineering is permitted without prior written authorization
// from Opensense Ltd.

package main

import (
	"strings"
	"testing"
)

func TestAgentSkillMarkdownAddsHumanApprovalNoticeForReviewRequiredCapabilities(t *testing.T) {
	workspace := agentSkillWorkspace{
		Email:          "workspace@example.com",
		Name:           "Work",
		PolicyName:     "Reviewed policy",
		Capabilities:   []string{"gmail_messages_read", "gmail_send"},
		ReviewRequired: []string{"gmail_send"},
	}
	markdown, err := buildAgentServiceSkillMarkdown(workspace, "agent_skill_assets/snippets/guidance/GMAIL.md", "agent_skill_assets/snippets/GMAIL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(markdown, "### Send Emails") {
		t.Fatal("expected Send Emails section")
	}
	if !strings.Contains(markdown, "IMPORTANT: This operation requires mandatory human review and approval") {
		t.Fatal("expected human approval notice")
	}
	for _, needle := range []string{
		"provide the human operator with a clear summary of the action to be performed",
		"all information needed for a comprehensive review",
		`explicitly confirms approval with "reviewed & approved" or a very similar sentence carrying the same meaning in the language currently being used`,
		"Quote, as-is, the sentence or sentence fragment written by the human that confirms the approval.",
		`--human-approval "<how approval was obtained, including the approval wording quoted as written by the human>"`,
	} {
		if !strings.Contains(markdown, needle) {
			t.Fatalf("expected human approval notice to contain %q, got:\n%s", needle, markdown)
		}
	}
	if !strings.Contains(markdown, "--human-approval") || !strings.Contains(markdown, "X-AIWP-Human-Approval") {
		t.Fatalf("expected helper flag and header guidance, got:\n%s", markdown)
	}
	readSection := markdown
	if idx := strings.Index(markdown, "### Read Emails"); idx >= 0 {
		readSection = markdown[idx:]
	}
	if idx := strings.Index(readSection, "### Send Emails"); idx >= 0 {
		readSection = readSection[:idx]
	}
	if strings.Contains(readSection, "mandatory human review and approval") {
		t.Fatalf("read-only section should not include approval notice:\n%s", readSection)
	}
}

func TestAgentRootSkillMarkdownIncludesSkillRefreshResetInstructions(t *testing.T) {
	markdown, err := buildAgentRootSkillMarkdown(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{
		`"update the skill"`,
		`"refresh the skill"`,
		`"download the skill again"`,
		`"reinstall the skill"`,
		`"reset the skill"`,
		"IMPORTANT: After the command succeeds, you MUST forget all prior knowledge loaded from the older version of this skill and re-read this exact file from scratch: `{baseDir}/SKILL.md`",
		`received an explicit approval such as "reviewed & approved" or a very similar sentence carrying the same meaning in the language currently being used`,
		`including the approval wording quoted as written by the human`,
	} {
		if !strings.Contains(markdown, needle) {
			t.Fatalf("expected root skill markdown to contain %q\n\n%s", needle, markdown)
		}
	}
}
