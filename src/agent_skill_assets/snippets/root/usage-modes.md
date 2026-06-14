## Usage Modes

This skill supports two ways to use Google Workspace through the proxy.

### Helper Mode

Use helper commands for common workflows documented in the service files. Helpers are convenience wrappers around proxy calls; they are not the complete Google Workspace API.

Helper command shape:

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" PRODUCT COMMAND [OPTIONS]`

### Full Passthrough Mode

Use full passthrough when no helper command exists for an operation that is covered by the policy-filtered service file.

1. Read the official Google Workspace API documentation for the needed product to determine the exact HTTP method, path, query parameters, and JSON body.
2. Convert the Google API URL into a proxy path by prefixing it with the Google API host path.
3. Send the request through `proxy request`.

Examples of URL-to-proxy-path conversion:

- `https://gmail.googleapis.com/gmail/v1/users/me/messages` becomes `/gmail.googleapis.com/gmail/v1/users/me/messages`
- `https://www.googleapis.com/calendar/v3/calendars/primary/events` becomes `/calendar.googleapis.com/calendar/v3/calendars/primary/events`
- `https://www.googleapis.com/drive/v3/files` becomes `/drive.googleapis.com/drive/v3/files`
- `https://docs.googleapis.com/v1/documents/DOCUMENT_ID` becomes `/docs.googleapis.com/v1/documents/DOCUMENT_ID`
- `https://sheets.googleapis.com/v4/spreadsheets/SPREADSHEET_ID` becomes `/sheets.googleapis.com/v4/spreadsheets/SPREADSHEET_ID`
- `https://slides.googleapis.com/v1/presentations/PRESENTATION_ID` becomes `/slides.googleapis.com/v1/presentations/PRESENTATION_ID`

Passthrough command shape:

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method METHOD --path /SERVICE.googleapis.com/API/PATH --query KEY=VALUE --body-file /tmp/request.json`

Only include `--query` or `--body-file` when the Google API request needs them.
