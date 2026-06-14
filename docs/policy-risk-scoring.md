# Policy Risk Scoring Criteria

This document defines how AI Workspace Proxy policy capability risk scores are assigned.

The score answers this question:

> If an AI agent goes rogue and fully exploits this allowed capability, how bad could the practical business or personal impact be?

The score is application-agnostic. It is not a generic security severity and is not a statement about a vendor OAuth scope. It is a user-facing operational risk estimate for one proxy capability, assuming the proxy still enforces all other configured limits such as selected accounts, allowed folders, and policy filters.

## Scores

### 1: Low

Use Low for operations that do not affect existing data, contact third parties, expose secrets, expose system details, alter auditability, or cause material harm.

Any read-only operation is Low unless it specifically targets secrets or system details.

For this scoring model, user-level content and user-level metadata are not system details. Reading messages, files, comments, labels, folders, calendar events, file permissions, sharing lists, or similar user-facing resource metadata is Low when the operation does not change anything.

Self-only create operations can be Low when they only create new user-owned content, do not alter existing data, do not invite/contact third parties, do not change access or sharing, and are easy to clean up.

System details means technical, administrative, security, infrastructure, billing, integration, audit, or configuration information that describes how a system operates or is controlled.

Typical Low capabilities:

- Read ordinary user or business content.
- Search, list, export, or inspect ordinary content without changing it.
- Read user-facing metadata such as labels, folders, comments, calendar events, sharing lists, or file permissions without changing it.
- Create new self-only, user-owned content that does not affect existing data or third parties.
- Query a third-party service only to retrieve values, status, calculations, or validation results.
- Preview or validate an action without staging or executing it.

### 2: Medium

Use Medium when misuse could alter existing data, create reversible but meaningful changes, create clutter beyond simple self-owned content, expose technical configuration, or prepare a harmful action that still requires manual user action before impact occurs.

Typical Medium capabilities:

- Reversible write operations that alter existing data.
- Write operations that are unlikely to be damaging or are mostly superfluous.
- Drafting, staging, previewing, or preparing actions involving third parties when the AI cannot execute the final impact itself.
- Read-only access to system settings, technical configuration, security posture, integration settings, billing configuration, or similar operational details.
- Archiving, hiding, labeling, categorizing, or organizing data when the original data remains retained and recoverable.

### 3: High

Use High when misuse could directly affect third parties, delete data, hide evidence, expose secrets, spend money, execute transactions, or cause irreversible or materially harmful changes.

Typical High capabilities:

- Sending, publishing, sharing, inviting, notifying, or otherwise directly impacting third parties.
- Deleting data, even when trash, retention, version history, or restore mechanisms exist.
- Altering logs, audit trails, history, retention rules, or evidence records.
- Accessing secrets, credentials, tokens, private keys, recovery codes, or similarly sensitive material.
- Executing payments, trades, purchases, transfers, or other financial transactions.
- Irreversible or hard-to-remediate write operations.
- Changing permissions, access controls, ownership, or security posture.

## Tie Breakers

When a capability appears to fit two levels, use the higher score only if the higher-impact outcome is a normal direct use of that capability.

Do not automatically rate any read permission above Low. Reads are Low unless they specifically target secrets or system details. User-facing content, sharing lists, permissions, labels, folders, comments, and other resource metadata are still Low when read-only.

Do not automatically rate every write permission Medium or High. Self-only creation of new user-owned content can be Low when it does not alter existing data or affect third parties. Drafting, staging, organization, and reversible edits to existing data are usually Medium. Deletion, sharing, external communication, financial execution, evidence tampering, and persistent security-impacting changes are High.

## Maintenance Rule

Every capability in `PolicyCatalog()` must have:

- an explicit entry in `policyCapabilityRiskScores`
- an explicit capability-specific entry in `policyCapabilityRiskDescriptions`

The server validates this at startup through `ValidatePolicyCatalog()` and refuses to start if any policy capability is missing a risk score, has an invalid score, has a missing or empty risk description, or if a risk score/description exists for an unknown capability.
