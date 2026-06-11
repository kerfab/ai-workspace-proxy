<!-- capability: sheets_read -->
### Read Google Sheets

Allowed: read Google Sheets spreadsheets and values in allowed Drive folders.

Example commands:

Use this after Drive search returns a Google Sheet file ID.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" sheets get --spreadsheet-id SPREADSHEET_ID`

Use `proxy request` for value reads, batch reads, and developer metadata reads.
<!-- /capability -->

<!-- capability: sheets_create -->
### Create Google Sheets

Allowed: create Google Sheets inside allowed Drive folders.

Example commands:

Use this to create a Sheet in the reference root.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" sheets create --ref "REFERENCE_NAME" --title "Spreadsheet title"`

Use this to create a Sheet in a cached subfolder.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" sheets create --ref "REFERENCE_NAME" --drive-path "Relative/Subfolder" --title "Spreadsheet title"`
<!-- /capability -->

<!-- capability: sheets_edit -->
### Edit Google Sheets

Allowed: update spreadsheet structure, metadata, and cell values in allowed Drive folders.

Example commands:

Use this to update cells. Create `/tmp/values.json` with a top-level `values` array.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" sheets values-update --spreadsheet-id SPREADSHEET_ID --range 'Sheet1!A1:B2' --values-file /tmp/values.json`

Use this for structural spreadsheet changes. Create `/tmp/sheets_requests.json`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" sheets batch-update --spreadsheet-id SPREADSHEET_ID --requests-file /tmp/sheets_requests.json`

Use `proxy request` for append, clear, batch clear, and data filter operations.
<!-- /capability -->
