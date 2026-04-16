package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
)

type PolicyFile struct {
	Rules        []PolicyRuleSpec            `json:"rules"`
	BodyPolicies map[string]BodyPolicyConfig `json:"body_policies"`
}

type PolicyRuleSpec struct {
	Name       string `json:"name"`
	Method     string `json:"method"`
	PathRegex  string `json:"path_regex"`
	BodyPolicy string `json:"body_policy,omitempty"`
}

type BodyPolicyConfig struct {
	AllowedTopLevelKeys []string `json:"allowed_top_level_keys"`
	DenyAddLabelIDs     []string `json:"deny_add_label_ids"`
	DenyRemoveLabelIDs  []string `json:"deny_remove_label_ids"`
	AllowRemoveLabelIDs []string `json:"allow_remove_label_ids"`
}

type compiledRule struct {
	spec PolicyRuleSpec
	rx   *regexp.Regexp
}

type PolicyEngine struct {
	Path         string
	rules        []compiledRule
	bodyPolicies map[string]BodyPolicyConfig
}

func LoadPolicy(path string) (*PolicyEngine, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pf PolicyFile
	if err := json.Unmarshal(raw, &pf); err != nil {
		return nil, err
	}
	engine := &PolicyEngine{
		Path:         path,
		rules:        make([]compiledRule, 0, len(pf.Rules)),
		bodyPolicies: pf.BodyPolicies,
	}
	for _, spec := range pf.Rules {
		rx, err := regexp.Compile(spec.PathRegex)
		if err != nil {
			return nil, fmt.Errorf("invalid path_regex for %s: %w", spec.Name, err)
		}
		engine.rules = append(engine.rules, compiledRule{spec: spec, rx: rx})
	}
	return engine, nil
}

func (p *PolicyEngine) Evaluate(method, normalizedPath string, body []byte) (bool, string, error) {
	for _, rule := range p.rules {
		if rule.spec.Method != method {
			continue
		}
		if !rule.rx.MatchString(normalizedPath) {
			continue
		}
		if rule.spec.BodyPolicy == "" {
			return true, rule.spec.Name, nil
		}
		cfg, ok := p.bodyPolicies[rule.spec.BodyPolicy]
		if !ok {
			return false, "", fmt.Errorf("missing body policy %q", rule.spec.BodyPolicy)
		}
		if err := validateBodyPolicy(cfg, body); err != nil {
			return false, err.Error(), nil
		}
		return true, rule.spec.Name, nil
	}
	return false, "no whitelist rule matched", nil
}

type labelModifyBody struct {
	AddLabelIDs    []string `json:"addLabelIds"`
	RemoveLabelIDs []string `json:"removeLabelIds"`
}

func validateBodyPolicy(cfg BodyPolicyConfig, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("request body is required")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("request body must be valid JSON: %w", err)
	}
	allowedKeys := sliceToSet(cfg.AllowedTopLevelKeys)
	for key := range raw {
		if !allowedKeys[key] {
			return fmt.Errorf("body key %q is not allowed", key)
		}
	}
	var payload labelModifyBody
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("invalid label modify payload: %w", err)
	}
	denyAdd := sliceToSet(cfg.DenyAddLabelIDs)
	denyRemove := sliceToSet(cfg.DenyRemoveLabelIDs)
	allowRemove := sliceToSet(cfg.AllowRemoveLabelIDs)
	wildcard := allowRemove["*"]

	for _, label := range payload.AddLabelIDs {
		if denyAdd[label] {
			return fmt.Errorf("adding label %q is forbidden", label)
		}
	}
	for _, label := range payload.RemoveLabelIDs {
		if denyRemove[label] {
			return fmt.Errorf("removing label %q is forbidden", label)
		}
		if !wildcard && !allowRemove[label] {
			return fmt.Errorf("removing label %q is not allowed by policy", label)
		}
	}
	return nil
}

func sliceToSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		out[v] = true
	}
	return out
}

func (p *PolicyEngine) DescribeRules() []string {
	out := make([]string, 0, len(p.rules))
	for _, rule := range p.rules {
		out = append(out, rule.spec.Name+": "+rule.spec.Method+" "+rule.spec.PathRegex)
	}
	sort.Strings(out)
	return out
}
