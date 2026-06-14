<!-- capability: slides_read -->
### Read Google Slides

Allowed: read Google Slides presentations, pages, and thumbnails in allowed Drive folders.

Example commands:

Use this after Drive metadata search returns a Slides presentation file ID, or after the user provides that presentation ID.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" slides get --presentation-id PRESENTATION_ID`

Use this to export Google Slides to another format.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive export --file-id PRESENTATION_ID --mime-type application/pdf --save-to /tmp/presentation.pdf`

Use `proxy request` for page and thumbnail reads.
<!-- /capability -->

<!-- capability: slides_create -->
### Create Google Slides

Allowed: create Google Slides presentations inside allowed Drive folders.

Example commands:

Use this to create Slides in the reference root.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" slides create --ref "REFERENCE_NAME" --title "Presentation title"`

Use this to create Slides in a cached subfolder.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" slides create --ref "REFERENCE_NAME" --drive-path "Relative/Subfolder" --title "Presentation title"`
<!-- /capability -->

<!-- capability: slides_edit -->
### Edit Google Slides

Allowed: apply Slides batch updates to presentations in allowed Drive folders.

Example commands:

Use this after creating `/tmp/slides_requests.json` with Google Slides batchUpdate requests.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" slides update --presentation-id PRESENTATION_ID --requests-file /tmp/slides_requests.json`
<!-- /capability -->

<!-- capability: slides_delete -->
### Delete Google Slides

Allowed: permanently delete Google Slides in allowed Drive folders.

Example command:

Use this only after explicit user confirmation. `PRESENTATION_ID` comes from Drive metadata search, create output, or the user.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method DELETE --path /drive.googleapis.com/drive/v3/files/PRESENTATION_ID`
<!-- /capability -->
