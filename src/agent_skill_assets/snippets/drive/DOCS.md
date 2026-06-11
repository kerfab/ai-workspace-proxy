<!-- capability: docs_read -->
### Read Google Docs

Allowed: read Google Docs located in allowed Drive folders.

Example commands:

Use this after Drive search returns a Google Doc file ID.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" docs get --document-id DOCUMENT_ID`
<!-- /capability -->

<!-- capability: docs_create -->
### Create Google Docs

Allowed: create Google Docs inside allowed Drive folders.

Example commands:

Use this to create a Doc in the reference root.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" docs create --ref "REFERENCE_NAME" --title "Document title"`

Use this to create a Doc in a cached subfolder.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" docs create --ref "REFERENCE_NAME" --drive-path "Relative/Subfolder" --title "Document title"`
<!-- /capability -->

<!-- capability: docs_edit -->
### Edit Google Docs

Allowed: apply Docs batch updates to documents in allowed Drive folders.

Example commands:

Use this after creating `/tmp/docs_requests.json` with Google Docs batchUpdate requests.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" docs update --document-id DOCUMENT_ID --requests-file /tmp/docs_requests.json`
<!-- /capability -->
