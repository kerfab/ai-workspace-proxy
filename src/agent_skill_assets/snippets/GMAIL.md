<!-- capability: gmail_profile_read -->
### Read Profile

Allowed: read mailbox profile and history metadata.

Example commands:

Use this to confirm which mailbox is connected.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /gmail.googleapis.com/gmail/v1/users/me/profile`

Use this only when you need change-history metadata. Add query parameters with `--query KEY=VALUE`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /gmail.googleapis.com/gmail/v1/users/me/history`
<!-- /capability -->

<!-- capability: gmail_messages_read -->
### Read Emails

Allowed: search, list, read emails and threads, and download attachments.

Example commands:

Use this to search emails. Replace the query with search terms from the user request.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail search --query 'from:person@example.com newer_than:7d' --max-results 10`

Use this to list unread email. For the latest unread email, use `--max-results 1`, then pass the returned Gmail API `message.id` to `gmail get-message`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail unread --max-results 10`

Use this after search/unread returns a Gmail API `message.id`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail get-message --id MESSAGE_ID --format metadata`

Use this after an email response returns a `threadId`, or when the user asks for the full conversation.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail get-thread --id THREAD_ID --format full`

Use `proxy request` for attachments. `MESSAGE_ID` and `ATTACHMENT_ID` come from email metadata.
<!-- /capability -->

<!-- capability: gmail_drafts_read -->
### Read Drafts

Allowed: list and read existing drafts without changing them.

Example commands:

Use this to find draft IDs.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail list-drafts`

Use this after `list-drafts` returns a draft `id`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail get-draft --id DRAFT_ID`
<!-- /capability -->

<!-- capability: gmail_labels_read -->
### Read Labels

Allowed: list and read label definitions.

Example commands:

Use this to find available label IDs.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail list-labels`

Use this after `list-labels` returns a label `id`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail get-label --id LABEL_ID`
<!-- /capability -->

<!-- capability: gmail_drafts_write -->
### Create And Edit Drafts

Allowed: create and update drafts. This does not delete drafts or send email.

Example commands:

Use this when the user asks to prepare an email but not send it. Create `/tmp/body.txt` first.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail create-draft --to person@example.com --subject 'Subject' --body-file /tmp/body.txt`

Use this to modify an existing draft. `DRAFT_ID` comes from `list-drafts`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail update-draft --id DRAFT_ID --to person@example.com --subject 'Updated subject' --body-file /tmp/body.txt`

Use this to draft a reply. `MESSAGE_ID` comes from search or unread results.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail reply-draft --message-id MESSAGE_ID --body-file /tmp/body.txt`
<!-- /capability -->

<!-- capability: gmail_drafts_delete -->
### Delete Drafts

Allowed: delete existing drafts. This does not send email.

Example commands:

Use this only after explicit confirmation. `DRAFT_ID` comes from `list-drafts`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail delete-draft --id DRAFT_ID`
<!-- /capability -->

<!-- capability: gmail_send -->
### Send Emails

Allowed: send emails and drafts. Use only when the user explicitly asks to send.

Example commands:

Use this to send an existing draft. Create `/tmp/send_draft.json` with the draft send body.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/drafts/send --body-file /tmp/send_draft.json`

Use this to send a raw email. Create `/tmp/send_message.json` with the email send body.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/messages/send --body-file /tmp/send_message.json`
<!-- /capability -->

<!-- capability: gmail_labels_apply_custom -->
### Apply Or Remove Custom Labels

Allowed: apply or remove user-created labels on emails and threads. `LABEL_ID` must be a custom label ID from `list-labels`, usually starting with `Label_`.

Example commands:

Use this to apply a custom label to one email.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail apply-label --message-id MESSAGE_ID --label-id LABEL_ID`

Use this to remove a custom label from an entire thread.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail remove-label --thread-id THREAD_ID --label-id LABEL_ID`
<!-- /capability -->

<!-- capability: gmail_messages_archive -->
### Archive Or Unarchive Emails

Allowed: archive emails by removing Inbox, or unarchive them by restoring Inbox. This does not delete emails.

Example commands:

Use this to archive one email.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail archive --message-id MESSAGE_ID`

Use this to unarchive an entire thread.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail unarchive --thread-id THREAD_ID`
<!-- /capability -->

<!-- capability: gmail_messages_status -->
### Change Email Status

Allowed: mark emails read or unread, starred or unstarred, and important or not important.

Example commands:

Use this to mark one email as read. `MESSAGE_ID` comes from search or unread results.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail mark-read --message-id MESSAGE_ID`

Use this to mark an entire thread as unread.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail mark-unread --thread-id THREAD_ID`

Use this to star one email.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail star --message-id MESSAGE_ID`

Use this to mark one email as not important.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail mark-not-important --message-id MESSAGE_ID`
<!-- /capability -->

<!-- capability: gmail_messages_spam -->
### Move Emails To Or From Spam

Allowed: mark emails or threads as spam, or remove them from spam.

Example commands:

Use this to mark one email as spam.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail mark-spam --message-id MESSAGE_ID`

Use this to remove an entire thread from spam.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail unmark-spam --thread-id THREAD_ID`
<!-- /capability -->

<!-- capability: gmail_messages_trash -->
### Move Emails To Or From Trash

Allowed: move emails and threads to Trash or restore them from Trash. This is different from permanent deletion.

Example commands:

Use this to move one email to trash. Prefer trash over permanent delete.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail trash --message-id MESSAGE_ID`

Use this to restore a trashed thread.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail untrash --thread-id THREAD_ID`
<!-- /capability -->

<!-- capability: gmail_labels_create_rename -->
### Create Or Rename Labels

Allowed: create user labels and rename existing user labels. System labels cannot be created or renamed.

Example commands:

Use this to create a label. Create `/tmp/label.json` with the label properties.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail create-label --body-file /tmp/label.json`

Use this to rename a label. `LABEL_ID` comes from `list-labels`; create `/tmp/label_patch.json`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail patch-label --id LABEL_ID --body-file /tmp/label_patch.json`
<!-- /capability -->

<!-- capability: gmail_labels_delete -->
### Delete Labels

Allowed: permanently delete user labels. System labels cannot be deleted.

Example commands:

Use this only after explicit confirmation. `LABEL_ID` comes from `list-labels`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail delete-label --id LABEL_ID`
<!-- /capability -->

<!-- capability: gmail_messages_delete -->
### Permanently Delete Emails

Allowed: permanently delete or batch-delete emails without Trash recovery.

Example commands:

Use this only after explicit confirmation to permanently delete.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" gmail delete-message --message-id MESSAGE_ID`

Use this only after explicit confirmation to permanently delete multiple emails. Create `/tmp/batch_delete.json` with `ids`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/messages/batchDelete --body-file /tmp/batch_delete.json`
<!-- /capability -->

<!-- capability: gmail_messages_import -->
### Import Emails

Allowed: import or insert emails into the mailbox.

Example commands:

Use this to import an email. Create `/tmp/import_message.json` with the import body.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/messages/import --body-file /tmp/import_message.json`

Use this to insert an email directly. Create `/tmp/insert_message.json`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/messages --body-file /tmp/insert_message.json`
<!-- /capability -->

<!-- capability: gmail_watch_manage -->
### Manage Change Notifications

Allowed: start or stop push notification watches.

Example commands:

Use this to start a watch. Create `/tmp/watch.json` with the Pub/Sub topic and label filters.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/watch --body-file /tmp/watch.json`

Use this to stop the active watch.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /gmail.googleapis.com/gmail/v1/users/me/stop`
<!-- /capability -->
