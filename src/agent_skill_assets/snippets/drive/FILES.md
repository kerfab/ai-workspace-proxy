<!-- capability: drive_files_read -->
### Read Drive Files

Allowed: read metadata and download non-Google-native files inside allowed Drive folders. Google Docs, Sheets, and Slides use their own service files.

Example commands:

Use this after Drive metadata search returns a non-Google-native file `id`, or after the user provides that file `id`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive get-file --file-id FILE_ID`

Use this to download non-Google-native file content.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive download --file-id FILE_ID --save-to /tmp/file.bin`

Use this to inspect cached subfolders under a reference.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive list-folders --ref "REFERENCE_NAME"`

Use this when the cached subfolder list looks stale or incomplete.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive refresh-folders --ref "REFERENCE_NAME"`
<!-- /capability -->

<!-- capability: drive_files_create -->
### Create Drive Files

Allowed: create non-Google-native files inside allowed Drive folders.

Example commands:

Use this to create a file in the reference root.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive create-file --ref "REFERENCE_NAME" --name "Notes.txt" --mime-type text/plain`

Use this to create a file in a cached subfolder.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive create-file --ref "REFERENCE_NAME" --drive-path "Relative/Subfolder" --name "Notes.txt" --mime-type text/plain`

Use `proxy request` for Drive copy operations; `FILE_ID` comes from search.
<!-- /capability -->

<!-- capability: drive_files_update -->
### Edit Drive Files

Allowed: rename, update metadata, or upload new content for non-Google-native files in allowed Drive folders.

Example commands:

Use this to rename a file. `FILE_ID` comes from search or create output.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive update-file --file-id FILE_ID --name "Updated name"`

Use this to apply metadata from JSON. Create `/tmp/drive_metadata.json` first.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive update-file --file-id FILE_ID --body-file /tmp/drive_metadata.json`
<!-- /capability -->

<!-- capability: drive_files_delete -->
### Delete Drive Files

Allowed: permanently delete non-Google-native files in allowed Drive folders.

Example commands:

Use this only after user confirmation. `FILE_ID` comes from search.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method DELETE --path /drive.googleapis.com/drive/v3/files/FILE_ID`

<!-- /capability -->

<!-- capability: drive_permissions_read -->
### Read Drive Sharing

Allowed: read Drive file permissions without changing them.

Example commands:

Use this to list sharing permissions for a file.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/files/FILE_ID/permissions`

Use this after the permission list returns a permission ID.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/files/FILE_ID/permissions/PERMISSION_ID`
<!-- /capability -->

<!-- capability: drive_permissions_manage -->
### Manage Drive Sharing

Allowed: create, update, or delete Drive file permissions.

Example commands:

Use this to add a sharing permission. Create `/tmp/permission.json`.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /drive.googleapis.com/drive/v3/files/FILE_ID/permissions --body-file /tmp/permission.json`

Use this to edit a sharing permission. `PERMISSION_ID` comes from the permission list.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method PATCH --path /drive.googleapis.com/drive/v3/files/FILE_ID/permissions/PERMISSION_ID --body-file /tmp/permission_patch.json`

Use this to remove a sharing permission only after user confirmation.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method DELETE --path /drive.googleapis.com/drive/v3/files/FILE_ID/permissions/PERMISSION_ID`
<!-- /capability -->

<!-- capability: drive_comments_read -->
### Read Drive Comments

Allowed: read Drive file comments and replies without changing them.

Example commands:

Use this to list comments on a file.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/files/FILE_ID/comments --query fields=comments(id,content,htmlContent,author,createdTime,modifiedTime,resolved)`

Use this to list replies under one comment. `COMMENT_ID` comes from comments output.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/files/FILE_ID/comments/COMMENT_ID/replies --query fields=replies(id,content,htmlContent,author,createdTime,modifiedTime,action)`
<!-- /capability -->

<!-- capability: drive_comments_create -->
### Create Drive Comments

Allowed: add new comments to Drive files in allowed folders.

Example command:

