<!-- capability: drive_files_read -->
### Read Drive Files

Allowed: search, read metadata, export, and download files inside allowed Drive folders.

Example commands:

Use this to search an allowed root folder. `REFERENCE_NAME` is configured in the proxy UI.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" drive search --query "name contains 'Report'" --ref "REFERENCE_NAME"`

Use this to search a cached subfolder. `drive-path` is relative to the reference root.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" drive search --query "name contains 'Report'" --ref "REFERENCE_NAME" --drive-path "Relative/Subfolder"`

Use this after search returns a file `id`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" drive get-file --file-id FILE_ID`

Use this to download binary or non-Google file content.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" drive download --file-id FILE_ID --save-to /tmp/file.bin`

Use this to export a Google Docs/Sheets/Slides file to another format.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" drive export --file-id FILE_ID --mime-type application/pdf --save-to /tmp/file.pdf`

Use this to inspect cached subfolders under a reference.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" drive list-folders --ref "REFERENCE_NAME"`

Use this when the cached subfolder list looks stale or incomplete.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" drive refresh-folders --ref "REFERENCE_NAME"`
<!-- /capability -->

<!-- capability: drive_files_create -->
### Create Drive Files

Allowed: create files inside allowed Drive folders.

Example commands:

Use this to create a file in the reference root.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" drive create-file --ref "REFERENCE_NAME" --name "Notes.txt" --mime-type text/plain`

Use this to create a file in a cached subfolder.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" drive create-file --ref "REFERENCE_NAME" --drive-path "Relative/Subfolder" --name "Notes.txt" --mime-type text/plain`

Use `proxy request` for Drive copy operations; `FILE_ID` comes from search.
<!-- /capability -->

<!-- capability: drive_files_update -->
### Edit Drive Files

Allowed: rename, update metadata, or upload new content for files in allowed Drive folders.

Example commands:

Use this to rename a file. `FILE_ID` comes from search or create output.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" drive update-file --file-id FILE_ID --name "Updated name"`

Use this to apply metadata from JSON. Create `/tmp/drive_metadata.json` first.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" drive update-file --file-id FILE_ID --body-file /tmp/drive_metadata.json`
<!-- /capability -->

<!-- capability: drive_files_delete -->
### Trash Or Delete Drive Files

Allowed: trash or permanently delete files in allowed Drive folders.

Example commands:

Use this only after user confirmation. `FILE_ID` comes from search.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method DELETE --path /drive.googleapis.com/drive/v3/files/FILE_ID`

Use this only if the user explicitly asks to empty trash.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method DELETE --path /drive.googleapis.com/drive/v3/files/trash`
<!-- /capability -->

<!-- capability: drive_permissions_read -->
### Read Drive Sharing

Allowed: read Drive file permissions without changing them.

Example commands:

Use this to list sharing permissions for a file.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/files/FILE_ID/permissions`

Use this after the permission list returns a permission ID.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/files/FILE_ID/permissions/PERMISSION_ID`
<!-- /capability -->

<!-- capability: drive_permissions_manage -->
### Manage Drive Sharing

Allowed: create, update, or delete Drive file permissions.

Example commands:

Use this to add a sharing permission. Create `/tmp/permission.json`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /drive.googleapis.com/drive/v3/files/FILE_ID/permissions --body-file /tmp/permission.json`

Use this to edit a sharing permission. `PERMISSION_ID` comes from the permission list.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method PATCH --path /drive.googleapis.com/drive/v3/files/FILE_ID/permissions/PERMISSION_ID --body-file /tmp/permission_patch.json`

Use this to remove a sharing permission only after user confirmation.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method DELETE --path /drive.googleapis.com/drive/v3/files/FILE_ID/permissions/PERMISSION_ID`
<!-- /capability -->

<!-- capability: drive_comments_read -->
### Read Drive Comments

Allowed: read Drive file comments and replies without changing them.

Example commands:

Use this to list comments on a file.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/files/FILE_ID/comments`

Use this to list replies under one comment. `COMMENT_ID` comes from comments output.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/files/FILE_ID/comments/COMMENT_ID/replies`
<!-- /capability -->

<!-- capability: drive_comments_manage -->
### Manage Drive Comments

Allowed: create, update, delete, and reply to Drive file comments.

Example commands:

Use this to add a comment. Create `/tmp/comment.json`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /drive.googleapis.com/drive/v3/files/FILE_ID/comments --body-file /tmp/comment.json`

Use this to reply to a comment. `COMMENT_ID` comes from comments output.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /drive.googleapis.com/drive/v3/files/FILE_ID/comments/COMMENT_ID/replies --body-file /tmp/reply.json`

Use this to delete a comment only after user confirmation.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method DELETE --path /drive.googleapis.com/drive/v3/files/FILE_ID/comments/COMMENT_ID`
<!-- /capability -->

<!-- capability: drive_metadata_read -->
### Read Drive Metadata

Allowed: read Drive changes, shared drives, apps, labels, and file revisions.

Example commands:

Use this to list Drive changes.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/changes`

Use this to list shared drives.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/drives`

Use this to list file revisions. `FILE_ID` comes from search.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/files/FILE_ID/revisions`
<!-- /capability -->

<!-- capability: drive_metadata_manage -->
### Manage Drive Metadata

Allowed: hide shared drives, update labels, and delete Drive revisions.

Example commands:

Use this to hide a shared drive. `DRIVE_ID` comes from shared-drive output.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /drive.googleapis.com/drive/v3/drives/DRIVE_ID/hide`

Use this to modify labels. Create `/tmp/label_modifications.json`.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method POST --path /drive.googleapis.com/drive/v3/files/FILE_ID/modifyLabels --body-file /tmp/label_modifications.json`

Use this to delete a revision only after user confirmation.

`python3 ./scripts/workspace_proxy_tool.py --workspace "WORKSPACE" proxy request --method DELETE --path /drive.googleapis.com/drive/v3/files/FILE_ID/revisions/REVISION_ID`
<!-- /capability -->
