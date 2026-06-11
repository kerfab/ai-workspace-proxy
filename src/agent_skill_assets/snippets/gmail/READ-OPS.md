<!-- capability: gmail_profile_read -->
### Read Gmail Profile

Allowed: read mailbox profile and Gmail history metadata.

Example commands:

Use this to confirm which mailbox is connected.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /gmail.googleapis.com/gmail/v1/users/me/profile`

Use this only when you need Gmail change-history metadata. Add query parameters with `--query KEY=VALUE`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /gmail.googleapis.com/gmail/v1/users/me/history`
<!-- /capability -->

<!-- capability: gmail_messages_read -->
### Read Gmail Messages

Allowed: search, list, read messages and threads, and download attachments.

Example commands:

Use this to search Gmail. Replace the query with Gmail search terms from the user request.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail search --query 'from:person@example.com newer_than:7d' --max-results 10`

Use this to triage unread mail.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail unread --max-results 10`

Use this after search/unread returns a message `id`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail get-message --id MESSAGE_ID --format metadata`

Use this after a message response returns a `threadId`, or when the user asks for the full conversation.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail get-thread --id THREAD_ID --format full`

Use `proxy request` for attachments. `MESSAGE_ID` and `ATTACHMENT_ID` come from message metadata.
<!-- /capability -->

<!-- capability: gmail_drafts_read -->
### Read Gmail Drafts

Allowed: list and read existing drafts without changing them.

Example commands:

Use this to find draft IDs.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail list-drafts`

Use this after `list-drafts` returns a draft `id`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail get-draft --id DRAFT_ID`
<!-- /capability -->

<!-- capability: gmail_labels_read -->
### Read Gmail Labels

Allowed: list and read Gmail label definitions.

Example commands:

Use this to find available label IDs.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail list-labels`

Use this after `list-labels` returns a label `id`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail get-label --id LABEL_ID`
<!-- /capability -->
