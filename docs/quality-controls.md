# Release Quality Controls

Run these controls before each software release. The goal is to make the codebase easier to maintain, safer to operate, and clear enough that another coding agent can take over without needing hidden context.

## 1. Design And Architecture

- Review whether the current software design is accurately documented.
- Update the design documentation when architecture, data flow, authentication, policy enforcement, storage, or agent-skill generation changes.
- Keep code split into clear domain areas. Large files or large functions are warning signs; split them when doing so makes ownership and behavior easier to understand.
- Do not add backward-compatibility code unless explicitly requested. If a release requires a clean database, migration, or configuration change, document it clearly.

## 2. Duplication And Naming

- Look for copy-pasted or near-identical logic.
- Deduplicate when the behavior is truly shared or when duplication could create future drift.
- Do not force abstractions merely because code looks similar; keep separate code when different domains are likely to evolve differently.
- Rationalize names: the same concept should not appear under multiple names, variable names, function names, or UI labels unless there is a clear reason.

## 3. Code Quality

- Keep each function focused on one purpose.
- Prefer small, explicit helpers over broad generic abstractions.
- Use structured APIs and parsers instead of ad hoc string manipulation when practical.
- Avoid surprising side effects. Data flow should be easy to follow from input, validation, processing, persistence, and output.
- Add comments only where they clarify non-obvious intent, security constraints, policy behavior, tricky edge cases, or exported behavior. Do not comment obvious code.

## 4. Performance

- Look for performance improvements that preserve behavior exactly.
- Prioritize improvements that reduce unnecessary network calls, repeated parsing, repeated database queries, excessive memory use, or slow startup/build paths.
- Do not trade correctness, security, or clear policy enforcement for speed.
- When performance behavior matters, document the expected baseline and how it was measured.

## 5. Bug And Edge-Case Review

- Trace important data flows end to end.
- Check input validation, empty values, malformed values, missing records, expired tokens, revoked OAuth access, quota errors, network errors, and database errors.
- Verify that error handling is explicit and that failures do not leak secrets.
- Confirm user-facing and AI-agent-facing errors explain what failed and what the user or agent can do next.

## 6. Security Review

- Perform a security assessment for every release.
- Review authentication, authorization, OAuth scope usage, token refresh, CSRF, session handling, secrets handling, proxy request forwarding, policy enforcement, logging, and generated skill packages.
- Confirm API keys, OAuth tokens, proxy tokens, and config secrets are never exposed in UI text, logs, generated docs, examples, commits, or error messages.
- Report findings in `docs/security-report.md`.
- Each finding should include severity, affected area, impact, remediation status, and residual risk.
- If no security issues are found, record that explicitly with the date and scope of review.

## 7. User And Agent Experience

- Review all user-facing and AI-agent-facing text.
- Error messages should be specific, useful, and safe.
- UI labels should use consistent terminology.
- Generated agent skill files should be clear, concise, path-safe, and aligned with current proxy behavior and policies.
- If the generated skill package changes, verify that the download, install, update, and bootstrap instructions still match the actual files.

## 8. Documentation Accuracy

- Review every maintained documentation file touched by the release, and any documentation that describes affected behavior, even if the file itself was not changed.
- Confirm documentation accurately reflects the current application design, features, user flows, API endpoints, configuration files, policy behavior, security boundaries, generated skill package contents, and deployment/runtime expectations.
- Check examples, filenames, endpoint paths, request/response shapes, environment variables, UI labels, and warnings against the implemented code.
- Remove or update obsolete documentation instead of leaving stale compatibility notes, old filenames, old screenshots, or behavior that no longer exists.
- If documentation intentionally omits a feature or limitation, confirm the omission does not mislead users, AI agents, testers, or future coding agents.

## 9. Static And Build Checks

Run the available static and build checks before release.

Required Go checks:

```bash
gofmt -w src/*.go
go test ./...
go vet ./...
git diff --check
```

Required Docker check when Docker behavior or deployment files changed:

```bash
make docker
```

If a required command cannot be run, document why and record the residual risk.

## 10. Functional And Regression Testing

Maintain the detailed test plan in `docs/testing-strategy.md`.

- Inspect if an individual test is covering sufficient surface, or if it could be missing scenarios.
- Treat every failing test as a possible product bug until triaged. Before refining or weakening a test, inspect the relevant backend implementation, policy rules, request rewriting, persistence, and Google-facing data flow. Change the test only when the implemented behavior is confirmed correct and the test expectation or setup is wrong, obsolete, unsafe, or dependent on unavailable external service configuration.
- Maintain automated tests for existing and new behavior. Manual user testing is not a substitute for release tests.
- Add narrow regression tests for bug fixes and policy enforcement changes.
- Each feature and each policy capability should have automated coverage at the lowest practical level.
- Unit tests should cover deterministic internal logic without calling Google:
  - policy catalog validation
  - policy allow/deny matching
  - request body inspection
  - Drive path and folder-tree resolution
  - Workspace selector resolution
  - token/config validation
  - generated skill Markdown filtering
- Mocked integration tests should cover proxy behavior with fake upstream Google HTTP services:
  - login/session behavior
  - CSRF behavior
  - OAuth token refresh behavior
  - workspace isolation
  - proxy request rewriting
  - denied-request error messages
  - generated agent skill download/config behavior
- Live Workspace integration tests should be available for release validation when a dedicated test Workspace account is configured:
  - Gmail read/write permissions
  - Calendar read/write permissions, including myself-only versus third-party event rules
  - Drive folder and subfolder authorization
  - Docs, Sheets, and Slides read/create/edit permissions
  - quota, revoked-token, invalid-request, and denied-policy errors where practical
- Live Workspace tests must use dedicated test accounts, test folders, and disposable test data only.
- Live Workspace tests must clean up created data whenever practical and must never run against a user's real mailbox or business data.
- If live Workspace tests cannot be run for a release, document exactly which capabilities were not live-tested and record the residual risk.
- Each test case should have a short, human-readable description so the user can understand what is being protected at a glance.

## 11. Release Readiness Summary

Before release, write a short summary containing:

- what changed
- what quality controls were run
- what tests passed
- what could not be tested
- whether database/configuration reset or migration is required
- known residual risks
