<!-- capability: docs_read -->
### Read Google Docs

Allowed: read Google Docs located in allowed Drive folders.

Example commands:

Use this after Drive metadata search returns a Google Doc file ID, or after the user provides that document ID.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" docs get --document-id DOCUMENT_ID`

Use this to export a Google Doc to another format.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive export --file-id DOCUMENT_ID --mime-type application/pdf --save-to /tmp/document.pdf`
<!-- /capability -->

<!-- capability: docs_create -->
### Create Google Docs

Allowed: create Google Docs inside allowed Drive folders.

Example commands:

Use this to create a Doc in the reference root.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" docs create --ref "REFERENCE_NAME" --title "Document title"`

Use this to create a Doc in a cached subfolder.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" docs create --ref "REFERENCE_NAME" --drive-path "Relative/Subfolder" --title "Document title"`
<!-- /capability -->

<!-- capability: docs_edit -->
### Edit Google Docs

Allowed: apply Docs batch updates to documents in allowed Drive folders.

Example commands:

Use this after creating `/tmp/docs_requests.json` with Google Docs batchUpdate requests.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" docs update --document-id DOCUMENT_ID --requests-file /tmp/docs_requests.json`
<!-- /capability -->

<!-- capability: docs_delete -->
### Delete Google Docs

Allowed: permanently delete Google Docs in allowed Drive folders.

Example command:

Use this only after explicit user confirmation. `DOCUMENT_ID` comes from Drive metadata search, create output, or the user.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method DELETE --path /drive.googleapis.com/drive/v3/files/DOCUMENT_ID`
<!-- /capability -->
