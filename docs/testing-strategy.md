# Testing Strategy

This document defines how AI Workspace Proxy tests should be built and maintained.

The goal is to test every feature and every policy capability without relying on manual user testing. Tests must prove both that valid use cases work and that invalid, malicious, or policy-bypassing cases fail safely. Live Google Workspace tests are valuable, but they should be the top layer of the test system, not the only layer.

## Test Case Philosophy

Every tested feature or policy capability should include both true cases and malicious/negative cases.

A failing test must be triaged as a possible application defect first. Before changing a test, inspect the relevant backend implementation, policy catalog/rules, request normalization, request rewriting, persistence, and Google-facing data flow. Refine a test only when the backend behavior is confirmed correct and the test setup or expectation is wrong, obsolete, unsafe, or blocked by unavailable external service configuration.

True cases reflect normal, intended use:

- valid inputs
- allowed policy capabilities
- authorized Workspace accounts
- registered Drive folders
- expected Google API method/path/body combinations
- expected user-facing or agent-facing success responses

Malicious/negative cases reflect misuse, mistakes, and bypass attempts:

- missing, malformed, ambiguous, or unauthorized Workspace selectors
- requests for another user's Workspace account
- unsupported Google API paths or methods
- attempts to use a policy capability outside its intended scope
- attempts to bypass Drive folder restrictions with raw file IDs, parent changes, subfolder ambiguity, or path traversal
- attempts to use third-party Calendar operations through myself-only permissions
- attempts to send, delete, share, or mutate data when only read permissions are enabled
- malformed JSON bodies, unexpected top-level keys, oversized bodies, and invalid query parameters
- expired, revoked, missing, or invalid tokens
- attempts to leak proxy tokens, OAuth tokens, config files, or other secrets through errors, logs, generated files, or agent instructions

Negative tests should verify the exact safety property being protected: denial status, safe error message, no upstream Google request when the proxy should block locally, no secret leakage, and no persisted state change when the operation is rejected.

## Test Layers

Use four layers of tests.

## 1. Unit Tests

Unit tests run with `go test ./...` and must not call Google or require real credentials.

Use unit tests for deterministic logic:

- policy catalog validation
- capability-to-rule mapping
- policy allow/deny decisions
- request body inspection
- Calendar myself-only versus third-party classification
- risk score/risk explanation coverage
- Drive path normalization
- Drive folder tree matching
- Workspace selector resolution
- config validation
- token parsing
- generated skill Markdown filtering

Unit tests should be table-driven whenever possible. Each test case should include a short name that explains the behavior being protected.

## 2. Store And Local Integration Tests

Store/local integration tests use temporary local resources but do not call Google.

Use these tests for:

- SQLite schema initialization
- user creation/update/deletion
- Workspace account persistence
- Workspace ordering
- user settings
- policy CRUD
- default policy behavior
- Drive folder reference and cached tree persistence
- single-use agent skill download tokens

Tests must use temporary directories and temporary SQLite files. They must not use the developer's real `db/` or `logs/` folders.

## 3. Mocked Proxy Integration Tests

Mocked integration tests run the proxy against fake Google HTTP servers.

Use these tests for:

- proxy relay authentication
- CSRF behavior
- OAuth callback handling
- token refresh logic
- Workspace isolation
- Gmail path normalization
- request rewriting
- Drive folder enforcement before forwarding
- policy allow/deny behavior at the HTTP layer
- upstream error propagation
- user-facing and AI-agent-facing error messages
- per-user request log entries for success and failure, including request IDs, client metadata, matched policy capability/rule, and error messages
- request-log console API behavior, including saved view settings, regex filtering, pagination, and CSV export hardening
- required AI agent motive enforcement when an agent Workspace grant demands accountability
- user audit logging for account, agent, Workspace, Drive folder, and settings actions
- generated agent skill zip contents
- role-gated and privilege-gated access control, including hidden navigation, blocked direct routes, denied AJAX handlers, and denied API entrypoints for users lacking the required role

To make this clean, the app should support injectable Google endpoint URLs in tests. Production should continue using the real Google endpoints from normal configuration.

Mocked integration tests should verify both:

- the proxy response to the caller
- the exact request received by the fake Google server

## 4. Live Workspace Integration Tests

Live tests call real Google Workspace APIs through the proxy.

