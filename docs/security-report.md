# Security Report

Date: 2026-06-13

Scope:

- Go HTTP service authentication, sessions, CSRF, OAuth token refresh, proxy token handling, policy enforcement, Drive folder enforcement, denied-request logging, generated agent skill packages, and release-control checks.
- TLS termination was not assessed in this report.

## Checks Run

- `gofmt -w src/*.go`
- `go test ./...`
- `go vet ./...`
- `git diff --check`
- shell syntax checks for generated installer/bootstrap/update scripts
- Python syntax check for `workspace_proxy_tool.py`
- repository scan for common token/private-key patterns
- scan for backward-compatibility/legacy code markers

## Findings

### Fixed: Weak Encryption Key Fallback

Severity: High

Affected area: `PROXY_ENCRYPTION_KEY` parsing in `src/config.go`

Impact: The code accepted arbitrary short strings by hashing them into a 32-byte key, even though the documented contract required 32 raw bytes, 64 hex characters, or base64 for exactly 32 bytes. This could allow weak operator-provided key material.

Remediation status: Fixed. Invalid key material is now rejected.

Residual risk: Existing development databases encrypted with a previously weak fallback key will not be readable after this change. This is acceptable because backward-compatibility support is intentionally not maintained unless explicitly requested.

### Fixed: Generic Skill Installer Could Target `$HOME`

Severity: High

Affected area: `src/agent_skill_assets/scripts/install_generic.sh`

Impact: The generic installer deletes the target skill directory before copying the package. It refused `/` and `/tmp`, but did not refuse `$HOME` itself.

Remediation status: Fixed. The generic installer now refuses `$HOME` as an install target.

Residual risk: The generic installer still trusts the user-provided install path. This is expected for a local installer, but users should provide a dedicated skill folder path.

## Residual Risks And Notes

- Generated agent skill packages include `config/agents-workspace-api-access.config.json` with the user's standard agent Workspace API key by design. The generated skill instructions tell agents never to reveal or print this file.
- Installed skill folders should be treated as secret-bearing local folders because they contain the standard agent Workspace API key.
- The standard agent Workspace API key lookup currently decrypts stored keys to find a match. This is acceptable for small deployments but should be revisited before scaling to many users.
- SQLite schema migrations are not implemented. Because this project currently does not maintain backward compatibility, database reset/recreation is required after development schema changes.
- No hard-coded real API keys, proxy tokens, OAuth access tokens, or private keys were found by the repository scan.

## Result

No unresolved release-blocking security issue was found in this pass after applying the fixes above.
