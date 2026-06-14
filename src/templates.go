package main

import "html/template"

var (
	loginTemplate = template.Must(template.New("login").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>{{.AppName}}</title>
<style>
body{font-family:Arial,sans-serif;max-width:960px;margin:2rem auto;padding:0 1rem;}
.card{border:1px solid #ddd;border-radius:8px;padding:1rem 1.25rem;margin-bottom:1rem;}
.btn{display:inline-flex;align-items:center;justify-content:center;min-height:2.35rem;box-sizing:border-box;background:#0b57d0;color:#fff;padding:.55rem .9rem;border-radius:6px;text-decoration:none;border:none;cursor:pointer;font:inherit;font-size:.875rem;line-height:1.2;}
small,.small{color:#666;}
code{background:#f4f4f4;padding:.15rem .35rem;border-radius:4px;}
</style></head><body>
<h1>{{.AppName}}</h1>
<div class="card">
  <p>Sign in to the proxy with your Google account. Google Workspace access is a separate step after login.</p>
  <a class="btn" href="/auth/google/login">Sign in with Google</a>
</div>
</body></html>`))

	dashboardTemplate = template.Must(template.New("dash").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>{{.AppName}}</title>
<style>
body{font-family:Arial,sans-serif;max-width:1200px;margin:2rem auto;padding:0 1rem;}
.card{border:1px solid #ddd;border-radius:8px;padding:1rem 1.25rem;margin-bottom:1rem;}
.card > :first-child{margin-top:0;}
.card > :last-child{margin-bottom:0;}
.card h2{margin:0 0 1rem 0;}
.btn{display:inline-flex;align-items:center;justify-content:center;min-height:2.35rem;box-sizing:border-box;background:#0b57d0;color:#fff;padding:.55rem .9rem;border-radius:6px;text-decoration:none;border:none;cursor:pointer;font:inherit;font-size:.875rem;line-height:1.2;}
.btn.secondary{background:#444;}
.btn.warn{background:#b73239;}
.btn:disabled{background:#aaa;color:#fff;cursor:not-allowed;opacity:1;}
code{background:#f4f4f4;padding:.15rem .35rem;border-radius:4px;}
small{color:#666;}
.error{color:#b00020;margin:.5rem 0 1rem 0;}
.inline-actions{display:flex;gap:.75rem;align-items:center;flex-wrap:wrap;}
.api-config-sections{display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:1rem;}
.api-config-section{border:1px solid #ddd;border-radius:6px;background:#fbfbfb;padding:1rem;}
.api-config-section h3{margin:0 0 .45rem 0;font-size:1rem;}
.api-config-section p{margin:.35rem 0 .85rem 0;}
.app-shell{display:grid;grid-template-columns:260px minmax(0,1fr);gap:1.25rem;align-items:start;}
.page-crumb{display:flex;gap:.45rem;align-items:center;margin:-.6rem 0 1.2rem 0;color:#666;font-size:.875rem;}
.page-crumb strong{color:#222;}
.page-crumb .sep{color:#999;}
.sidebar{display:flex;flex-direction:column;border:1px solid #ddd;border-radius:8px;padding:.85rem;background:#fbfbfb;}
.sidebar-section{margin-bottom:.95rem;}
.sidebar-label{margin:0 0 .45rem 0;color:#666;font-size:.72rem;font-weight:700;letter-spacing:.05em;text-transform:uppercase;}
.sidebar-nav,.workspace-list{display:flex;flex-direction:column;gap:.2rem;}
.workspace-list{margin:.35rem 0 0 0;}
.workspace-link{display:block;position:relative;min-width:0;padding:.55rem .65rem .55rem .8rem;border-radius:6px;color:#222;text-decoration:none;}
.workspace-link:hover{background:#f1f4f8;}
.workspace-link.active{background:#eaf1ff;color:#0b57d0;}
.workspace-link.active:before{content:"";position:absolute;left:0;top:.45rem;bottom:.45rem;width:3px;border-radius:2px;background:#0b57d0;}
.workspace-link.dragging{opacity:.45;}
.workspace-link[draggable="true"]{cursor:grab;}
.workspace-link[draggable="true"]:active{cursor:grabbing;}
.truncate{display:block;max-width:100%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
.workspace-link strong{font-size:.9rem;}
.workspace-link small{display:block;margin-top:.15rem;color:#666;font-size:.78rem;}
.sidebar-connect{display:inline-flex;margin-top:.55rem;min-height:2rem;padding:.4rem .6rem;font-size:.8rem;}
.logged-as{border-top:1px solid #ddd;margin-top:.95rem;padding-top:.9rem;}
.logged-as h2{margin:0 0 .55rem 0;color:#666;font-size:.72rem;font-weight:700;letter-spacing:.05em;text-transform:uppercase;}
.logged-as .identity{margin-bottom:.75rem;}
.logged-as .identity strong{font-weight:400;}
.logged-as .identity small{color:#666;margin-top:.2rem;}
.logged-actions{display:flex;gap:.5rem;align-items:center;flex-wrap:wrap;}
.logged-as .btn{min-height:2rem;padding:.4rem .7rem;font-size:.8rem;}
.main{min-width:0;}
.settings-form{display:flex;gap:.75rem;align-items:flex-end;flex-wrap:wrap;max-width:640px;}
.settings-field{flex:1 1 260px;}
.settings-field label{display:block;margin-bottom:.35rem;font-weight:700;}
.policy-toolbar{display:flex;gap:.75rem;align-items:flex-end;flex-wrap:wrap;margin-bottom:1rem;}
.policy-field{flex:1 1 260px;}
.policy-field label{display:block;margin-bottom:.35rem;font-weight:700;}
.policy-apps{display:flex;flex-direction:column;gap:.65rem;margin:1rem 0;}
.policy-app{border:1px solid #ddd;border-radius:6px;background:#fff;overflow:hidden;}
.policy-app-toggle{display:flex;align-items:center;justify-content:space-between;gap:.75rem;width:100%;min-height:2.75rem;padding:.7rem .85rem;border:none;background:#f8f9fb;color:#222;cursor:pointer;font:inherit;font-weight:700;text-align:left;}
.policy-app-toggle:hover{background:#f1f4f8;}
.policy-app-arrow{font-size:.9rem;color:#555;}
.policy-app-body{padding:.85rem;}
.policy-subgroup + .policy-subgroup{margin-top:1rem;}
.policy-subgroup h3{margin:0 0 .55rem 0;color:#444;font-size:.95rem;}
.policy-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:.65rem 1rem;margin:0;}
.policy-capability{display:flex;gap:.55rem;align-items:flex-start;border:1px solid #ddd;border-radius:6px;padding:.65rem;background:#fff;}
.policy-capability input{margin-top:.2rem;}
.policy-capability strong{display:block;margin-bottom:.15rem;}
.policy-capability small{display:block;line-height:1.35;}
.policy-capability .risk-score{display:inline-flex;align-items:center;gap:.35rem;margin-bottom:.15rem;color:#666;font-size:.78rem;}
.risk-help{position:relative;display:inline-flex;align-items:center;justify-content:center;width:1rem;height:1rem;border:1px solid #bbb;border-radius:999px;color:#555;background:#fff;font-size:.68rem;font-weight:700;line-height:1;cursor:help;}
.risk-help:focus{outline:2px solid #0b57d0;outline-offset:2px;}
.risk-help:before{content:attr(data-tooltip);position:absolute;left:50%;bottom:calc(100% + .45rem);transform:translateX(-50%);display:none;width:220px;max-width:70vw;padding:.5rem .6rem;border:1px solid #ccc;border-radius:6px;background:#222;color:#fff;box-shadow:0 6px 18px rgba(0,0,0,.18);font-size:.75rem;font-weight:400;line-height:1.35;text-align:left;white-space:normal;z-index:20;}
.risk-help:after{content:"";position:absolute;left:50%;bottom:calc(100% + .18rem);transform:translateX(-50%);display:none;border:.3rem solid transparent;border-top-color:#222;z-index:21;}
.risk-help:hover:before,.risk-help:hover:after,.risk-help:focus:before,.risk-help:focus:after{display:block;}
.risk-swatch{display:inline-block;width:.65rem;height:.65rem;border-radius:2px;flex:0 0 auto;}
.risk-low{background:#188038;}
.risk-medium{background:#f29900;}
.risk-high{background:#b3261e;}
.policy-group{margin:1.25rem 0 .35rem 0;}
.account-actions{display:flex;gap:.75rem;align-items:flex-start;flex-wrap:wrap;}
.account-update-form{display:flex;gap:.75rem;align-items:flex-start;flex:1 1 420px;min-width:280px;}
.account-update-form input[type="text"]{flex:1 1 220px;min-width:180px;}
.account-update-form .btn{flex:0 0 auto;padding-left:.75rem;padding-right:.75rem;}
.account-actions > form:not(.account-update-form){flex:0 0 auto;}
table{border-collapse:collapse;width:100%;table-layout:fixed;}
th,td{border:1px solid #ddd;padding:.65rem;text-align:left;vertical-align:top;word-wrap:break-word;overflow-wrap:anywhere;}
.folder-form{display:grid;grid-template-columns:minmax(180px,220px) minmax(260px,1fr) auto;gap:.75rem;align-items:start;}
.folder-form .checks{grid-column:1 / span 2;display:flex;gap:.75rem;flex-wrap:wrap;align-items:center;}
.folder-edit-grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(0,1.7fr);gap:.5rem 1rem;align-items:center;max-width:100%;}
.folder-edit-grid .checks{grid-column:1 / span 2;display:flex;gap:.75rem;flex-wrap:wrap;align-items:center;}
.folder-action-row{display:flex;gap:.6rem;flex-wrap:wrap;align-items:center;margin-top:.5rem;}
.allowed-list{margin:.1rem 0 0 1.1rem;padding:0;}
.allowed-list li{margin:.15rem 0;}
.discovery-overlay{position:fixed;inset:0;background:rgba(255,255,255,.86);display:none;align-items:center;justify-content:center;z-index:1000;padding:1rem;}
.discovery-overlay.active{display:flex;}
.discovery-box{max-width:420px;background:#fff;border:1px solid #ddd;border-radius:8px;padding:1.25rem 1.5rem;box-shadow:0 8px 28px rgba(0,0,0,.14);}
.discovery-box h2{margin:.1rem 0 .5rem 0;}
.discovery-box p{margin:.4rem 0;color:#444;}
.setup-box{max-width:720px;}
.setup-box pre{white-space:pre-wrap;overflow:auto;background:#f6f8fa;border:1px solid #ddd;border-radius:6px;padding:.85rem;font-size:.82rem;line-height:1.45;}
.setup-box pre code{display:block;margin:0;text-indent:0;}
.overlay-title-row{display:flex;align-items:center;justify-content:space-between;gap:1rem;margin:.1rem 0 .5rem 0;}
.overlay-title-row h2{margin:0;}
.overlay-close{display:inline-flex;align-items:center;justify-content:center;flex:0 0 auto;width:2rem;height:2rem;border:none;border-radius:6px;background:transparent;color:#444;cursor:pointer;font:inherit;font-size:1.25rem;line-height:1;}
.overlay-close:hover{background:#f1f4f8;color:#111;}
.setup-heading{display:flex;align-items:center;justify-content:space-between;gap:.75rem;margin-top:1rem;}
.setup-heading h3{margin:0;}
.copy-btn{min-height:1.9rem;padding:.35rem .6rem;font-size:.78rem;}
.setup-actions{display:flex;justify-content:flex-end;margin-top:1rem;}
.spinner{width:28px;height:28px;border:3px solid #d7d7d7;border-top-color:#0b57d0;border-radius:50%;animation:spin .9s linear infinite;margin-bottom:.75rem;}
@keyframes spin{to{transform:rotate(360deg);}}
input[type="text"],select{box-sizing:border-box;max-width:100%;width:100%;min-height:2.35rem;padding:.55rem .6rem;font:inherit;font-size:.875rem;line-height:1.2;}
label.checkbox{display:inline-flex;gap:.35rem;align-items:center;white-space:nowrap;}
@media (max-width: 900px){
  .app-shell{grid-template-columns:1fr;}
  .policy-grid{grid-template-columns:repeat(2,minmax(0,1fr));}
  .folder-form{grid-template-columns:1fr;}
  .folder-form .checks{grid-column:auto;}
  .account-update-form{flex-direction:column;align-items:stretch;}
  .folder-edit-grid{grid-template-columns:1fr;}
  .folder-edit-grid .checks{grid-column:auto;}
  table,thead,tbody,tr,td,th{display:block;}
  thead{display:none;}
  tr{border:1px solid #ddd;margin-bottom:1rem;padding:.5rem;}
  td{border:none;padding:.35rem 0;}
}
@media (max-width: 640px){
  .policy-grid{grid-template-columns:1fr;}
}
</style>
<script>
function confirmAgentWorkspaceAPIKeyRotate(){
  return confirm('Rotate the AI agents Workspace API key? Existing agent configs and installed skills using the old key will stop working immediately.');
}
function confirmUserBackendAPIKeyRotate(){
  return confirm('Rotate the end-user backend API key? Existing user backend configs using the old key will stop working immediately.');
}
function ensureDriveTypeSelection(prefix){
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
function showAgentSkillSetupOverlay(){
  const overlay = document.getElementById('agentSkillSetupOverlay');
  if (overlay) {
    overlay.classList.add('active');
  }
}
function closeAgentSkillSetupOverlay(){
  const overlay = document.getElementById('agentSkillSetupOverlay');
  if (overlay) {
    overlay.classList.remove('active');
  }
}
function copyAgentSkillCommand(id, button){
  const el = document.getElementById(id);
  if (!el || !navigator.clipboard) {
    return;
  }
  navigator.clipboard.writeText(el.textContent.trim()).then(() => {
    const original = button.textContent;
    button.textContent = 'Copied';
    setTimeout(() => { button.textContent = original; }, 1200);
  }).catch(() => {});
}
function syncAgentSkillDownloadButton(){
  const select = document.getElementById('agentSkillPlatform');
  const button = document.getElementById('agentSkillDownloadButton');
  if (select && button) {
    button.disabled = !select.value;
  }
}
function startAgentSkillDownload(){
  const select = document.getElementById('agentSkillPlatform');
  const platform = select ? select.value : '';
  if (!platform) {
    syncAgentSkillDownloadButton();
    return false;
  }
  showAgentSkillSetupOverlay();
  const installCommand = document.getElementById('agentSkillInstallCommand');
  const expires = document.getElementById('agentSkillInstallExpires');
  const error = document.getElementById('agentSkillInstallError');
  const installNote = document.getElementById('agentSkillInstallNote');
  const postInstallNote = document.getElementById('agentSkillPostInstallNote');
  if (installCommand) installCommand.textContent = 'Generating secure install command...';
  if (expires) expires.textContent = '';
  if (error) error.textContent = '';
  if (installNote) installNote.textContent = '';
  if (postInstallNote) postInstallNote.textContent = '';
  const body = new URLSearchParams();
  body.append('csrf_token', '{{.CSRFToken}}');
  body.append('platform', platform);
  fetch('/api/agent-skill/install-token', {
    method: 'POST',
    credentials: 'same-origin',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body
  })
    .then(async response => {
      const data = await response.json();
      if (!response.ok) {
        throw new Error(data.message || 'Unable to generate install command.');
      }
      if (installCommand) installCommand.textContent = (data.install_command || '').trim();
      if (expires) expires.textContent = 'The download token expires at ' + (data.expires_at_display || data.expires_at || '') + ' and is consumed after one successful download.';
      if (installNote) installNote.textContent = data.install_note || '';
      if (postInstallNote) postInstallNote.textContent = data.post_install_note || '';
    })
    .catch(err => {
      if (error) error.textContent = err.message || String(err);
      if (installCommand) installCommand.textContent = '';
    });
  return false;
}
function prepareDriveFolderSubmit(prefix){
  if (!ensureDriveTypeSelection(prefix)) {
    return false;
  }
  return showDriveDiscoveryOverlay();
}
function persistWorkspaceOrder(){
  const list = document.getElementById('workspaceList');
  if (!list) return;
  const body = new URLSearchParams();
  body.append('csrf_token', '{{.CSRFToken}}');
  list.querySelectorAll('[data-workspace-email]').forEach(item => {
    body.append('workspace', item.dataset.workspaceEmail);
  });
  fetch('/workspace/order', {
    method: 'POST',
    credentials: 'same-origin',
    headers: {'Content-Type': 'application/x-www-form-urlencoded'},
    body
  }).catch(() => {});
}
function initWorkspaceDrag(){
  const list = document.getElementById('workspaceList');
  if (!list) return;
  let dragged = null;
  let suppressClick = false;
  list.addEventListener('click', event => {
    if (!suppressClick) return;
    event.preventDefault();
    suppressClick = false;
  });
  list.addEventListener('dragstart', event => {
    dragged = event.target.closest('[data-workspace-email]');
    if (!dragged) return;
    dragged.classList.add('dragging');
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move';
    }
  });
  list.addEventListener('dragover', event => {
    event.preventDefault();
    if (!dragged) return;
    const siblings = [...list.querySelectorAll('[data-workspace-email]:not(.dragging)')];
    const next = siblings.find(item => event.clientY < item.getBoundingClientRect().top + item.offsetHeight / 2);
    list.insertBefore(dragged, next || null);
  });
  list.addEventListener('dragend', () => {
    if (!dragged) return;
    dragged.classList.remove('dragging');
    dragged = null;
    suppressClick = true;
    setTimeout(() => { suppressClick = false; }, 150);
    persistWorkspaceOrder();
  });
}
function trackedFieldValue(el){
  const tag = el.tagName.toLowerCase();
  const type = (el.type || '').toLowerCase();
  if (!el.name || type === 'hidden' || type === 'submit' || type === 'button' || type === 'reset') {
    return null;
  }
  if (type === 'checkbox' || type === 'radio') {
    return el.name + '=' + el.value + ':' + el.checked;
  }
  if (tag === 'select' && el.multiple) {
    return el.name + '=' + [...el.selectedOptions].map(option => option.value).join(',');
  }
  return el.name + '=' + el.value;
}
function trackedFormSnapshot(form){
  return [...form.elements]
    .map(trackedFieldValue)
    .filter(value => value !== null)
    .join('\n');
}
function isSubmitButton(el){
  return (el.getAttribute('type') || 'submit').toLowerCase() === 'submit';
}
function trackedFormButtons(form){
  const buttons = [...form.querySelectorAll('button,input[type="submit"]')].filter(isSubmitButton);
  if (form.id) {
    document.querySelectorAll('button[form],input[form]').forEach(button => {
      if (button.getAttribute('form') === form.id && isSubmitButton(button)) {
        buttons.push(button);
      }
    });
  }
  return [...new Set(buttons)];
}
function initTrackedForms(){
  document.querySelectorAll('form[data-track-changes="true"]').forEach(form => {
    const initialSnapshot = trackedFormSnapshot(form);
    const buttons = trackedFormButtons(form);
    const syncButtons = () => {
      const changed = trackedFormSnapshot(form) !== initialSnapshot;
      buttons.forEach(button => { button.disabled = !changed; });
    };
    form.addEventListener('input', syncButtons);
    form.addEventListener('change', syncButtons);
    form.addEventListener('reset', () => setTimeout(syncButtons, 0));
    form.addEventListener('submit', () => {
      buttons.forEach(button => { button.disabled = true; });
    });
    syncButtons();
  });
}
function requiredValuePresent(el){
  const type = (el.type || '').toLowerCase();
  if (type === 'checkbox' || type === 'radio') {
    return el.checked;
  }
  return String(el.value || '').trim() !== '';
}
function initRequiredForms(){
  document.querySelectorAll('form[data-require-complete="true"]').forEach(form => {
    const buttons = trackedFormButtons(form);
    const requiredFields = [...form.querySelectorAll('[required]')];
    const syncButtons = () => {
      const complete = requiredFields.every(requiredValuePresent);
      buttons.forEach(button => { button.disabled = !complete; });
    };
    form.addEventListener('input', syncButtons);
    form.addEventListener('change', syncButtons);
    form.addEventListener('reset', () => setTimeout(syncButtons, 0));
    form.addEventListener('submit', () => {
      buttons.forEach(button => { button.disabled = true; });
    });
    syncButtons();
  });
}
function selectPolicyForEditing(select){
  window.location = '/policies?policy=' + encodeURIComponent(select.value);
}
function applySystemDefaultPolicy(){
  document.querySelectorAll('[data-policy-capability="true"]').forEach(input => {
    input.checked = input.dataset.systemDefault === 'true';
    input.dispatchEvent(new Event('change', {bubbles: true}));
  });
}
function togglePolicyApp(button){
  const body = document.getElementById(button.getAttribute('aria-controls'));
  const arrow = button.querySelector('.policy-app-arrow');
  if (!body) return;
  const expanded = button.getAttribute('aria-expanded') === 'true';
  button.setAttribute('aria-expanded', expanded ? 'false' : 'true');
  body.hidden = expanded;
  if (arrow) arrow.innerHTML = expanded ? '&#9662;' : '&#9652;';
}
document.addEventListener('DOMContentLoaded', () => {
  initWorkspaceDrag();
  initTrackedForms();
  initRequiredForms();
  syncAgentSkillDownloadButton();
});
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
<div id="agentSkillSetupOverlay" class="discovery-overlay" role="dialog" aria-modal="true" aria-labelledby="agentSkillSetupTitle">
  <div class="discovery-box setup-box">
    <div class="overlay-title-row">
      <h2 id="agentSkillSetupTitle">Install the AI agent skill</h2>
      <button class="overlay-close" type="button" aria-label="Close overlay" onclick="closeAgentSkillSetupOverlay()">&times;</button>
    </div>
    <p>The command below downloads the skill to <code>/tmp</code>, installs it locally, runs the bootstrap, and deletes the temporary zip.</p>
    <p id="agentSkillInstallExpires"><small></small></p>
    <p id="agentSkillInstallError" class="error"></p>
    <div class="setup-heading">
      <h3>With curl or wget</h3>
      <button class="btn copy-btn" type="button" onclick="copyAgentSkillCommand('agentSkillInstallCommand', this)">COPY</button>
    </div>
    <pre><code id="agentSkillInstallCommand">Generating secure install command...</code></pre>
    <p><small>The download token is single-use. If the command fails before the download starts, click the button again to generate a fresh command.</small></p>
    <p><small id="agentSkillInstallNote"></small></p>
    <p><small id="agentSkillPostInstallNote"></small></p>
    <div class="setup-actions">
      <button class="btn" type="button" onclick="closeAgentSkillSetupOverlay()">Understood</button>
    </div>
  </div>
</div>
<h1>{{.AppName}}</h1>
<div class="page-crumb">
  {{if .ShowPolicies}}
    <strong>Policy Editor</strong>
  {{else if .ShowSettings}}
    <strong>Settings</strong>
  {{else if .ShowDashboard}}
    <strong>Dashboard</strong>
  {{else}}
    <span>Workspace</span><span class="sep">&gt;</span><strong class="truncate">{{.Workspace.Email}}</strong>
  {{end}}
</div>

<div class="app-shell">
<aside class="sidebar">
  <div class="sidebar-section">
    <p class="sidebar-label">Navigation</p>
    <div class="sidebar-nav">
      <a class="workspace-link {{if .ShowDashboard}}active{{end}}" href="/">
        <strong class="truncate">Dashboard</strong>
        <small class="truncate">Proxy token and config</small>
      </a>
    </div>
  </div>
  <div class="sidebar-section">
    <p class="sidebar-label">Workspaces</p>
    <a class="btn sidebar-connect" href="/auth/workspace/connect">Add a workspace</a>
    {{if .Workspaces}}
    <div id="workspaceList" class="workspace-list">
      {{range .Workspaces}}
        <a class="workspace-link {{if .Active}}active{{end}}" href="/?workspace={{.Email}}" draggable="true" data-workspace-email="{{.Email}}">
          <strong class="truncate">{{.Name}}</strong>
          <small class="truncate">{{.Email}}</small>
        </a>
      {{end}}
    </div>
    {{else}}
      <p><small>No Workspace accounts connected yet.</small></p>
    {{end}}
  </div>
  <div class="sidebar-section">
    <p class="sidebar-label">Policies</p>
    <div class="sidebar-nav">
      <a class="workspace-link {{if .ShowPolicies}}active{{end}}" href="/policies">
        <strong class="truncate">Policy Editor</strong>
        <small class="truncate">Google operation permissions</small>
      </a>
    </div>
  </div>
  <div class="logged-as">
    <h2>Logged as</h2>
    <div class="identity">
      <strong class="truncate">{{.User.Email}}</strong>
      <small class="truncate">{{.User.Name}}</small>
    </div>
    <div class="logged-actions">
      <a class="btn" href="/settings">Settings</a>
      <form method="post" action="/logout"><input type="hidden" name="csrf_token" value="{{.CSRFToken}}"><button class="btn secondary" type="submit">Logout</button></form>
    </div>
  </div>
</aside>

<main class="main">

{{if .ShowPolicies}}
<div class="card">
  <h2>Policy Editor</h2>
  {{if .PolicyEditor.Error}}<p class="error">{{.PolicyEditor.Error}}</p>{{end}}
  {{if .PolicyEditor.Saved}}<p><small>Policy saved.</small></p>{{end}}
  {{if .PolicyEditor.DefaultSaved}}<p><small>Default policy selection saved.</small></p>{{end}}
  {{if eq .PolicyEditor.Deleted "default"}}<p><small>The applied custom policy was deleted. Affected selections have reverted to the system default policy.</small></p>{{end}}
  {{if eq .PolicyEditor.Deleted "custom"}}<p><small>Custom policy deleted.</small></p>{{end}}

  <div class="policy-toolbar">
    <div class="policy-field">
      <label for="policy_select">Policy to edit</label>
      <select id="policy_select" onchange="selectPolicyForEditing(this)">
        {{range .PolicyEditor.Options}}<option value="{{.ID}}" {{if .Selected}}selected{{end}}>{{.Name}}{{if .IsDefault}} (default){{end}}</option>{{end}}
      </select>
    </div>
    <a class="btn" href="/policies?policy=new">New custom policy</a>
  </div>

  <form method="post" action="/policies/default" class="policy-toolbar" data-track-changes="true">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <div class="policy-field">
      <label for="default_policy_id">Default policy to apply</label>
      <select id="default_policy_id" name="default_policy_id">
        {{range .PolicyEditor.Options}}<option value="{{.ID}}" {{if .IsDefault}}selected{{end}}>{{.Name}}</option>{{end}}
      </select>
    </div>
    <button class="btn" type="submit">Apply default policy</button>
  </form>
</div>

<div class="card">
  <h2>{{if .PolicyEditor.SelectedIsNew}}New custom policy{{else}}{{.PolicyEditor.SelectedName}}{{end}}</h2>
  {{if .PolicyEditor.SelectedIsSystem}}
    <p><small>The system default policy allows all Low-risk capabilities and cannot be edited or deleted. Create a custom policy to change selections.</small></p>
  {{else}}
    <p><small>Select the friendly capabilities this policy should allow. The proxy translates these choices into Google API allow rules.</small></p>
  {{end}}
  <form method="post" action="/policies/save" {{if .PolicyEditor.SelectedIsNew}}data-require-complete="true"{{else}}data-track-changes="true"{{end}}>
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <input type="hidden" name="policy_id" value="{{if .PolicyEditor.SelectedIsNew}}{{else}}{{.PolicyEditor.SelectedID}}{{end}}">
    <div class="policy-toolbar">
      <div class="policy-field">
        <label for="policy_name">Policy name</label>
        <input id="policy_name" type="text" name="name" value="{{.PolicyEditor.SelectedName}}" {{if .PolicyEditor.SelectedIsSystem}}disabled{{else}}required{{end}}>
      </div>
      {{if not .PolicyEditor.SelectedIsSystem}}
        <button class="btn" type="button" onclick="applySystemDefaultPolicy()">Revert selections to system default</button>
        <button class="btn" type="submit">Save policy</button>
      {{end}}
    </div>
    <div class="policy-apps">
      {{range .PolicyEditor.CapabilityGroups}}
      <section class="policy-app">
        <button class="policy-app-toggle" type="button" aria-expanded="false" aria-controls="policy_app_{{.Name}}" onclick="togglePolicyApp(this)">
          <span>{{.Name}}</span>
          <span class="policy-app-arrow" aria-hidden="true">&#9662;</span>
        </button>
        <div id="policy_app_{{.Name}}" class="policy-app-body" hidden>
          {{$showSubgroups := .ShowSubgroups}}
          {{range .Subgroups}}
            <div class="policy-subgroup">
              {{if $showSubgroups}}<h3>{{.Name}}</h3>{{end}}
              <div class="policy-grid">
                {{range .Capabilities}}
                <label class="policy-capability">
                  <input data-policy-capability="true" data-system-default="{{.SystemDefault}}" type="checkbox" name="capability" value="{{.Key}}" {{if .Checked}}checked{{end}} {{if $.PolicyEditor.SelectedIsSystem}}disabled{{end}}>
                  <span>
                    <small class="risk-score"><span>Risk:</span><span class="risk-swatch {{.RiskClass}}" aria-hidden="true"></span><span>{{.RiskLabel}}</span><span class="risk-help" tabindex="0" role="img" aria-label="{{.RiskDescription}}" data-tooltip="{{.RiskDescription}}">(?)</span></small>
                    <strong>{{.Title}}</strong>
                    <small>{{.Summary}}{{if .RequiresDriveRefs}} Uses configured allowed folders.{{end}}</small>
                  </span>
                </label>
                {{end}}
              </div>
            </div>
            {{end}}
        </div>
      </section>
      {{end}}
    </div>
  </form>
  {{if and (not .PolicyEditor.SelectedIsSystem) (not .PolicyEditor.SelectedIsNew)}}
    <form method="post" action="/policies/delete" onsubmit="return confirm('{{if .PolicyEditor.SelectedIsApplied}}This policy is currently applied. Deleting it will revert affected selections to the system default policy. Continue?{{else}}Delete this custom policy?{{end}}');">
      <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
      <input type="hidden" name="policy_id" value="{{.PolicyEditor.SelectedID}}">
      <button class="btn warn" type="submit">Delete policy</button>
    </form>
  {{end}}
</div>
{{else if .ShowSettings}}
<div class="card">
  <h2>Settings</h2>
  {{if .SettingsError}}<p class="error">{{.SettingsError}}</p>{{end}}
  {{if .SettingsSaved}}<p><small>Settings saved.</small></p>{{end}}
  <form method="post" action="/settings/update" class="settings-form" data-track-changes="true">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <div class="settings-field">
      <label for="timezone">Timezone</label>
      <select id="timezone" name="timezone" required>
        {{range .Settings.Timezones}}<option value="{{.Name}}" {{if .Selected}}selected{{end}}>{{.Name}}</option>{{end}}
      </select>
    </div>
    <button class="btn" type="submit">Save settings</button>
  </form>
  <p><small>Select the local timezone used to display timestamps in the User Interface.</small></p>
  <p><small>Currently saved timezone: {{.Settings.Timezone}}</small></p>
  <p><small>Current time in currently saved timezone: {{.Settings.CurrentTime}}</small></p>
</div>
{{else if .ShowDashboard}}
<div class="card">
  <h2>Download skill for AI agent</h2>
  <p>The skill package is the recommended way to grant AI agents access to Google Workspace applications through the proxy. It contains the proxy helper script, policy-filtered Workspace instructions, and a configuration file with the API key required to use the proxy.</p>
  <div class="policy-toolbar">
    <div class="policy-field">
      <label for="agentSkillPlatform">Skill package</label>
      <select id="agentSkillPlatform" onchange="syncAgentSkillDownloadButton()">
        <option value="">Please select the skill to download</option>
        <option value="generic">Generic</option>
        <option value="openclaw">OpenClaw</option>
      </select>
    </div>
    <button id="agentSkillDownloadButton" class="btn" type="button" onclick="return startAgentSkillDownload()" disabled>Download</button>
  </div>
</div>
<div class="card">
  <h2>Proxy API Configurations</h2>
  <p>Download standalone configuration files when you need to set up tools manually instead of using the skill package. Rotating an API key is useful if you fear that one of your API keys may have leaked. Rotation immediately invalidates the current key, so tools that rely on that key will stop working.</p>
  <div class="api-config-sections">
    <section class="api-config-section">
      <h3>Configuration file for AI agents</h3>
      <p><small>Download this file for AI agents and generated skills. It contains the standard Workspace API key used to access Gmail, Calendar, Drive, Docs, Sheets, and Slides through the proxy policy controls.</small></p>
      <div class="inline-actions">
        <a class="btn" href="/api/config/agents-workspace-api-access/download">Download config</a>
        <form method="post" action="/api/config/agents-workspace-api-access/rotate" onsubmit="return confirmAgentWorkspaceAPIKeyRotate();">
          <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
          <button class="btn warn" type="submit">Rotate API key</button>
        </form>
      </div>
    </section>
    <section class="api-config-section">
      <h3>Configuration file for end-users</h3>
      <p><small>Download this file for user-owned backend tools that manage your account settings, proxy policies, access to logs, etc. Do NOT give this privileged configuration file to AI agents.</small></p>
      <div class="inline-actions">
        <a class="btn" href="/api/config/user-backend-api-access/download">Download config</a>
        <form method="post" action="/api/config/user-backend-api-access/rotate" onsubmit="return confirmUserBackendAPIKeyRotate();">
          <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
          <button class="btn warn" type="submit">Rotate API key</button>
        </form>
      </div>
    </section>
  </div>
</div>
{{else}}

<div class="card">
  <h2>Google Workspace account</h2>
  {{if .WorkspaceError}}<p class="error">{{.WorkspaceError}}</p>{{end}}
  {{if .WorkspaceConnected}}
    <p><strong>Account:</strong> {{.Workspace.Email}}</p>
    <p><strong>Friendly name:</strong> {{.Workspace.Name}}</p>
    <p><strong>Allowed Scopes:</strong></p>
    <ul class="allowed-list">
      {{range .Workspace.Scopes}}<li>{{.}}</li>{{end}}
    </ul>
    <p><strong>Connected since:</strong> {{.Workspace.ConnectedSince}}</p>
    <p><strong>Connection status:</strong> {{.Workspace.ConnectionStatus}}</p>
    <p><strong>Agent Workspace API key:</strong> {{.Workspace.ProxyTokenStatus}}</p>
    <p><small>Workspace access is refreshed automatically and remains active until access is revoked or disconnected.</small></p>
    <div class="account-actions">
      <form method="post" action="/workspace/account/update" class="account-update-form" data-track-changes="true">
        <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
        <input type="hidden" name="workspace" value="{{.Workspace.Selector}}">
        <input type="text" name="friendly_name" value="{{.Workspace.Name}}" required>
        <button class="btn" type="submit">Update friendly name</button>
      </form>
      <form method="post" action="/auth/workspace/disconnect" onsubmit="return confirm('Disconnect this Workspace account and delete its Drive folder settings?');">
        <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
        <input type="hidden" name="workspace" value="{{.Workspace.Selector}}">
        <button class="btn warn" type="submit">Disconnect Workspace</button>
      </form>
    </div>
  {{else}}
    <p>No Workspace account selected. Connect one from the left menu.</p>
  {{end}}
</div>

{{if .WorkspaceConnected}}
<div class="card">
  <h2>Workspace policy</h2>
  {{if .WorkspacePolicyError}}<p class="error">{{.WorkspacePolicyError}}</p>{{end}}
  {{if .WorkspacePolicySaved}}<p><small>Workspace policy saved.</small></p>{{end}}
  <p><small>Select which policy the proxy applies when AI agents make Google API requests for this Workspace account.</small></p>
  <form method="post" action="/workspace/policy/update" class="policy-toolbar" data-track-changes="true">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <input type="hidden" name="workspace" value="{{.Workspace.Selector}}">
    <div class="policy-field">
      <label for="workspace_policy_id">Applied policy</label>
      <select id="workspace_policy_id" name="policy_id">
        {{range .Workspace.PolicyOptions}}<option value="{{.ID}}" {{if .Selected}}selected{{end}}>{{.Name}}</option>{{end}}
      </select>
    </div>
    <button class="btn" type="submit">Apply policy</button>
  </form>
</div>

<div class="card">
  <h2>Allowed Drive folders</h2>
  <p><small>Use a unique Reference Name. Agents can refer to folders by that name. Registered folders include cached subfolders.</small></p>
  {{if .FolderError}}<p class="error">{{.FolderError}}</p>{{end}}
  <form method="post" action="/workspace/drive-folders/add" class="folder-form" data-require-complete="true" onsubmit="return prepareDriveFolderSubmit('new');">
    <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
    <input type="hidden" name="workspace" value="{{.Workspace.Selector}}">
    <input type="text" name="reference_name" placeholder="Reference Name" required>
    <input type="text" name="folder_link" placeholder="Google Drive folder link" required>
    <button class="btn" type="submit">Add allowed folder</button>
    <div class="checks">
      <label class="checkbox"><input id="new_docs" type="checkbox" name="allow_docs" checked> Docs</label>
      <label class="checkbox"><input id="new_sheets" type="checkbox" name="allow_sheets" checked> Sheets</label>
      <label class="checkbox"><input id="new_slides" type="checkbox" name="allow_slides" checked> Slides</label>
      <label class="checkbox"><input id="new_drive" type="checkbox" name="allow_drive_files" checked> Non-Google files</label>
    </div>
  </form>

  {{if .DriveFolders}}
  <table style="margin-top:1rem;">
    <thead>
      <tr>
        <th style="width:18%;">Reference Name</th>
        <th style="width:18%;">Real Name</th>
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
        </td>
        <td>
          <ul class="allowed-list">
            {{if .AllowDocs}}<li>Docs</li>{{end}}
            {{if .AllowSheets}}<li>Sheets</li>{{end}}
            {{if .AllowSlides}}<li>Slides</li>{{end}}
            {{if .AllowDriveFiles}}<li>Non-Google files</li>{{end}}
          </ul>
        </td>
        <td>
          <form id="update_{{.ID}}" method="post" action="/workspace/drive-folders/{{.ID}}/update" class="folder-edit-grid" data-track-changes="true" onsubmit="return prepareDriveFolderSubmit('f_{{.ID}}');">
            <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
            <input type="hidden" name="workspace" value="{{$.Workspace.Selector}}">
            <input type="text" name="reference_name" value="{{.ReferenceName}}" required>
            <input type="text" name="folder_link" value="{{.FolderURL}}" required>
            <div class="checks">
              <label class="checkbox"><input id="f_{{.ID}}_docs" type="checkbox" name="allow_docs" {{if .AllowDocs}}checked{{end}}> Docs</label>
              <label class="checkbox"><input id="f_{{.ID}}_sheets" type="checkbox" name="allow_sheets" {{if .AllowSheets}}checked{{end}}> Sheets</label>
              <label class="checkbox"><input id="f_{{.ID}}_slides" type="checkbox" name="allow_slides" {{if .AllowSlides}}checked{{end}}> Slides</label>
              <label class="checkbox"><input id="f_{{.ID}}_drive" type="checkbox" name="allow_drive_files" {{if .AllowDriveFiles}}checked{{end}}> Non-Google files</label>
            </div>
          </form>
          <div class="folder-action-row">
            <button class="btn secondary" type="submit" form="update_{{.ID}}">Update</button>
            <form method="post" action="/workspace/drive-folders/{{.ID}}/refresh" onsubmit="return showDriveDiscoveryOverlay();">
              <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
              <input type="hidden" name="workspace" value="{{$.Workspace.Selector}}">
              <button class="btn secondary" type="submit">Refresh tree</button>
            </form>
            <form method="post" action="/workspace/drive-folders/{{.ID}}/delete" onsubmit="return confirm('Delete this allowed folder reference?');">
              <input type="hidden" name="csrf_token" value="{{$.CSRFToken}}">
              <input type="hidden" name="workspace" value="{{$.Workspace.Selector}}">
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
{{end}}
{{end}}

{{if .User.IsAdmin}}
<div class="card">
  <h2>Admin</h2>
  <p><a class="btn secondary" href="/admin/users">Open admin section</a></p>
</div>
{{end}}
</main>
</div>
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
  <p><strong>Created:</strong> {{.UserCreatedAtLocal}}</p>
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
