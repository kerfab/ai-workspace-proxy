<!-- capability: gmail_drafts_write -->
### Create And Edit Gmail Drafts

Allowed: create, update, and delete drafts. This does not send email unless the send capability is also present.

Example commands:

Use this when the user asks to prepare an email but not send it. Create `/tmp/body.txt` first.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail create-draft --to person@example.com --subject 'Subject' --body-file /tmp/body.txt`

Use this to modify an existing draft. `DRAFT_ID` comes from `list-drafts`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail update-draft --id DRAFT_ID --to person@example.com --subject 'Updated subject' --body-file /tmp/body.txt`

Use this to draft a reply. `MESSAGE_ID` comes from search or unread results.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail reply-draft --message-id MESSAGE_ID --body-file /tmp/body.txt`

Use this to delete a draft. Replace `DRAFT_ID`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method DELETE --path /gmail.googleapis.com/gmail/v1/users/me/drafts/DRAFT_ID`
<!-- /capability -->

<!-- capability: gmail_send -->
### Send Gmail Messages

Allowed: send Gmail messages and drafts. Use only when the user explicitly asks to send.

Example commands:

Use this to send an existing draft. Create `/tmp/send_draft.json` with the Gmail draft send body.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/drafts/send --body-file /tmp/send_draft.json`

Use this to send a raw message. Create `/tmp/send_message.json` with the Gmail message send body.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/messages/send --body-file /tmp/send_message.json`
<!-- /capability -->

<!-- capability: gmail_labels_manage -->
### Manage Gmail Labels

Allowed: create, rename, update, and delete labels.

Example commands:

Use this to create a label. Create `/tmp/label.json` with the label properties.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail create-label --body-file /tmp/label.json`

Use this to rename or change a label. `LABEL_ID` comes from `list-labels`; create `/tmp/label_patch.json`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail patch-label --id LABEL_ID --body-file /tmp/label_patch.json`

Use this to delete a label. Confirm with the user first.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail delete-label --id LABEL_ID`
<!-- /capability -->

<!-- capability: gmail_labels_apply_safe -->
### Apply Safe Gmail Label Changes

Allowed: add or remove ordinary labels while the proxy blocks destructive system-label changes.

Example commands:

Use this to mark one message as read. `MESSAGE_ID` comes from search or unread results.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail mark-read --message-id MESSAGE_ID`

Use this to archive one message by removing `INBOX`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail archive --message-id MESSAGE_ID`

Use this to mark an entire thread as read. `THREAD_ID` comes from a message result.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail mark-read --thread-id THREAD_ID`

Use this to archive an entire thread.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" gmail archive --thread-id THREAD_ID`
<!-- /capability -->

<!-- capability: gmail_messages_delete -->
### Delete Or Trash Gmail Messages

Allowed: trash, untrash, permanently delete, or batch-delete messages.

Example commands:

Use this to move a message to trash. Prefer trash over permanent delete.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/messages/MESSAGE_ID/trash`

Use this to restore a trashed message.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/messages/MESSAGE_ID/untrash`

Use this only after explicit confirmation to permanently delete.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method DELETE --path /gmail.googleapis.com/gmail/v1/users/me/messages/MESSAGE_ID`
<!-- /capability -->

<!-- capability: gmail_messages_import -->
### Import Gmail Messages

Allowed: import or insert messages into the mailbox.

Example commands:

Use this to import a message. Create `/tmp/import_message.json` with the Gmail import body.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/messages/import --body-file /tmp/import_message.json`

Use this to insert a message directly. Create `/tmp/insert_message.json`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/messages --body-file /tmp/insert_message.json`
<!-- /capability -->

<!-- capability: gmail_watch_manage -->
### Manage Gmail Change Notifications

Allowed: start or stop Gmail push notification watches.

Example commands:

Use this to start a watch. Create `/tmp/watch.json` with the Pub/Sub topic and label filters.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/watch --body-file /tmp/watch.json`

Use this to stop the active watch.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/stop`
<!-- /capability -->
