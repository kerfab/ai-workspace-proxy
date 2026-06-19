# Security Report

Date: 2026-06-16

Scope: full-application security review for AI Workspace Proxy, including browser login, sessions, CSRF, optional 2FA, Workspace OAuth, per-agent API keys, privileged backend API keys, Google relay enforcement, Drive scoping, activity logging, generated skill packages, and operational deployment assumptions.

## Checks Run

- `gofmt -w src/*.go`
- `go test ./...`
- `go vet ./...`
- `git diff --check`
- shell syntax checks for generated installer/bootstrap/update scripts
- Python syntax checks for `workspace_proxy_tool.py` and live helper scripts
- `python3 testing/start-test.py --live spamboxfr@gmail.com` with resumed completion after correcting live harness metadata
- repository scan for common token/private-key strings and local secret-bearing test config
- manual review of untested auth/admin/persistence paths

## Findings

### 2026-06-16 Full Application Review

Severity: None

Affected area: full proxy service, dashboard, privileged user APIs, agent APIs, Google relay, Drive scoping, generated skill package, and activity logging.

Impact: No unresolved release-blocking security issue was found. The current implementation keeps Google OAuth tokens server-side and encrypted, separates per-agent API keys from privileged user-backend keys, validates Workspace ownership before use, enforces policy before Google forwarding, enforces Drive folder grants before Drive-family requests, and records agent/user activity with useful audit context.

Remediation status: No remediation required for release in this pass.

### 2026-06-16 Two-Factor Authentication Hardening Gap

Severity: Medium

Affected area: optional browser 2FA flow.

Impact: The current TOTP implementation provides real second-factor verification, but it does not yet include attempt throttling, temporary lockout, recovery codes, or a hardened re-enrollment recovery story. That means it should be described as a simple 2FA layer, not a mature account-recovery/security program.

Remediation status: Open follow-up. This was an explicitly requested simple implementation, so it is not treated as a release blocker for current internal use.

### 2026-06-16 Agent Firewall Trusted-Proxy Limitation

Severity: Medium

Affected area: per-agent firewall rules.

Impact: Firewall enforcement intentionally uses the direct remote connection IP and ignores `X-Forwarded-For`, `Forwarded`, and similar headers. That is the correct default for direct deployments and prevents header spoofing. However, when the proxy is placed behind Cloudflare or another reverse proxy, the effective source IP becomes the proxy hop unless trusted-proxy resolution is added deliberately.

Remediation status: Open follow-up. Not a blocker for the current direct/local deployment model used in this assessment.

### 2026-06-16 Secret-Bearing Skill Folders

Severity: Low

Affected area: generated agent skill packages and installed skill folders.

Impact: Generated skill packages intentionally contain a live agent API key. Installed skill folders therefore remain secret-bearing local material and should be treated like credentials, not like ordinary documentation or scripts.

Remediation status: Accepted by design. User guidance and report notes should continue to make this explicit.

### 2026-06-16 Local Live-Test Config Secrets

Severity: Low

Affected area: `testing/config/`.

Impact: Local live-test configuration can contain real backend and agent tokens. In this repo it remains git-ignored, which is correct, but it is still sensitive operational material on the developer machine.

Remediation status: Accepted with operational caution. No tracked-repo secret leak was found in this pass.

## Verified Positive Security Behaviors

- Workspace reauthorization rejects a wrong Google account and preserves the previously stored valid token set.
- Disconnected Workspace relay calls return a user-facing reauthorization URL for the human operator rather than silently failing.
- Agent firewall checks use the direct remote address and ignore spoofable forwarding headers.
- Human-review-required policy permissions enforce a non-empty approval explanation before forwarding.
- Per-grant agent accountability enforces a non-empty motive only where the grant requires it.
- Request logging captures effective client IP/forwarding headers for audit purposes without logging request or response bodies.

## Residual Risks And Notes

- The app does not terminate HTTPS itself. Deploy behind HTTPS and network access controls.
- Agent and backend token lookup still decrypts stored tokens for comparison; acceptable at current scale, but worth revisiting before larger deployments.
- SQLite schema migrations are intentionally not maintained for development churn; database reset/recreation is still required after incompatible schema changes.
- Admin features exist but still lack direct automated tests in the registry, so their current security confidence comes from manual review plus shared auth/CSRF helpers rather than dedicated regression tests.

## Result

No unresolved release-blocking security issue was found for the current internal deployment model and feature scope.