These tests are release-validation tests and must be gated so they never run accidentally.

The Google Cloud project used by the proxy must have the Workspace APIs enabled before the corresponding live tests can pass:

- Gmail API
- People API
- Google Calendar API
- Google Drive API
- Google Docs API
- Google Sheets API
- Google Slides API

If Google returns `SERVICE_DISABLED`, treat it as a live-test environment prerequisite failure. Enable the named API in the Google Cloud project, wait a few minutes for propagation, and rerun from the failed test number.

Required gate:

```bash
python3 testing/start-test.py --live test-account@example.com
```

The test runner passes the test account email to live tests as `AIWP_TEST_WORKSPACE` and enables their internal live-test guard. Live tests may also require explicit configuration values, for example:

```bash
AIWP_TEST_BASE_URL=http://localhost:8080
AIWP_TEST_AGENT_API_TOKEN=...
AIWP_TEST_CALENDAR_ID=primary
AIWP_TEST_THIRD_PARTY_EMAIL=noreply@google.com
AIWP_TEST_CALENDAR_ACL_EMAIL=noreply@google.com
```

Calendar live tests default third-party event attendees and calendar sharing ACL targets to `noreply@google.com`. Override `AIWP_TEST_THIRD_PARTY_EMAIL` or `AIWP_TEST_CALENDAR_ACL_EMAIL` only when a test needs a different third-party address.

Drive and Docs live tests discover the allowed Drive folder reference through the privileged backend API. The test Workspace must have at least one allowed Drive folder whose reference name contains `Test`; the first matching reference is used. Cleanup policies include the relevant native delete permissions so Google Docs, Sheets, and Slides are not deleted through the generic Drive files permission.

When a privileged or role-gated feature depends on real external behavior, live tests should be added in addition to mocked coverage. Examples include organization-admin domain verification against real DNS, real Workspace OAuth refresh and reconnect flows, and future organization-admin APIs that act on real account state.

Live tests must use:

- dedicated test Google accounts
- dedicated test Workspace mailboxes
- dedicated test Drive folders
- disposable Docs, Sheets, Slides, emails, and calendar events
- clear naming prefixes such as `AIWP_TEST_`

Live tests must not run against a user's real mailbox, real business Drive folders, or real production calendar data.

Live tests must clean up created data whenever practical. When cleanup is impossible or unreliable, the test data must be created in dedicated disposable containers such as a test Drive folder or test calendar.

## Test Authentication Model

Automated API tests should use two different proxy keys.

Agent key:

- Used by AI agents.
- Used by tests to perform Google Workspace operations through the proxy.
- Always subject to agent identity, Workspace grant, grant policy, Drive folder enforcement, request inspection, and normal proxy restrictions.
- Stored for local live tests in the downloaded `agents-workspace-api-access.config.json` as `agent_api_token` or provided with `AIWP_TEST_AGENT_API_TOKEN`.
- Live tests that update agent grants also need the internal agent ID as test harness metadata, provided with `AIWP_TEST_AGENT_ID`.

Privileged key:

- Used by user-owned testing and administration tools.
- Used by tests to create, update, delete, and apply user-level proxy configuration such as policies, default policy, agent Workspace grants, user settings, and Drive folder references.
- Must not bypass policy enforcement for Google Workspace operations.
- Must not be included in generated agent skill packages.
- Stored for local live tests in the downloaded `user-backend-api-access.config.json` or provided with `AIWP_TEST_USER_BACKEND_API_TOKEN`.
- Must not be exposed to AI agents.

The intended live policy test flow is:

1. Use the privileged key to create or update a temporary test policy.
2. Use the user interface or a setup fixture to attach that policy to the test agent's Workspace grant.
3. Use the agent key to perform a true Google Workspace operation that should be allowed.
4. Use the agent key to perform malicious or negative operations that should be denied.
5. Disable the tested capability, attach the restricted policy to the agent grant, and verify denial with the agent key.
6. Use the privileged key to clean up test policies and restore the previous agent grant policy.

The connected test Workspace account is assumed to have already completed OAuth consent. Test automation should not attempt to fake or bypass Google OAuth consent.

Local live-test configuration lives under `testing/config/`. That folder is git-ignored because it may contain real agent API keys, backend API keys, and Workspace identifiers.

## Policy Capability Coverage