Use this to add a plain comment. Replace `FILE_ID` with the file id from search or file metadata.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive create-comment --file-id FILE_ID --content "COMMENT_TEXT"`
<!-- /capability -->

<!-- capability: drive_comments_reply -->
### Reply To Drive Comments

Allowed: add normal replies to existing Drive file comments. This does not resolve the comment.

Example command:

Use this to reply to a comment. Replace `FILE_ID` and `COMMENT_ID` with values from comments output.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive reply-comment --file-id FILE_ID --comment-id COMMENT_ID --content "REPLY_TEXT"`
<!-- /capability -->

<!-- capability: drive_comments_resolve -->
### Resolve Drive Comments

Allowed: mark Drive comment discussions as resolved by posting a resolving reply.

Example command:

Use this only when the user clearly asks to resolve the discussion. Replace `FILE_ID` and `COMMENT_ID` with values from comments output.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive resolve-comment --file-id FILE_ID --comment-id COMMENT_ID --content "RESOLUTION_TEXT"`
<!-- /capability -->

<!-- capability: drive_comments_update -->
### Update Own Drive Comments And Replies

Allowed: edit only comments and replies authored by the connected Workspace account. The proxy checks ownership before forwarding.

Example commands:

Use this to update a comment that the connected Workspace account authored.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive update-comment --file-id FILE_ID --comment-id COMMENT_ID --content "UPDATED_COMMENT_TEXT"`

Use this to update a reply that the connected Workspace account authored.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive update-reply --file-id FILE_ID --comment-id COMMENT_ID --reply-id REPLY_ID --content "UPDATED_REPLY_TEXT"`
<!-- /capability -->

<!-- capability: drive_comments_delete -->
### Delete Own Drive Comments And Replies

Allowed: delete only comments and replies authored by the connected Workspace account. The proxy checks ownership before forwarding.

Example commands:

Use this only after explicit user confirmation to delete a comment authored by the connected Workspace account.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive delete-comment --file-id FILE_ID --comment-id COMMENT_ID`

Use this only after explicit user confirmation to delete a reply authored by the connected Workspace account.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive delete-reply --file-id FILE_ID --comment-id COMMENT_ID --reply-id REPLY_ID`
<!-- /capability -->

<!-- capability: drive_metadata_read -->
### Read Drive Metadata

Allowed: search allowed Drive folders and read Drive changes, account info, shared drives, apps, file labels, and file revisions. This reads metadata, not file content.

Example commands:

Use this to search an allowed root folder. `REFERENCE_NAME` is configured in the proxy UI.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive search --query "name contains 'Report'" --ref "REFERENCE_NAME"`

Use this to search a cached subfolder. `drive-path` is relative to the reference root.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" drive search --query "name contains 'Report'" --ref "REFERENCE_NAME" --drive-path "Relative/Subfolder"`

Use this to list Drive changes.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/changes`

Use this to list shared drives.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/drives`

Use this to list labels applied to a file. `FILE_ID` comes from search.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/files/FILE_ID/listLabels`

Use this to list file revisions. `FILE_ID` comes from search.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method GET --path /drive.googleapis.com/drive/v3/files/FILE_ID/revisions`
<!-- /capability -->

<!-- capability: drive_labels_update -->
### Update Drive Labels

Allowed: apply, update, or remove Drive labels on files.

Example command:

Use this to modify labels applied to a file. Create `/tmp/label_modifications.json` first. `FILE_ID` comes from search or file metadata.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method POST --path /drive.googleapis.com/drive/v3/files/FILE_ID/modifyLabels --body-file /tmp/label_modifications.json`
<!-- /capability -->

<!-- capability: drive_revisions_delete -->
### Delete Drive Revisions

Allowed: permanently delete Drive file revisions where Google Drive permits it.

Example command:

Use this only after explicit user confirmation. `FILE_ID` and `REVISION_ID` come from revision output. Google Drive only permits revision deletion for some file types and revision states.

`python3 "{baseDir}/scripts/workspace_proxy_tool.py" --workspace "WORKSPACE" proxy request --method DELETE --path /drive.googleapis.com/drive/v3/files/FILE_ID/revisions/REVISION_ID`
<!-- /capability -->
