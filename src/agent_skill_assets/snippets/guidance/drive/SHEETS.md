## How To Use These Examples

- Commands are examples. Replace uppercase placeholders such as `SPREADSHEET_ID` with real values from earlier command output.
- The `--workspace` value is already set to this workspace's friendly name. Keep it unless the user explicitly asks for another workspace.
- File paths such as `/tmp/values.json` or `/tmp/sheets_requests.json` mean the agent must create that local file before running the command.
- `REFERENCE_NAME` is a Drive folder reference configured in the proxy UI. `SPREADSHEET_ID` comes from Drive search, Sheets create, or Sheets read responses.
- `--drive-path` values are Google Drive subfolder paths under `REFERENCE_NAME`, not local filesystem paths.
