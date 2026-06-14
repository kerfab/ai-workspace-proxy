## How To Use These Examples

- Commands are examples. Replace uppercase placeholders such as `MESSAGE_ID`, `THREAD_ID`, `DRAFT_ID`, or `LABEL_ID` with real values from earlier command output.
- The `--workspace` value is already set to this workspace's friendly name. Keep it unless the user explicitly asks for another workspace.
- File paths such as `/tmp/body.txt` or `/tmp/request.json` mean the agent must create that local file before running the command.
- Gmail API calls emails `messages`. In these examples, `MESSAGE_ID` means the Gmail ID of an email returned by search, unread, or thread output.
- Gmail IDs come from Gmail search, unread, thread, draft, or label responses.
- Use exact Gmail command names from this file. Do not invent commands such as `get-latest-unread-email`, and do not invent flags such as `--action`.
- For "latest unread email", first run `gmail unread --max-results 1`, then run `gmail get-message --id MESSAGE_ID --format metadata` with the returned Gmail API `message.id`.