Every policy capability in `PolicyCatalog()` should have coverage at one or more layers.

Track feature and policy coverage in `testing/test-coverage.json`. Each item has a `tests` array; an empty array means no automated test is currently attached to that feature or permission.

Minimum expected coverage:

- unit test proving the capability allows and denies the expected method/path/body combinations
- mocked proxy test proving HTTP relay behavior is correct for at least one representative operation
- live Workspace test for commonly used and high-risk capabilities when test credentials are available
- live or real-environment integration test for privileged flows that depend on external state or external trust systems, when mocks cannot fully prove the security boundary

For each new capability:

1. Add or update policy unit tests.
2. Add or update generated skill snippet tests.
3. Add a mocked proxy integration test if the capability affects request rewriting, body inspection, or user-visible errors.
4. Add a live test when the capability depends on Google behavior that cannot be fully represented with mocks.
5. Include at least one true case and one malicious/negative case unless the capability is impossible to exercise safely.

For each new role-gated or privilege-gated feature:

1. Add a regression test proving authorized users can reach the feature.
2. Add a regression test proving unauthorized users cannot see or execute it through UI, route, AJAX, or API paths.
3. Add a live or real-environment test when the privilege boundary depends on real DNS, real OAuth state, real third-party account behavior, or other external trust systems.

## Live Test Safety Rules

Live tests must be defensive.

- Require `python3 testing/start-test.py --live TEST_ACCOUNT_EMAIL` for normal execution.
- Keep an internal guard such as `AIWP_LIVE_TESTS=1` so live scripts do not run accidentally when executed directly.
- Refuse to run if required environment variables are missing.
- Refuse to run if test resource names do not contain the expected `AIWP_TEST_` prefix when applicable.
- Avoid permanent deletion tests unless they use disposable data created during the same test run.
- Prefer trash/archive/reversible operations when testing destructive permissions.
- Keep cleanup best-effort but explicit.
- Log test-created resource IDs so cleanup can be performed manually if needed.

## Suggested Test Organization

Suggested Go files:

- `policy_test.go`: policy catalog and policy engine tests.
- `config_test.go`: configuration validation tests.
- `store_test.go`: SQLite persistence tests.
- `proxy_relay_test.go`: mocked proxy integration tests.
- `agent_skill_test.go`: generated zip and Markdown filtering tests.
- `live_workspace_test.go`: live Google Workspace tests guarded by `//go:build live`.

Suggested helper files:

- `test_helpers_test.go`: shared local test helpers.
- `test_google_fake_test.go`: fake Google upstream server helpers.
- `test_live_helpers_test.go`: live-test setup and cleanup helpers guarded by `//go:build live`.

## Commands

Fast local tests:

```bash
go test ./...
```

Race-enabled local tests:

```bash
go test -race ./...
```

Live Workspace release tests:

```bash
python3 testing/start-test.py --live test-account@example.com
```

Static checks:

```bash
go vet ./...
git diff --check
```

Generated script syntax checks:

```bash
sh -n src/agent_skill_assets/scripts/install_generic.sh
sh -n src/agent_skill_assets/scripts/install_openclaw.sh
sh -n src/agent_skill_assets/scripts/bootstrap_skill.sh
sh -n src/agent_skill_assets/scripts/update_skill.sh
python3 -m py_compile src/agent_skill_assets/scripts/workspace_proxy_tool.py
```

## Test Documentation

Each meaningful test should have a human-readable case name.

When adding a new test file, include a short comment or table-case names that answer:

- what behavior is protected
- what input is tested
- what result is expected
- why the case matters

Release summaries should list:

- which local tests passed
- which mocked integration tests passed
- which live Workspace tests passed
- which live Workspace capabilities were not tested
- cleanup status for live test data
- residual testing risk

## Implementation Roadmap

Build the test system incrementally.

1. Expand policy unit tests until every capability has at least one allow and deny case.
2. Add store tests with temporary SQLite databases.
3. Add agent skill zip tests that inspect generated files without running an external AI agent.
4. Add fake Google endpoint injection and mocked proxy relay tests.
5. Add live Workspace tests as registry entries marked with `"live": true`, executed through `python3 testing/start-test.py --live TEST_ACCOUNT_EMAIL`.
6. Add a release test report template once the live test suite exists.
