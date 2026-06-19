# Drive, Docs, Sheets, and Slides

Read only the file that matches the user's task:

- `{baseDir}/skills/{{WORKSPACE_EMAIL}}/drive/FILES.md`: Drive metadata and Other file types operations - search metadata, read/download other file types, create/edit/delete other file types, share, comment, label, and inspect revisions.
- `{baseDir}/skills/{{WORKSPACE_EMAIL}}/drive/DOCS.md`: Google Docs operations - read, export, create, edit, and delete Google Docs.
- `{baseDir}/skills/{{WORKSPACE_EMAIL}}/drive/SHEETS.md`: Google Sheets operations - read, export, create, edit, and delete Google Sheets.
- `{baseDir}/skills/{{WORKSPACE_EMAIL}}/drive/SLIDES.md`: Google Slides operations - read, export, create, edit, and delete Google Slides.

If the needed operation is not covered by an allowed capability in the matching file, treat it as unavailable for this workspace unless the user asks to refresh/update the skill package.

If the operation is covered by an allowed capability but no helper example is shown, use full passthrough mode with the official Google Workspace API documentation.
