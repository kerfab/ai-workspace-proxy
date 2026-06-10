package main

import "html/template"

var (
	loginTemplate = template.Must(template.New("login").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>{{.AppName}}</title>
<style>
body{font-family:Arial,sans-serif;max-width:960px;margin:2rem auto;padding:0 1rem;}
.card{border:1px solid #ddd;border-radius:8px;padding:1rem 1.25rem;margin-bottom:1rem;}
.btn{display:inline-block;background:#0b57d0;color:#fff;padding:.6rem .9rem;border-radius:6px;text-decoration:none;border:none;cursor:pointer;font:inherit;font-size:.875rem;line-height:1.2;}
small,.small{color:#666;}
code{background:#f4f4f4;padding:.15rem .35rem;border-radius:4px;}
</style></head><body>
<h1>{{.AppName}}</h1>
<div class="card">
  <p>Sign in to the proxy with Google. Google Workspace access is a separate step after login.</p>
  <a class="btn" href="/auth/google/login">Sign in with Google</a>
</div>
<p class="small">Proxy endpoint prefixes: <code>/gmail.googleapis.com/</code>, <code>/calendar.googleapis.com/</code>, <code>/drive.googleapis.com/</code>, <code>/docs.googleapis.com/</code>, <code>/sheets.googleapis.com/</code>, <code>/slides.googleapis.com/</code></p>
</body></html>`))

	dashboardTemplate = template.Must(template.New("dash").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>{{.AppName}}</title>
<style>
body{font-family:Arial,sans-serif;max-width:1200px;margin:2rem auto;padding:0 1rem;}
.card{border:1px solid #ddd;border-radius:8px;padding:1rem 1.25rem;margin-bottom:1rem;}
.btn{display:inline-block;background:#0b57d0;color:#fff;padding:.6rem .9rem;border-radius:6px;text-decoration:none;border:none;cursor:pointer;font:inherit;font-size:.875rem;line-height:1.2;}
.btn.secondary{background:#444;}
.btn.warn{background:#b73239;}
code{background:#f4f4f4;padding:.15rem .35rem;border-radius:4px;}
.hidden{filter:blur(5px);user-select:none;}
small{color:#666;}
.error{color:#b00020;margin:.5rem 0 1rem 0;}
.inline-actions{display:flex;gap:.75rem;align-items:center;flex-wrap:wrap;}
table{border-collapse:collapse;width:100%;table-layout:fixed;}
th,td{border:1px solid #ddd;padding:.65rem;text-align:left;vertical-align:top;word-wrap:break-word;overflow-wrap:anywhere;}
.folder-id{display:inline-block;margin-top:.35rem;color:#666;font-size:.85rem;background:#f6f6f6;border-radius:4px;padding:.15rem .35rem;}
.folder-form{display:grid;grid-template-columns:minmax(180px,220px) minmax(260px,1fr) auto;gap:.75rem;align-items:start;}
.folder-form .checks{display:flex;gap:.75rem;flex-wrap:wrap;align-items:center;}
.folder-edit-grid{display:grid;grid-template-columns:minmax(160px,190px) minmax(260px,1fr);gap:.5rem 1rem;align-items:center;}
.folder-edit-grid .checks{grid-column:1 / span 2;display:flex;gap:.75rem;flex-wrap:wrap;align-items:center;}
.folder-action-row{display:flex;gap:.6rem;flex-wrap:wrap;align-items:center;margin-top:.5rem;}
.discovery-overlay{position:fixed;inset:0;background:rgba(255,255,255,.86);display:none;align-items:center;justify-content:center;z-index:1000;padding:1rem;}
.discovery-overlay.active{display:flex;}
.discovery-box{max-width:420px;background:#fff;border:1px solid #ddd;border-radius:8px;padding:1.25rem 1.5rem;box-shadow:0 8px 28px rgba(0,0,0,.14);}
.discovery-box h2{margin:.1rem 0 .5rem 0;}
.discovery-box p{margin:.4rem 0;color:#444;}
.spinner{width:28px;height:28px;border:3px solid #d7d7d7;border-top-color:#0b57d0;border-radius:50%;animation:spin .9s linear infinite;margin-bottom:.75rem;}
@keyframes spin{to{transform:rotate(360deg);}}
input[type="text"]{box-sizing:border-box;max-width:100%;width:100%;padding:.35rem .45rem;}
label.checkbox{display:inline-flex;gap:.35rem;align-items:center;white-space:nowrap;}
@media (max-width: 900px){
  .folder-form{grid-template-columns:1fr;}
  .folder-edit-grid{grid-template-columns:1fr;}
  .folder-edit-grid .checks{grid-column:auto;}
  table,thead,tbody,tr,td,th{display:block;}
  thead{display:none;}
  tr{border:1px solid #ddd;margin-bottom:1rem;padding:.5rem;}
  td{border:none;padding:.35rem 0;}
}
</style>
<script>
function toggleToken(){
  const el = document.getElementById('proxyToken');
  if (!el.dataset.loaded) {
    fetch('/api/token/reveal',{credentials:'same-origin'})
      .then(r=>r.json())
      .then(d=>{ if(d.token){ el.textContent=d.token; el.dataset.loaded='1'; el.classList.remove('hidden'); } });
    return;
  }
  el.classList.toggle('hidden');
}
function confirmRotate(){
  return confirm('Rotate the Proxy API token? The old token will stop working immediately.');
}
function syncCheckboxFallback(prefix){
  const boxes = [
    document.getElementById(prefix + '_docs'),
    document.getElementById(prefix + '_sheets'),
    document.getElementById(prefix + '_slides'),
    document.getElementById(prefix + '_drive')
  ].filter(Boolean);
  if (boxes.length && boxes.every(b => !b.checked)) {
    boxes.forEach(b => b.checked = true);
  }
  return true;
}
function showDriveDiscoveryOverlay(){
  const overlay = document.getElementById('driveDiscoveryOverlay');
  if (overlay) {
    overlay.classList.add('active');
  }
  return true;
}
function prepareDriveFolderSubmit(prefix){
  if (!syncCheckboxFallback(prefix)) {
    return false;
  }
  return showDriveDiscoveryOverlay();
}
</script>
</head><body>
<div id="driveDiscoveryOverlay" class="discovery-overlay" aria-live="polite" aria-busy="true">
  <div class="discovery-box">
    <div class="spinner"></div>
    <h2>Discovering Drive folders</h2>
    <p>The proxy is discovering the folder tree and caching subfolders for agent access.</p>
    <p>This can take a few minutes for large Google Drive folders. The dashboard will return automatically when the operation finishes.</p>
  </div>
</div>
<h1>{{.AppName}}</h1>

<div class="card">
  <p><strong>User:</strong> {{.User.Email}}</p>
  <p><strong>Name:</strong> {{.User.Name}}</p>
  <form method="post" action="/logout"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><button class="btn secondary" type="submit">Logout</button></form>
</div>

<div class="card">
  <h2>Proxy API token</h2>
  <p><code id="proxyToken" class="hidden">{{.TokenHint}}</code></p>
  <div class="inline-actions">
    <button class="btn" type="button" onclick="toggleToken()">Show / hide token</button>
    <a class="btn" href="/api/token/download-config">Download JSON</a>
    <form method="post" action="/api/token/rotate" onsubmit="return confirmRotate();">
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      <button class="btn warn" type="submit">Rotate key</button>
    </form>
  </div>
  <p><small>Use with <code>Authorization: Bearer &lt;token&gt;</code></small></p>
</div>

<div class="card">
  <h2>Google Workspace connection</h2>
  {{if .WorkspaceConnected}}
    <p><strong>Account:</strong> {{.Workspace.AccountEmail}}</p>
    <p><strong>Scopes:</strong> {{.Workspace.Scopes}}</p>
    <p><strong>Connected since:</strong> {{.Workspace.ConnectedSince}}</p>
    <p><strong>Connection status:</strong> {{.Workspace.ConnectionStatus}}</p>
    <p><strong>Proxy API token:</strong> {{.Workspace.ProxyTokenStatus}}</p>
    <p><small>Workspace access is refreshed automatically and remains active until access is revoked or disconnected.</small></p>
    <form method="post" action="/auth/workspace/disconnect"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><button class="btn warn" type="submit">Disconnect Google Workspace</button></form>
  {{else}}
    <p>No Google Workspace account connected yet.</p>
    <a class="btn" href="/auth/workspace/connect">Connect Google Workspace</a>
  {{end}}
</div>

<div class="card">
  <h2>Allowed AI Drive folders</h2>
  <p><small>Use a unique Reference Name. Agents can refer to folders by that name. Registered folders include cached subfolders.</small></p>
  {{if .FolderError}}<p class="error">{{.FolderError}}</p>{{end}}
  <form method="post" action="/workspace/drive-folders/add" class="folder-form" onsubmit="return prepareDriveFolderSubmit('new');">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <input type="text" name="reference_name" placeholder="Reference Name" required>
    <input type="text" name="folder_link" placeholder="Google Drive folder link" required>
    <div class="checks">
      <label class="checkbox"><input id="new_docs" type="checkbox" name="allow_docs" checked> Docs</label>
      <label class="checkbox"><input id="new_sheets" type="checkbox" name="allow_sheets" checked> Sheets</label>
      <label class="checkbox"><input id="new_slides" type="checkbox" name="allow_slides" checked> Slides</label>
      <label class="checkbox"><input id="new_drive" type="checkbox" name="allow_drive_files" checked> Drive files</label>
      <button class="btn" type="submit">Add allowed folder</button>
    </div>
  </form>

  {{if .DriveFolders}}
  <table style="margin-top:1rem;">
    <thead>
      <tr>
        <th style="width:18%;">Reference Name</th>
        <th style="width:18%;">Folder</th>
        <th style="width:16%;">Allowed types</th>
        <th style="width:48%;">Actions</th>
      </tr>
    </thead>
    <tbody>
      {{range .DriveFolders}}
      <tr>
        <td>{{.ReferenceName}}</td>
        <td>
          <div>{{.FolderName}}</div>
          <div class="folder-id">{{.FolderID}}</div>
        </td>
        <td>
          {{if .AllowDocs}}Docs {{end}}
          {{if .AllowSheets}}Sheets {{end}}
          {{if .AllowSlides}}Slides {{end}}
          {{if .AllowDriveFiles}}Drive {{end}}
        </td>
        <td>
          <form id="update_{{.ID}}" method="post" action="/workspace/drive-folders/{{.ID}}/update" class="folder-edit-grid" onsubmit="return prepareDriveFolderSubmit('f_{{.ID}}');">
            <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
            <input type="text" name="reference_name" value="{{.ReferenceName}}" required>
            <input type="text" name="folder_link" value="{{.FolderURL}}" required>
            <div class="checks">
              <label class="checkbox"><input id="f_{{.ID}}_docs" type="checkbox" name="allow_docs" {{if .AllowDocs}}checked{{end}}> Docs</label>
              <label class="checkbox"><input id="f_{{.ID}}_sheets" type="checkbox" name="allow_sheets" {{if .AllowSheets}}checked{{end}}> Sheets</label>
              <label class="checkbox"><input id="f_{{.ID}}_slides" type="checkbox" name="allow_slides" {{if .AllowSlides}}checked{{end}}> Slides</label>
              <label class="checkbox"><input id="f_{{.ID}}_drive" type="checkbox" name="allow_drive_files" {{if .AllowDriveFiles}}checked{{end}}> Drive</label>
            </div>
          </form>
          <div class="folder-action-row">
            <button class="btn secondary" type="submit" form="update_{{.ID}}">Update</button>
            <form method="post" action="/workspace/drive-folders/{{.ID}}/refresh" onsubmit="return showDriveDiscoveryOverlay();">
              <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
              <button class="btn secondary" type="submit">Refresh tree</button>
            </form>
            <form method="post" action="/workspace/drive-folders/{{.ID}}/delete" onsubmit="return confirm('Delete this allowed folder reference?');">
              <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
              <button class="btn warn" type="submit">Delete</button>
            </form>
          </div>
        </td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{else}}
    <p><small>No allowed Drive folders configured yet.</small></p>
  {{end}}
</div>

<div class="card">
  <h2>Proxy endpoints</h2>
  <p><code>{{.BaseURL}}/gmail.googleapis.com/gmail/v1/users/me/messages</code></p>
  <p><code>{{.BaseURL}}/calendar.googleapis.com/calendar/v3/users/me/calendarList</code></p>
  <p><code>{{.BaseURL}}/drive.googleapis.com/drive/v3/files</code></p>
  <p><small>The proxy only forwards requests that match <code>policy.json</code>.</small></p>
</div>

{{if .User.IsAdmin}}
<div class="card">
  <h2>Admin</h2>
  <p><a class="btn secondary" href="/admin/users">Open admin section</a></p>
</div>
{{end}}
</body></html>`))

	adminUsersTemplate = template.Must(template.New("adminUsers").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>{{.AppName}} Admin</title>
<style>
body{font-family:Arial,sans-serif;max-width:1200px;margin:2rem auto;padding:0 1rem;}
.card{border:1px solid #ddd;border-radius:8px;padding:1rem .25rem;margin-bottom:1rem;}
.btn{display:inline-block;background:#0b57d0;color:#fff;padding:.45rem .7rem;border-radius:6px;text-decoration:none;border:none;cursor:pointer;}
.btn.secondary{background:#444;}
table{border-collapse:collapse;width:100%;}
th,td{border:1px solid #ddd;padding:.5rem;text-align:left;vertical-align:top;}
</style></head><body>
<h1>{{.AppName}} Admin</h1>
<p><a class="btn secondary" href="/">Back to dashboard</a></p>
<div class="card">
  <p><strong>Policy file:</strong> {{.PolicyPath}}</p>
  <p><strong>Denied log:</strong> {{.DeniedLogPath}}</p>
</div>
<div class="card">
<table>
<thead><tr><th>Email</th><th>Name</th><th>Admin</th><th>Suspended</th><th>Actions</th></tr></thead>
<tbody>
{{range .Users}}
<tr>
  <td><a href="/admin/users/{{.ID}}">{{.Email}}</a></td>
  <td>{{.Name}}</td>
  <td>{{.IsAdmin}}</td>
  <td>{{.IsSuspended}}</td>
  <td><a class="btn" href="/admin/users/{{.ID}}">Review</a></td>
</tr>
{{end}}
</tbody>
</table>
</div>
</body></html>`))

	adminUserTemplate = template.Must(template.New("adminUser").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>{{.AppName}} Admin User</title>
<style>
body{font-family:Arial,sans-serif;max-width:1080px;margin:2rem auto;padding:0 1rem;}
.card{border:1px solid #ddd;border-radius:8px;padding:1rem 1.25rem;margin-bottom:1rem;}
.btn{display:inline-block;background:#0b57d0;color:#fff;padding:.45rem .7rem;border-radius:6px;text-decoration:none;border:none;cursor:pointer;}
.btn.secondary{background:#444;}
.btn.warn{background:#a33;}
table{border-collapse:collapse;width:100%;}
th,td{border:1px solid #ddd;padding:.5rem;text-align:left;}
</style></head><body>
<h1>{{.AppName}} Admin</h1>
<p><a class="btn secondary" href="/admin/users">Back to users</a></p>
<div class="card">
  <p><strong>Email:</strong> {{.User.Email}}</p>
  <p><strong>Name:</strong> {{.User.Name}}</p>
  <p><strong>Admin:</strong> {{.User.IsAdmin}}</p>
  <p><strong>Suspended:</strong> {{.User.IsSuspended}}</p>
  <p><strong>Created:</strong> {{.User.CreatedAt}}</p>
</div>
<div class="card">
  {{if .User.IsSuspended}}
  <form method="post" action="/admin/users/{{.User.ID}}/unsuspend"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><button class="btn" type="submit">Unsuspend user</button></form>
  {{else}}
  <form method="post" action="/admin/users/{{.User.ID}}/suspend"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><button class="btn warn" type="submit">Suspend user</button></form>
  {{end}}
  <form method="post" action="/admin/users/{{.User.ID}}/delete" onsubmit="return confirm('Delete user and stored credentials?');"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><button class="btn warn" type="submit">Delete user</button></form>
</div>
<div class="card">
  <h2>Last 30 days</h2>
  <table>
    <thead><tr><th>Day</th><th>Allowed</th><th>Denied</th></tr></thead>
    <tbody>
      {{range .Stats}}
      <tr><td>{{.Day}}</td><td>{{.AllowedCount}}</td><td>{{.DeniedCount}}</td></tr>
      {{end}}
    </tbody>
  </table>
</div>
</body></html>`))
)
