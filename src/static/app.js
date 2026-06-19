function confirmAgentWorkspaceAPIKeyRotate(){
  return confirm('Rotate the AI agents Workspace API key? Existing agent configs and installed skills using the old key will stop working immediately.');
}
function confirmUserBackendAPIKeyRotate(){
  return confirm('Rotate the end-user backend API key? Existing user backend configs using the old key will stop working immediately.');
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
function showAgentCreateOverlay(){
  const overlay = document.getElementById('agentCreateOverlay');
  if (overlay) {
    overlay.classList.add('active');
    const input = document.getElementById('agent_create_friendly_name');
    if (input) input.focus();
  }
  return false;
}
function closeAgentCreateOverlay(){
  const overlay = document.getElementById('agentCreateOverlay');
  if (overlay) {
    overlay.classList.remove('active');
  }
}
function showAgentEditOverlay(agentID, field){
  const overlay = document.getElementById('agentEditOverlay');
  const form = document.getElementById('agentEditForm');
  const title = document.getElementById('agentEditTitle');
  const label = document.getElementById('agent_edit_value_label');
  const input = document.getElementById('agent_edit_value');
  if (!overlay || !form || !title || !label || !input) return false;
  const editingName = field === 'friendly_name';
  const current = editingName ? document.getElementById('agentName_' + agentID)?.textContent : document.getElementById('agentLocation_' + agentID)?.textContent;
  form.elements.agent_id.value = agentID;
  form.elements.field.value = field;
  form.dataset.flashTarget = 'agentActionsFlash_' + agentID;
  title.textContent = editingName ? 'Rename agent' : 'Edit location';
  label.textContent = editingName ? 'Agent name' : 'Agent location';
  input.maxLength = editingName ? 256 : 512;
  input.value = (current || '').trim();
  overlay.classList.add('active');
  input.focus();
  input.select();
  return false;
}
function closeAgentEditOverlay(){
  const overlay = document.getElementById('agentEditOverlay');
  if (overlay) {
    overlay.classList.remove('active');
  }
}
function showAdminDeleteUserOverlay(){
  const overlay = document.getElementById('adminDeleteUserOverlay');
  if (overlay) {
    overlay.classList.add('active');
  }
  document.body.classList.add('modal-open');
  return false;
}
function closeAdminDeleteUserOverlay(){
  const overlay = document.getElementById('adminDeleteUserOverlay');
  if (overlay) {
    overlay.classList.remove('active');
  }
  document.body.classList.remove('modal-open');
  return false;
}
function driveFolderGrantOverlay(agentID, workspaceKey){
  return document.getElementById('driveFolderGrantOverlay_' + agentID + '_' + workspaceKey);
}
function driveFolderGrantCheckboxes(overlay){
  return overlay ? [...overlay.querySelectorAll('[data-folder-grant-checkbox]')] : [];
}
function driveFolderGrantChanged(overlay){
  return driveFolderGrantCheckboxes(overlay).some(box => box.checked !== (box.dataset.saved === 'true'));
}
function syncDriveFolderGrantOverlay(overlay){
  if (!overlay) return;
  const apply = overlay.querySelector('[data-folder-grant-apply]');
  if (apply) apply.disabled = !driveFolderGrantChanged(overlay);
}
function showDriveFolderGrantOverlay(agentID, workspaceKey){
  const overlay = driveFolderGrantOverlay(agentID, workspaceKey);
  if (!overlay) return false;
  overlay.classList.add('active');
  syncDriveFolderGrantOverlay(overlay);
  return false;
}
function resetDriveFolderGrantOverlay(overlay){
  driveFolderGrantCheckboxes(overlay).forEach(box => {
    box.checked = box.dataset.saved === 'true';
  });
  syncDriveFolderGrantOverlay(overlay);
}
function closeDriveFolderGrantOverlay(agentID, workspaceKey){
  const overlay = driveFolderGrantOverlay(agentID, workspaceKey);
  if (!overlay) return false;
  if (driveFolderGrantChanged(overlay) && !confirm('Changes were made and will not be saved. Close without saving?')) {
    return false;
  }
  resetDriveFolderGrantOverlay(overlay);
  overlay.classList.remove('active');
  return false;
}
function formatDriveFolderGrantCount(count){
  count = Number(count || 0);
  return count + (count === 1 ? ' folder allowed' : ' folders allowed');
}
function formatMatchingLogEntries(count){
  count = Number(count || 0);
  return count + (count === 1 ? ' matching log entry' : ' matching log entries');
}
async function applyDriveFolderGrantOverlay(agentID, workspaceKey){
  const overlay = driveFolderGrantOverlay(agentID, workspaceKey);
  if (!overlay) return false;
  const apply = overlay.querySelector('[data-folder-grant-apply]');
  if (apply) apply.disabled = true;
  const body = new URLSearchParams();
  body.append('csrf_token', csrfTokenValue());
  body.append('agent_id', agentID);
  body.append('workspace', overlay.dataset.workspace || '');
  driveFolderGrantCheckboxes(overlay).forEach(box => {
    if (box.checked) body.append('folder_ref_id', box.value);
  });
  try {
    const response = await fetch('/agents/drive-folders/grants', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {'Accept': 'application/json', 'X-Requested-With': 'fetch', 'Content-Type': 'application/x-www-form-urlencoded'},
      body,
    });
    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.message || data.error || 'Request failed.');
    }
    driveFolderGrantCheckboxes(overlay).forEach(box => {
      box.dataset.saved = box.checked ? 'true' : 'false';
    });
    const countTarget = document.getElementById(overlay.dataset.countTarget || '');
    if (countTarget) countTarget.textContent = formatDriveFolderGrantCount(data.allowed_count);
    setAgentSkillWarning(data);
    syncDriveFolderGrantOverlay(overlay);
    const flashTarget = overlay.querySelector('.form-flash-slot');
    if (flashTarget) flashTarget.innerHTML = '';
    overlay.classList.remove('active');
  } catch (err) {
    syncDriveFolderGrantOverlay(overlay);
    const flashTarget = overlay.querySelector('.form-flash-slot');
    if (flashTarget) {
      flashTarget.innerHTML = '';
      flashTarget.appendChild(createFlash('error', err.message || String(err)));
    }
  }
  return false;
}
function showAgentFirewallAddressOverlay(agentID){
  const overlay = document.getElementById('agentFirewallAddressOverlay_' + agentID);
  const input = document.getElementById('agentFirewallAddressInput_' + agentID);
  if (overlay) overlay.classList.add('active');
  if (input) {
    input.value = '';
    setTimeout(() => input.focus(), 0);
  }
  const slot = document.getElementById('agentFirewallAddressFlash_' + agentID);
  if (slot) slot.replaceChildren();
  return false;
}
function closeAgentFirewallAddressOverlay(agentID){
  const overlay = document.getElementById('agentFirewallAddressOverlay_' + agentID);
  if (overlay) overlay.classList.remove('active');
  return false;
}
function agentFirewallRuleCount(agentID){
  return document.querySelectorAll('#agentFirewallRulesBody_' + agentID + ' tr[data-firewall-rule-id]').length;
}
function updateAgentFirewallEmptyState(agentID){
  const empty = document.getElementById('agentFirewallEmpty_' + agentID);
  const table = document.getElementById('agentFirewallTable_' + agentID);
  const count = agentFirewallRuleCount(agentID);
  if (empty) empty.hidden = count !== 0;
  if (table) table.hidden = count === 0;
  const form = document.getElementById('agentFirewallToggleForm_' + agentID);
  if (form) form.dataset.firewallRuleCount = String(count);
}
function updateAgentFirewallUI(data){
  const agentID = data.agent_id || '';
  const enabled = Boolean(data.firewall_enabled);
  const status = document.getElementById('agentFirewallStatus_' + agentID);
  const form = document.getElementById('agentFirewallToggleForm_' + agentID);
  const button = document.getElementById('agentFirewallToggleButton_' + agentID);
  if (status) {
    status.textContent = enabled ? 'Enabled' : 'Disabled';
    status.className = enabled ? 'logging-status-enabled' : 'firewall-status-disabled';
  }
  if (form) {
    const nextValue = enabled ? '0' : '1';
    ensureHiddenInput(form, 'enabled', nextValue);
  }
  if (button) {
    button.textContent = enabled ? 'Disable firewall' : 'Enable firewall';
    button.classList.toggle('warn', enabled);
  }
}
function appendAgentFirewallRule(agentID, rule){
  const body = document.getElementById('agentFirewallRulesBody_' + agentID);
  if (!body || !rule) return;
  const row = document.createElement('tr');
  row.id = 'agentFirewallRule_' + rule.id;
  row.dataset.firewallRuleId = rule.id || '';
  const typeCell = document.createElement('td');
  typeCell.textContent = rule.type || '';
  const dateCell = document.createElement('td');
  dateCell.textContent = rule.date_added || '';
  const valueCell = document.createElement('td');
  valueCell.textContent = rule.value || '';
  const actionsCell = document.createElement('td');
  const button = document.createElement('button');
  button.className = 'btn warn';
  button.type = 'button';
  button.textContent = 'Remove';
  button.addEventListener('click', () => deleteAgentFirewallRule(agentID, rule.id || ''));
  actionsCell.appendChild(button);
  row.appendChild(typeCell);
  row.appendChild(dateCell);
  row.appendChild(valueCell);
  row.appendChild(actionsCell);
  body.appendChild(row);
  updateAgentFirewallEmptyState(agentID);
}
async function addAgentFirewallAddress(event, agentID){
  event.preventDefault();
  const form = event.currentTarget;
  const button = form.querySelector('button[type="submit"]');
  if (!form.reportValidity()) return false;
  if (button) button.disabled = true;
  try {
    const response = await fetch(form.action, {
      method: 'POST',
      credentials: 'same-origin',
      headers: {'Accept': 'application/json', 'X-Requested-With': 'fetch'},
      body: new FormData(form),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.message || data.error || 'Request failed.');
    appendAgentFirewallRule(agentID, data.rule);
    closeAgentFirewallAddressOverlay(agentID);
    const slot = document.getElementById('agentFirewallFlash_' + agentID);
    if (slot) slot.replaceChildren(createFlash('success', data.message || 'Firewall address added.'));
  } catch (err) {
    const slot = document.getElementById('agentFirewallAddressFlash_' + agentID);
    if (slot) slot.replaceChildren(createFlash('error', err.message || String(err)));
  } finally {
    if (button) button.disabled = false;
  }
  return false;
}
async function deleteAgentFirewallRule(agentID, ruleID){
  if (!agentID || !ruleID) return false;
  const body = new URLSearchParams();
  body.set('csrf_token', csrfTokenValue());
  body.set('agent_id', agentID);
  body.set('rule_id', ruleID);
  try {
    const response = await fetch('/agents/firewall/delete', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {'Accept': 'application/json', 'X-Requested-With': 'fetch', 'Content-Type': 'application/x-www-form-urlencoded'},
      body: body.toString(),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.message || data.error || 'Request failed.');
    document.getElementById('agentFirewallRule_' + ruleID)?.remove();
    updateAgentFirewallEmptyState(agentID);
    const slot = document.getElementById('agentFirewallFlash_' + agentID);
    if (slot) slot.replaceChildren(createFlash('success', data.message || 'Firewall address removed.'));
  } catch (err) {
    const slot = document.getElementById('agentFirewallFlash_' + agentID);
    if (slot) slot.replaceChildren(createFlash('error', err.message || String(err)));
  }
  return false;
}
function copyAgentSkillCommand(id, button){
  copyElementText(id, button);
}
function copyTextToClipboard(text){
  if (navigator.clipboard && window.isSecureContext) {
    return navigator.clipboard.writeText(text);
  }
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', 'readonly');
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  textarea.style.pointerEvents = 'none';
  document.body.appendChild(textarea);
  textarea.select();
  const ok = document.execCommand('copy');
  document.body.removeChild(textarea);
  return ok ? Promise.resolve() : Promise.reject(new Error('Copy failed'));
}
function flashCopyButton(button){
  if (!button) return;
  if (button.classList.contains('copy-btn')) {
    const original = button.textContent;
    button.textContent = 'Copied';
    setTimeout(() => { button.textContent = original; }, 1200);
    return;
  }
  const originalTitle = button.dataset.originalTitle || button.title || '';
  button.dataset.originalTitle = originalTitle;
  button.classList.add('copied');
  button.setAttribute('aria-label', 'Copied');
  button.title = 'Copied';
  if (button._copyFlashTimer) {
    clearTimeout(button._copyFlashTimer);
  }
  button._copyFlashTimer = setTimeout(() => {
    button.classList.remove('copied');
    button.setAttribute('aria-label', originalTitle);
    button.title = originalTitle;
  }, 1200);
}
function copyElementText(id, button){
  const el = document.getElementById(id);
  if (!el) {
    return;
  }
  copyTextToClipboard(el.textContent.trim()).then(() => {
    flashCopyButton(button);
  }).catch(() => {});
}
function syncAgentSkillDownloadButton(selectID, buttonID){
  const select = document.getElementById(selectID || 'agentSkillPlatform');
  const button = document.getElementById(buttonID || 'agentSkillDownloadButton');
  if (select && button) {
    button.disabled = !select.value;
  }
}
function sizeSelectToOptions(select){
  if (!select) return;
  const measurer = document.createElement('span');
  const styles = window.getComputedStyle(select);
  measurer.style.position = 'absolute';
  measurer.style.visibility = 'hidden';
  measurer.style.whiteSpace = 'nowrap';
  measurer.style.font = styles.font;
  document.body.appendChild(measurer);
  const longest = [...select.options].reduce((max, option) => {
    measurer.textContent = option.textContent.trim();
    return Math.max(max, measurer.getBoundingClientRect().width);
  }, 0);
  measurer.remove();
  const padding = parseFloat(styles.paddingLeft || '0') + parseFloat(styles.paddingRight || '0');
  const border = parseFloat(styles.borderLeftWidth || '0') + parseFloat(styles.borderRightWidth || '0');
  const arrowAllowance = 28;
  const minWidth = 8 * parseFloat(styles.fontSize || '16');
  let width = Math.ceil(Math.max(minWidth, longest + padding + border + arrowAllowance));
  const maxChars = parseFloat(select.dataset.sizeMaxCh || '0');
  if (maxChars > 0) {
    width = Math.min(width, Math.ceil((maxChars * parseFloat(styles.fontSize || '16')) + padding + border + arrowAllowance));
  }
  select.style.width = width + 'px';
}
function initAutoSizedSelects(){
  document.querySelectorAll('select[data-size-to-options="true"]').forEach(sizeSelectToOptions);
}
function startAgentSkillDownload(agentID, selectID, buttonID){
  const select = document.getElementById(selectID || 'agentSkillPlatform');
  const platform = select ? select.value : '';
  agentID = String(agentID || '').trim();
  if (!platform || !agentID) {
    syncAgentSkillDownloadButton(selectID, buttonID);
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
  if (error) {
    const errorText = error.querySelector('span');
    if (errorText) errorText.textContent = '';
    error.hidden = true;
  }
  if (installNote) installNote.textContent = '';
  if (postInstallNote) postInstallNote.textContent = '';
  const body = new URLSearchParams();
  body.append('csrf_token', csrfTokenValue());
  body.append('platform', platform);
  body.append('agent_id', agentID);
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
      if (error) {
        const errorText = error.querySelector('span');
        if (errorText) errorText.textContent = err.message || String(err);
        error.hidden = false;
      }
      if (installCommand) installCommand.textContent = '';
    });
  return false;
}
function persistWorkspaceOrder(){
  const list = document.getElementById('workspaceList');
  if (!list) return;
  const body = new URLSearchParams();
  body.append('csrf_token', csrfTokenValue());
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
  if (list.dataset.aiwpWorkspaceDragInitialized === 'true') return;
  list.dataset.aiwpWorkspaceDragInitialized = 'true';
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
    if (form.dataset.aiwpTrackedInitialized === 'true') return;
    form.dataset.aiwpTrackedInitialized = 'true';
    let savedSnapshot = trackedFormSnapshot(form);
    const buttons = trackedFormButtons(form);
    const syncButtons = () => {
      const changed = trackedFormSnapshot(form) !== savedSnapshot;
      buttons.forEach(button => { button.disabled = !changed; });
    };
    form.aiwpSyncTrackedState = syncButtons;
    form.aiwpResetTrackedState = () => {
      savedSnapshot = trackedFormSnapshot(form);
      syncButtons();
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
    if (form.dataset.aiwpRequiredInitialized === 'true') return;
    form.dataset.aiwpRequiredInitialized = 'true';
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
let sessionTimeLeftTimer = null;
let savedCurrentTimeTimer = null;
function formatSavedCurrentTime(now, timezone){
  try {
    const parts = new Intl.DateTimeFormat('en-CA', {
      timeZone: timezone || 'UTC',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hourCycle: 'h23',
      timeZoneName: 'short',
    }).formatToParts(now).reduce((acc, part) => {
      acc[part.type] = part.value;
      return acc;
    }, {});
    return (parts.year + '-' + parts.month + '-' + parts.day + ' ' + parts.hour + ':' + parts.minute + ':' + parts.second + ' ' + (parts.timeZoneName || '')).trim();
  } catch (err) {
    return now.toISOString().slice(0, 19).replace('T', ' ') + ' UTC';
  }
}
function initSavedCurrentTimeClock(){
  const target = document.getElementById('savedCurrentTime');
  const timezoneTarget = document.getElementById('savedTimezone');
  if (!target || !timezoneTarget) return;
  const sync = () => {
    target.textContent = formatSavedCurrentTime(new Date(), timezoneTarget.textContent || 'UTC');
  };
  if (savedCurrentTimeTimer) {
    window.clearInterval(savedCurrentTimeTimer);
  }
  sync();
  savedCurrentTimeTimer = window.setInterval(sync, 1000);
}
function formatSessionTimeLeft(expiresAtMs){
  const diffMs = Math.max(0, expiresAtMs - Date.now());
  const totalSeconds = Math.floor(diffMs / 1000);
  const days = Math.floor(totalSeconds / 86400);
  const hours = Math.floor((totalSeconds % 86400) / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  const dayLabel = days === 1 ? 'day' : 'days';
  return String(days).padStart(2, '0') + ' ' + dayLabel + ', ' + String(hours).padStart(2, '0') + ' h ' + String(minutes).padStart(2, '0') + ' m ' + String(seconds).padStart(2, '0') + ' s';
}
function initSessionTimeLeftClock(){
  const target = document.getElementById('sessionTimeLeft');
  if (!target) return;
  const sync = () => {
    const expiresAtMs = Number(target.dataset.expiresAtMs || '0');
    target.textContent = expiresAtMs ? formatSessionTimeLeft(expiresAtMs) : '';
  };
  if (sessionTimeLeftTimer) {
    window.clearInterval(sessionTimeLeftTimer);
  }
  sync();
  sessionTimeLeftTimer = window.setInterval(sync, 1000);
}
function initAdminUsersPage(){
  const root = document.querySelector('[data-admin-users-page]');
  if (!root) return;
  if (root.dataset.aiwpAdminUsersInitialized === 'true') return;
  root.dataset.aiwpAdminUsersInitialized = 'true';
  const tableBody = root.querySelector('[data-admin-users-body]');
  if (!tableBody) return;
  const rows = Array.from(tableBody.querySelectorAll('[data-admin-user-row]'));
  const emptyState = document.getElementById('adminUsersEmptyState');
  const countTarget = document.getElementById('adminUsersCount');
  const paginationTarget = document.getElementById('adminUsersPagination');
  const searchInput = document.getElementById('adminUserSearch');
  const roleFilter = document.getElementById('adminUserRoleFilter');
  const statusFilter = document.getElementById('adminUserStatusFilter');
  const pageSizeSelect = document.getElementById('adminUserPageSize');
  const sortButtons = Array.from(root.querySelectorAll('[data-admin-sort]'));
  const state = {
    search: '',
    role: '',
    status: '',
    sort: '',
    dir: '',
    page: 1,
    pageSize: Math.min(Math.max(Number(pageSizeSelect?.value || '50') || 50, 1), 250),
  };
  const statusRank = {active: 0, pending: 1, suspended: 2};
  const roleRank = {administrator: 0, user: 1};
  const normalize = value => String(value || '').toLowerCase().trim();
  const userCountLabel = count => count === 1 ? 'account' : 'accounts';
  const compareText = (left, right) => left.localeCompare(right, undefined, {sensitivity: 'base'});
  const compareNumber = (left, right) => left - right;
  const compareRows = (left, right) => {
    const sortKey = state.sort;
    if (sortKey === 'last_activity') {
      return compareNumber(Number(left.dataset.lastActivity || '0'), Number(right.dataset.lastActivity || '0'));
    }
    if (sortKey === 'status') {
      const leftRank = statusRank[normalize(left.dataset.status)] ?? 99;
      const rightRank = statusRank[normalize(right.dataset.status)] ?? 99;
      return leftRank - rightRank || compareText(normalize(left.dataset.name), normalize(right.dataset.name));
    }
    if (sortKey === 'role') {
      const leftRank = roleRank[normalize(left.dataset.role)] ?? 99;
      const rightRank = roleRank[normalize(right.dataset.role)] ?? 99;
      return leftRank - rightRank || compareText(normalize(left.dataset.name), normalize(right.dataset.name));
    }
    return compareText(normalize(left.dataset[sortKey] || ''), normalize(right.dataset[sortKey] || ''));
  };
  const filteredRows = () => {
    const search = normalize(state.search);
    const role = normalize(state.role);
    const status = normalize(state.status);
    const matches = rows.filter(row => {
      if (search && !normalize(row.dataset.search || '').includes(search)) return false;
      if (role && normalize(row.dataset.role || '') !== role) return false;
      if (status && normalize(row.dataset.status || '') !== status) return false;
      return true;
    });
    if (state.sort) {
      matches.sort((left, right) => {
        const result = compareRows(left, right);
        return state.dir === 'desc' ? -result : result;
      });
    }
    return matches;
  };
  const updateSortButtons = () => {
    sortButtons.forEach(button => {
      const active = button.dataset.adminSort === state.sort;
      button.classList.toggle('active', active);
      button.dataset.sortDir = active ? state.dir : '';
      button.setAttribute('aria-pressed', active ? 'true' : 'false');
    });
  };
  const buildPaginationButton = (label, page, disabled, extraClass) => {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'admin-users-page-btn' + (extraClass ? ' ' + extraClass : '');
    button.textContent = label;
    button.disabled = disabled;
    if (!disabled) {
      button.addEventListener('click', () => {
        state.page = page;
        render();
      });
    }
    return button;
  };
  const renderPagination = totalPages => {
    if (!paginationTarget) return;
    paginationTarget.replaceChildren();
    if (totalPages <= 1) return;
    paginationTarget.appendChild(buildPaginationButton('Prev', Math.max(1, state.page - 1), state.page <= 1, 'secondary'));
    const start = Math.max(1, state.page - 2);
    const end = Math.min(totalPages, start + 4);
    const adjustedStart = Math.max(1, end - 4);
    for (let page = adjustedStart; page <= end; page += 1) {
      const button = buildPaginationButton(String(page), page, false, page === state.page ? 'active' : '');
      button.setAttribute('aria-current', page === state.page ? 'page' : 'false');
      paginationTarget.appendChild(button);
    }
    paginationTarget.appendChild(buildPaginationButton('Next', Math.min(totalPages, state.page + 1), state.page >= totalPages, 'secondary'));
  };
  const render = () => {
    const matches = filteredRows();
    const total = matches.length;
    const totalPages = Math.max(1, Math.ceil(total / state.pageSize));
    state.page = Math.min(Math.max(state.page, 1), totalPages);
    const startIndex = total === 0 ? 0 : (state.page - 1) * state.pageSize;
    const endIndex = Math.min(total, startIndex + state.pageSize);
    rows.forEach(row => { row.hidden = true; });
    matches.slice(startIndex, endIndex).forEach(row => {
      row.hidden = false;
      tableBody.appendChild(row);
    });
    if (emptyState) tableBody.appendChild(emptyState);
    if (emptyState) emptyState.hidden = total !== 0;
    if (countTarget) {
      if (total === 0) {
        countTarget.textContent = 'No matching user accounts.';
      } else {
        countTarget.textContent = 'Showing ' + String(startIndex + 1) + '-' + String(endIndex) + ' of ' + String(total) + ' ' + userCountLabel(total) + '.';
      }
    }
    updateSortButtons();
    renderPagination(totalPages);
  };
  searchInput?.addEventListener('input', event => {
    state.search = event.target.value || '';
    state.page = 1;
    render();
  });
  roleFilter?.addEventListener('change', event => {
    state.role = event.target.value || '';
    state.page = 1;
    render();
  });
  statusFilter?.addEventListener('change', event => {
    state.status = event.target.value || '';
    state.page = 1;
    render();
  });
  pageSizeSelect?.addEventListener('change', event => {
    const nextSize = Number(event.target.value || '50') || 50;
    state.pageSize = Math.min(Math.max(nextSize, 1), 250);
    state.page = 1;
    render();
  });
  sortButtons.forEach(button => {
    button.addEventListener('click', () => {
      const nextSort = button.dataset.adminSort || 'name';
      if (state.sort === nextSort) {
        if (state.dir === 'asc') {
          state.dir = 'desc';
        } else if (state.dir === 'desc') {
          state.sort = '';
          state.dir = '';
        } else {
          state.dir = 'asc';
        }
      } else {
        state.sort = nextSort;
        state.dir = 'asc';
      }
      state.page = 1;
      render();
    });
  });
  const table = root.querySelector('.admin-users-table');
  const resizeHandles = Array.from(root.querySelectorAll('[data-admin-column-resize]'));
  resizeHandles.forEach(handle => {
    handle.addEventListener('click', event => event.stopPropagation());
    handle.addEventListener('mousedown', event => {
      event.preventDefault();
      event.stopPropagation();
      const column = root.querySelector('col[data-admin-column="' + handle.dataset.adminColumnResize + '"]');
      const header = handle.closest('th');
      if (!column || !header) return;
      const columns = Array.from(root.querySelectorAll('col[data-admin-column]'));
      const headerCells = Array.from(header.parentElement?.children || []);
      const columnWidths = headerCells.map(cell => cell.getBoundingClientRect().width);
      const tableStartWidth = columnWidths.reduce((total, width) => total + width, 0);
      columns.forEach((col, index) => {
        if (columnWidths[index]) col.style.width = String(columnWidths[index]) + 'px';
      });
      if (table) table.style.width = String(tableStartWidth) + 'px';
      const startX = event.clientX;
      const startWidth = header.getBoundingClientRect().width;
      const minWidth = Number(handle.dataset.minWidth || '80');
      table?.classList.add('is-resizing');
      const onMove = moveEvent => {
        const nextWidth = Math.max(minWidth, startWidth + moveEvent.clientX - startX);
        const delta = nextWidth - startWidth;
        column.style.width = String(nextWidth) + 'px';
        if (table) table.style.width = String(Math.max(920, tableStartWidth + delta)) + 'px';
      };
      const onUp = () => {
        table?.classList.remove('is-resizing');
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
      };
      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
    });
  });
  render();
}
function selectPolicyForEditing(select){
  navigateWithinApp('/policies?policy=' + encodeURIComponent(select.value));
}
function applySystemDefaultPolicy(){
  document.querySelectorAll('[data-policy-capability="true"]').forEach(input => {
    input.checked = input.dataset.systemDefault === 'true';
    input.dispatchEvent(new Event('change', {bubbles: true}));
  });
  document.querySelectorAll('[data-policy-review="true"]').forEach(input => {
    input.checked = false;
    input.dispatchEvent(new Event('change', {bubbles: true}));
  });
}
function syncPolicyReviewControls(){
  document.querySelectorAll('[data-policy-review="true"]').forEach(input => {
    const capability = document.querySelector('[data-policy-capability="true"][value="' + CSS.escape(input.value) + '"]');
    const enabled = Boolean(capability && capability.checked && !capability.disabled);
    input.disabled = !enabled;
    if (!enabled) input.checked = false;
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
function dismissFlash(button){
  const flash = button.closest('.flash');
  if (flash) flash.remove();
}
function initSidebarScrollContain(){
  const sidebar = document.querySelector('.sidebar');
  if (!sidebar) return;
  sidebar.addEventListener('wheel', event => {
    if (event.deltaY === 0) return;
    const atTop = sidebar.scrollTop <= 0;
    const atBottom = sidebar.scrollTop + sidebar.clientHeight >= sidebar.scrollHeight - 1;
    if ((event.deltaY < 0 && atTop) || (event.deltaY > 0 && atBottom)) {
      event.preventDefault();
    }
  }, {passive:false});
}
function formatFlashDetail(detail){
  if (typeof detail === 'string') {
    return detail;
  }
  if (!detail || typeof detail !== 'object') {
    return String(detail || '');
  }
  const left = [];
  if (detail.query_type) left.push(detail.query_type);
  if (detail.hostname) left.push(detail.hostname);
  let line = left.join(' ');
  if (detail.dns_server) line += (line ? ' via ' : '') + detail.dns_server;
  if (detail.result) line += (line ? ' -> ' : '') + detail.result;
  if (detail.notes) line += (line ? ' (' : '') + detail.notes + (line ? ')' : '');
  return line || JSON.stringify(detail);
}
function toggleDetails(button){
  const panel = document.getElementById(button.getAttribute('aria-controls'));
  if (!panel) return;
  const expanded = button.getAttribute('aria-expanded') === 'true';
  const showLabel = button.dataset.showLabel || 'Show more details';
  const hideLabel = button.dataset.hideLabel || 'Hide details';
  button.setAttribute('aria-expanded', expanded ? 'false' : 'true');
  panel.hidden = expanded;
  button.textContent = expanded ? showLabel : hideLabel;
}
function createFlash(kind, message, details){
  const flash = document.createElement('div');
  flash.className = 'flash ' + (kind === 'error' ? 'flash-error' : 'flash-success');
  const body = document.createElement('div');
  body.className = 'flash-body';
  const text = document.createElement('span');
  text.textContent = message;
  body.appendChild(text);
  if (Array.isArray(details) && details.length > 0) {
    const list = document.createElement('ul');
    list.className = 'flash-details';
    details.forEach(detail => {
      const item = document.createElement('li');
      item.textContent = formatFlashDetail(detail);
      list.appendChild(item);
    });
    body.appendChild(list);
  }
  const close = document.createElement('button');
  close.className = 'flash-close';
  close.type = 'button';
  close.setAttribute('aria-label', 'Close message');
  close.textContent = 'x';
  close.addEventListener('click', () => flash.remove());
  flash.appendChild(body);
  flash.appendChild(close);
  return flash;
}
function formFlashSlot(form){
  if (form.dataset.flashTarget) {
    const target = document.getElementById(form.dataset.flashTarget);
    if (target) return target;
  }
  let slot = form.nextElementSibling;
  if (!slot || !slot.classList.contains('form-flash-slot')) {
    slot = document.createElement('div');
    slot.className = 'form-flash-slot';
    slot.setAttribute('aria-live', 'polite');
    form.insertAdjacentElement('afterend', slot);
  }
  return slot;
}
function showFormFlash(form, kind, message, details){
  const slot = formFlashSlot(form);
  slot.replaceChildren(createFlash(kind, message, details));
}
function ensureHiddenInput(form, name, value){
  let input = form.querySelector('input[type="hidden"][name="' + name + '"]');
  if (!input) {
    input = document.createElement('input');
    input.type = 'hidden';
    input.name = name;
    form.appendChild(input);
  }
  input.value = value;
  return input;
}
function removeHiddenInput(form, name){
  form.querySelector('input[type="hidden"][name="' + name + '"]')?.remove();
}
function updateAgentToggleUI(data){
  const agentID = data.agent_id || '';
  const status = document.getElementById('agentStatus_' + agentID);
  const form = document.getElementById('agentToggleForm_' + agentID);
  const button = document.getElementById('agentToggleButton_' + agentID);
  if (typeof data.enabled !== 'boolean' || !form || !button) return;
  if (status) {
    status.textContent = data.enabled ? 'Enabled' : 'Access suspended';
    status.className = data.enabled ? 'logging-status-enabled' : 'logging-status-disabled';
  }
  if (data.enabled) {
    removeHiddenInput(form, 'enabled');
    button.textContent = 'Suspend access';
    button.classList.add('warn');
  } else {
    ensureHiddenInput(form, 'enabled', '1');
    button.textContent = 'Restore access';
    button.classList.remove('warn');
  }
}
function updateAgentTokenUI(data){
  const agentID = data.agent_id || '';
  const token = document.getElementById('agentTokenHint_' + agentID);
  if (token && data.token_hint) token.textContent = data.token_hint;
  setAgentSkillWarning(data);
}
function csrfTokenValue(){
  return document.querySelector('input[name="csrf_token"]')?.value || document.querySelector('meta[name="csrf-token"]')?.content || '';
}
function appNameValue(){
  return window.AIWP_APP_CONFIG?.appName || 'AI Workspace Proxy';
}
function ensureAgentSkillWarning(agentID){
  let warning = document.getElementById('agentSkillWarning_' + agentID);
  if (warning) return warning;
  const card = document.getElementById('agent-' + agentID);
  const title = document.getElementById('agentName_' + agentID);
  if (!card || !title) return null;
  warning = document.createElement('div');
  warning.id = 'agentSkillWarning_' + agentID;
  warning.className = 'flash flash-error agent-skill-warning';

  const body = document.createElement('span');
  const titleLine = document.createElement('p');
  const titleStrong = document.createElement('strong');
  titleStrong.textContent = 'SKILL UPDATE REQUIRED';
  titleLine.appendChild(titleStrong);
  const detailLine = document.createElement('p');
  detailLine.textContent = 'Update the installed skill so this agent receives the latest access settings. You may re-download the skill package or ask your agent to update the skill.';
  const reasons = document.createElement('ul');
  reasons.id = 'agentSkillWarningReasons_' + agentID;
  body.appendChild(titleLine);
  body.appendChild(detailLine);
  body.appendChild(reasons);

  const close = document.createElement('button');
  close.className = 'flash-close';
  close.type = 'button';
  close.setAttribute('aria-label', 'Dismiss agent skill update warning');
  close.textContent = 'x';
  close.addEventListener('click', () => dismissAgentSkillWarning(agentID));

  warning.appendChild(body);
  warning.appendChild(close);
  title.insertAdjacentElement('afterend', warning);
  return warning;
}
function setAgentSkillWarning(data){
  const agentID = data.agent_id || '';
  if (!agentID) return;
  const stale = Boolean(data.skill_update_required);
  const badge = document.getElementById('agentSkillStaleBadge_' + agentID);
  const warning = stale ? ensureAgentSkillWarning(agentID) : document.getElementById('agentSkillWarning_' + agentID);
  const reasons = document.getElementById('agentSkillWarningReasons_' + agentID);
  if (badge) badge.hidden = !stale;
  if (warning) warning.hidden = !stale;
  if (reasons && Array.isArray(data.skill_update_reasons)) {
    reasons.replaceChildren();
    data.skill_update_reasons.forEach(reason => {
      const item = document.createElement('li');
      item.textContent = reason;
      reasons.appendChild(item);
    });
  }
}
async function dismissAgentSkillWarning(agentID){
  const body = new URLSearchParams();
  body.set('csrf_token', csrfTokenValue());
  body.set('agent_id', agentID);
  try {
    const response = await fetch('/agents/skill-warning/dismiss', {
      method: 'POST',
      credentials: 'same-origin',
      headers: {'Accept': 'application/json', 'X-Requested-With': 'fetch', 'Content-Type': 'application/x-www-form-urlencoded'},
      body: body.toString(),
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.message || data.error || 'Request failed.');
    setAgentSkillWarning(data);
  } catch (err) {
    const slot = document.getElementById('agentActionsFlash_' + agentID);
    if (slot) slot.replaceChildren(createFlash('error', err.message));
  }
}
function updateAgentProfileUI(data){
  const agentID = data.agent_id || '';
  const name = data.friendly_name || '';
  const location = data.default_location || '';
  if (name) {
    const profileName = document.getElementById('agentName_' + agentID);
    const navName = document.getElementById('agentNavName_' + agentID);
    const pageTitle = document.getElementById('pageCrumbTitle');
    if (profileName) profileName.textContent = name;
    if (navName) navName.textContent = name;
    if (pageTitle && document.body.dataset.activeSection === 'agents') pageTitle.textContent = name;
  }
  if (location) {
    const profileLocation = document.getElementById('agentLocation_' + agentID);
    const navLocation = document.getElementById('agentNavLocation_' + agentID);
    if (profileLocation) profileLocation.textContent = location;
    if (navLocation) navLocation.textContent = location;
  }
  setAgentSkillWarning(data);
  closeAgentEditOverlay();
}
function updateAgentDeletedUI(data){
  const agentID = data.agent_id || '';
  document.getElementById('agentProfileBlock_' + agentID)?.remove();
  document.getElementById('agentNav_' + agentID)?.remove();
  const main = document.querySelector('main');
  if (!main) return;
  const card = document.createElement('div');
  card.className = 'card';
  card.appendChild(createFlash('success', data.message || 'Agent profile deleted.'));
  const title = document.createElement('h2');
  title.textContent = 'Agent profile deleted';
  const text = document.createElement('p');
  text.textContent = 'Select another agent from the left menu or create a new one.';
  card.appendChild(title);
  card.appendChild(text);
  main.prepend(card);
}
function truncatePolicyOptionLabel(text){
  text = String(text || '').trim();
  const maxChars = 75;
  if (text.length <= maxChars) return text;
  return text.slice(0, maxChars - 1) + '…';
}
function policyOptionLabel(option, defaultPolicyID){
  const name = option.dataset.policyName || option.textContent;
  const isSystem = option.dataset.systemPolicy === 'true';
  const fullLabel = isSystem
    ? 'System policy' + (option.value === defaultPolicyID ? ' (default)' : '') + ' (not editable & read permissions only)'
    : (option.value === defaultPolicyID ? name + ' (default)' : name);
  option.title = fullLabel;
  return truncatePolicyOptionLabel(fullLabel);
}
function refreshPolicyOptionLabels(defaultPolicyID){
  document.querySelectorAll('#policy_select option').forEach(option => {
    option.textContent = policyOptionLabel(option, defaultPolicyID);
  });
  document.querySelectorAll('#default_policy_id option').forEach(option => {
    option.textContent = policyOptionLabel(option, defaultPolicyID);
  });
  sizeSelectToOptions(document.getElementById('policy_select'));
  sizeSelectToOptions(document.getElementById('default_policy_id'));
}
function updatePolicyEditorOptions(policyID, policyName){
  if (!policyID || !policyName) return;
  document.querySelectorAll('option[data-policy-name][value="' + policyID + '"]').forEach(option => {
    option.dataset.policyName = policyName;
  });
  const defaultSelect = document.getElementById('default_policy_id');
  refreshPolicyOptionLabels(defaultSelect ? defaultSelect.value : '');
}
function showOrganizationAdminVerifiedOverlay(data){
  const overlay = document.getElementById('organizationAdminVerifiedOverlay');
  const title = document.getElementById('organizationAdminVerifiedOverlayTitle');
  const body = document.getElementById('organizationAdminVerifiedOverlayBody');
  if (!overlay || !title || !body) return;
  const domain = String(data.domain || '').trim();
  title.textContent = 'Domain ' + domain + ' successfully verified';
  body.innerHTML = '';
  const paragraphs = [
    'The domain ' + domain + ' is now successfully verified.',
    'You are now the primary admin for this domain in AI Workspace Proxy, and the DNS TXT record can safely be deleted.',
    'When you close this message, you will be redirected to the Admin Dashboard.',
  ];
  paragraphs.forEach(text => {
    const p = document.createElement('p');
    p.textContent = text;
    body.appendChild(p);
  });
  overlay.classList.add('active');
  document.body.classList.add('modal-open');
}
function closeOrganizationAdminVerifiedOverlay(){
  const overlay = document.getElementById('organizationAdminVerifiedOverlay');
  if (overlay) overlay.classList.remove('active');
  document.body.classList.remove('modal-open');
  window.location.assign('/admin/');
}
function showOrganizationAdminClaimFlow(){
  const claimIntro = document.getElementById('organizationAdminClaimIntro');
  const pendingBlock = document.getElementById('organizationAdminPendingBlock');
  if (claimIntro) claimIntro.hidden = true;
  if (pendingBlock) pendingBlock.hidden = false;
  return false;
}
function startOrganizationAdminCooldown(form){
  const button = form?.querySelector('button[type="submit"]');
  if (!button) return;
  const baseLabel = button.dataset.baseLabel || button.textContent;
  button.dataset.baseLabel = baseLabel;
  let remaining = 10;
  button.disabled = true;
  button.textContent = baseLabel + ' (' + remaining + 's)';
  const timer = window.setInterval(() => {
    remaining -= 1;
    if (remaining <= 0) {
      window.clearInterval(timer);
      button.textContent = baseLabel;
      button.disabled = false;
      return;
    }
    button.textContent = baseLabel + ' (' + remaining + 's)';
  }, 1000);
}
function applyAjaxFormUpdate(form, data){
  switch (form.dataset.ajaxUpdate) {
  case 'org-domain-verify':
    showOrganizationAdminVerifiedOverlay(data);
    break;
  case 'policy-editor':
    updatePolicyEditorOptions(data.policy_id, data.policy_name);
    const title = document.getElementById('policyEditorSelectedTitle');
    if (title && data.policy_name) title.textContent = data.policy_name;
    if (Array.isArray(data.skill_update_agent_ids)) {
      data.skill_update_agent_ids.forEach(agentID => setAgentSkillWarning({
        agent_id: agentID,
        skill_update_required: true,
        skill_update_reasons: data.skill_update_reasons || [],
      }));
    }
    break;
  case 'policy-default':
    refreshPolicyOptionLabels(data.policy_id || '');
    const policySelect = document.getElementById('policy_select');
    if (policySelect && data.policy_id) {
      policySelect.value = data.policy_id;
    }
    break;
  case 'settings':
    document.getElementById('savedTimezone').textContent = data.timezone || '';
    document.getElementById('savedCurrentTime').textContent = data.current_time || '';
    initSavedCurrentTimeClock();
    break;
  case 'session-timeout':
    document.getElementById('savedSessionTimeoutHours').textContent = String(data.session_timeout_hours || '');
    document.getElementById('sessionTimeLeft').dataset.expiresAtMs = String(data.session_expires_at_unix_ms || '');
    initSessionTimeLeftClock();
    break;
  case 'agent-profile':
    updateAgentProfileUI(data);
    break;
  case 'workspace-friendly-name':
    document.querySelectorAll('[data-workspace-name]').forEach(el => {
      if (el.dataset.workspaceName === data.workspace) el.textContent = data.friendly_name || '';
    });
    const workspaceTitleName = document.getElementById('workspaceAccountTitleName');
    if (workspaceTitleName) workspaceTitleName.textContent = data.friendly_name || '';
    if (Array.isArray(data.skill_update_agent_ids)) {
      data.skill_update_agent_ids.forEach(agentID => setAgentSkillWarning({
        agent_id: agentID,
        skill_update_required: true,
        skill_update_reasons: data.skill_update_reasons || [],
      }));
    }
    break;
  case 'agent-toggle':
    updateAgentToggleUI(data);
    break;
  case 'agent-rotate':
    updateAgentTokenUI(data);
    break;
  case 'agent-grants':
    setAgentSkillWarning(data);
    break;
  case 'agent-firewall-toggle':
    updateAgentFirewallUI(data);
    break;
  case 'agent-delete':
    updateAgentDeletedUI(data);
    break;
  }
}
function initAjaxForms(){
  document.querySelectorAll('form[data-ajax-form="true"]').forEach(form => {
    if (form.dataset.aiwpAjaxInitialized === 'true') return;
    form.dataset.aiwpAjaxInitialized = 'true';
    form.addEventListener('submit', async event => {
      event.preventDefault();
      if (form.dataset.confirm && !confirm(form.dataset.confirm)) {
        form.aiwpSyncTrackedState?.();
        return;
      }
      if (form.dataset.confirmEmptyFirewall && form.querySelector('input[name="enabled"]')?.value === '1' && Number(form.dataset.firewallRuleCount || '0') === 0) {
        if (!confirm('Enable the firewall with no allowed IP addresses? This agent will be blocked until at least one address is added.')) {
          form.aiwpSyncTrackedState?.();
          return;
        }
      }
      if (!form.reportValidity()) {
        form.aiwpSyncTrackedState?.();
        return;
      }
      const buttons = trackedFormButtons(form);
      buttons.forEach(button => { button.disabled = true; });
      try {
        const response = await fetch(form.action, {
          method: form.method || 'POST',
          credentials: 'same-origin',
          headers: {'Accept': 'application/json', 'X-Requested-With': 'fetch'},
          body: new FormData(form),
        });
        const data = await response.json();
        if (!response.ok) {
          const error = new Error(data.message || data.error || 'Request failed.');
          error.flashDetails = data.details || [];
          throw error;
        }
        applyAjaxFormUpdate(form, data);
        if (form.aiwpResetTrackedState) {
          form.aiwpResetTrackedState();
        } else {
          buttons.forEach(button => { button.disabled = false; });
        }
        if (form.dataset.suppressSuccessFlash !== 'true') {
          showFormFlash(form, 'success', data.message || form.dataset.successMessage || 'Saved.');
        }
      } catch (err) {
        showFormFlash(form, 'error', err.message || String(err), err.flashDetails || []);
        if (form.dataset.ajaxUpdate === 'org-domain-verify') {
          startOrganizationAdminCooldown(form);
        } else if (form.aiwpSyncTrackedState) {
          form.aiwpSyncTrackedState();
        } else {
          buttons.forEach(button => { button.disabled = false; });
        }
      }
    });
  });
}
const LOG_DATE_MONTHS = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
const LOG_DATE_WEEKDAYS = ['Mon','Tue','Wed','Thu','Fri','Sat','Sun'];
let activeLogDatePicker = null;
function pad2(value){
  return String(value).padStart(2, '0');
}
function dateKeyFromDate(date){
  return date.getFullYear() + '-' + pad2(date.getMonth() + 1) + '-' + pad2(date.getDate());
}
function dateFromKey(key){
  const parts = String(key || '').split('-').map(Number);
  if (parts.length !== 3 || parts.some(Number.isNaN)) return null;
  return new Date(parts[0], parts[1] - 1, parts[2], 0, 0, 0, 0);
}
function compareDateKeys(a, b){
  return String(a || '').localeCompare(String(b || ''));
}
function formatLogDateKey(key){
  const date = dateFromKey(key);
  if (!date) return '';
  return date.getFullYear() + ' ' + LOG_DATE_MONTHS[date.getMonth()] + ' ' + pad2(date.getDate());
}
function setLogDateInput(input, key){
  if (!input || !key) return;
  input.dataset.value = key;
  input.value = formatLogDateKey(key);
}
function buildLogDatePicker(input, onSelect){
  const picker = document.createElement('div');
  picker.className = 'log-date-picker';
  picker.hidden = true;
  picker.setAttribute('role', 'dialog');
  picker.setAttribute('aria-label', 'Choose date');
  const header = document.createElement('div');
  header.className = 'log-date-picker-header';
  const prev = document.createElement('button');
  prev.type = 'button';
  prev.className = 'log-date-picker-nav';
  prev.textContent = '<';
  const title = document.createElement('div');
  title.className = 'log-date-picker-title';
  const next = document.createElement('button');
  next.type = 'button';
  next.className = 'log-date-picker-nav';
  next.textContent = '>';
  header.appendChild(prev);
  header.appendChild(title);
  header.appendChild(next);
  const weekdays = document.createElement('div');
  weekdays.className = 'log-date-weekdays';
  LOG_DATE_WEEKDAYS.forEach(day => {
    const item = document.createElement('span');
    item.textContent = day;
    weekdays.appendChild(item);
  });
  const days = document.createElement('div');
  days.className = 'log-date-days';
  picker.appendChild(header);
  picker.appendChild(weekdays);
  picker.appendChild(days);
  input.closest('.log-date-field').appendChild(picker);

  let viewDate = dateFromKey(input.dataset.value) || new Date();
  viewDate = new Date(viewDate.getFullYear(), viewDate.getMonth(), 1);
  const minKey = input.dataset.min || '';
  const maxKey = input.dataset.max || '';
  const inRange = key => (!minKey || compareDateKeys(key, minKey) >= 0) && (!maxKey || compareDateKeys(key, maxKey) <= 0);
  const render = () => {
    title.textContent = LOG_DATE_MONTHS[viewDate.getMonth()] + ' ' + viewDate.getFullYear();
    days.innerHTML = '';
    const first = new Date(viewDate.getFullYear(), viewDate.getMonth(), 1);
    const offset = (first.getDay() + 6) % 7;
    const gridStart = new Date(first);
    gridStart.setDate(first.getDate() - offset);
    for (let i = 0; i < 42; i++) {
      const day = new Date(gridStart);
      day.setDate(gridStart.getDate() + i);
      const key = dateKeyFromDate(day);
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'log-date-day';
      button.textContent = String(day.getDate());
      if (day.getMonth() !== viewDate.getMonth()) button.classList.add('outside');
      if (key === input.dataset.value) button.classList.add('selected');
      if (!inRange(key)) button.disabled = true;
      button.addEventListener('click', () => {
        setLogDateInput(input, key);
        picker.hidden = true;
        activeLogDatePicker = null;
        onSelect();
      });
      days.appendChild(button);
    }
  };
  const open = () => {
    if (activeLogDatePicker && activeLogDatePicker !== picker) activeLogDatePicker.hidden = true;
    viewDate = dateFromKey(input.dataset.value) || viewDate;
    viewDate = new Date(viewDate.getFullYear(), viewDate.getMonth(), 1);
    render();
    picker.hidden = false;
    activeLogDatePicker = picker;
  };
  prev.addEventListener('click', () => {
    viewDate.setMonth(viewDate.getMonth() - 1);
    render();
  });
  next.addEventListener('click', () => {
    viewDate.setMonth(viewDate.getMonth() + 1);
    render();
  });
  input.addEventListener('click', open);
  input.addEventListener('focus', open);
  document.querySelector('[data-date-target="' + input.id + '"]')?.addEventListener('click', open);
  return picker;
}
async function fetchJSON(url, options){
  const response = await fetch(url, Object.assign({credentials: 'same-origin'}, options || {}));
  const data = await response.json();
  if (!response.ok) {
    throw new Error(data.message || data.error || 'Request failed.');
  }
  return data;
}
function containScrollWithin(element){
  if (!element || element.dataset.scrollContained === 'true') return;
  element.dataset.scrollContained = 'true';
  element.addEventListener('wheel', event => {
    const deltaY = Number(event.deltaY || 0);
    if (!deltaY) return;
    const maxScrollTop = element.scrollHeight - element.clientHeight;
    if (maxScrollTop <= 0) {
      event.preventDefault();
      return;
    }
    const atTop = element.scrollTop <= 0;
    const atBottom = element.scrollTop >= maxScrollTop - 1;
    if ((deltaY < 0 && atTop) || (deltaY > 0 && atBottom)) {
      event.preventDefault();
    }
  }, {passive: false});
}
function initRequestLogConsole(){
  const tableEl = document.getElementById('requestLogTable');
  if (!tableEl) return;
  if (tableEl.dataset.aiwpLogConsoleInitialized === 'true') return;
  tableEl.dataset.aiwpLogConsoleInitialized = 'true';
  const statusEl = document.getElementById('logConsoleStatus');
  const columnPanel = document.getElementById('logColumnPanel');
  const filterPanel = document.getElementById('logFilterPanel');
  const columnOptions = document.getElementById('logColumnOptions');
  const filterGrid = document.getElementById('logRegexFilters');
  const logTypeFilters = document.getElementById('logTypeFilters');
  const pageSizeSelect = document.getElementById('logPageSize');
  const startInput = document.getElementById('logStart');
  const endInput = document.getElementById('logEnd');
  const detail = document.getElementById('logDetailOverlay');
  const detailGrid = document.getElementById('logDetailGrid');
  const detailBody = detail?.querySelector('.log-detail-body');
  const contextMenu = document.getElementById('logRowContextMenu');
  const copyDetailButton = document.getElementById('copyLogDetail');
  const closeDetailButtons = [document.getElementById('closeLogDetail'), document.getElementById('closeLogDetailX')].filter(Boolean);
  const rangeStartDate = tableEl.dataset.rangeStartDate || '';
  const rangeEndDate = tableEl.dataset.rangeEndDate || '';
  if (startInput && rangeStartDate) {
    startInput.dataset.min = rangeStartDate;
    startInput.dataset.max = rangeEndDate;
    setLogDateInput(startInput, rangeStartDate);
  }
  if (endInput && rangeEndDate) {
    endInput.dataset.min = rangeStartDate;
    endInput.dataset.max = rangeEndDate;
    setLogDateInput(endInput, rangeEndDate);
  }
  if (typeof Tabulator === 'undefined') {
    if (statusEl) statusEl.textContent = 'The log table library did not load.';
    return;
  }
  const state = {columns: [], columnsById: {}, logTypes: [], visible: [], widths: {}, pageSize: 100, table: null};
  let defaultVisibleColumns = [];
  let pendingColumnOrder = null;
  let logTableResizeTimer = null;
  const maxInitialColumnChars = 30;
  const setStatus = message => {
    if (statusEl) statusEl.textContent = message || '';
  };
  const widthForChars = chars => Math.max(48, Math.round((chars * 8.4) + 28));
  const defaultColumnMinWidth = col => widthForChars(Math.min(Math.max((col.label || '').length, 8), 16));
  const defaultColumnWidthGuess = col => widthForChars(Math.min(Math.max((col.label || '').length, 12), maxInitialColumnChars));
  const selectedColumnWidthTotal = () => {
    if (state.table) {
      const liveWidth = state.table.getColumns().reduce((total, column) => {
        const field = column.getField();
        if (!field) return total;
        return total + Number(column.getWidth() || 0);
      }, 0);
      if (liveWidth > 0) return liveWidth;
    }
    return selectedColumns().reduce((total, id) => {
      const col = state.columnsById[id] || {};
      return total + Number(state.widths[id] || defaultColumnWidthGuess(col));
    }, 0);
  };
  const syncLogTableScrollWidth = () => {
    const wrapper = tableEl.parentElement;
    const visibleWidth = wrapper ? wrapper.clientWidth : tableEl.clientWidth;
    tableEl.style.minWidth = Math.max(selectedColumnWidthTotal(), visibleWidth || 0) + 'px';
  };
  const redrawLogTable = () => {
    if (!state.table) return;
    window.clearTimeout(logTableResizeTimer);
    logTableResizeTimer = window.setTimeout(() => {
      syncLogTableScrollWidth();
      state.table.redraw();
    }, 80);
  };
  const selectedColumns = () => state.visible.filter(id => state.columnsById[id]);
  const selectedLogTypes = () => logTypeFilters ? [...logTypeFilters.querySelectorAll('input[data-log-type]:checked')].map(input => input.value) : [];
  const queryParams = page => {
    const params = new URLSearchParams();
    params.set('page', String(page || 1));
    params.set('page_size', String(state.pageSize));
    params.set('columns', selectedColumns().join(','));
    params.set('start_date', startInput ? startInput.dataset.value || '' : '');
    params.set('end_date', endInput ? endInput.dataset.value || '' : '');
    const agentID = new URLSearchParams(window.location.search).get('agent_id') || '';
    if (agentID) params.set('agent_id', agentID);
    const logTypes = selectedLogTypes();
    if (logTypes.length) {
      logTypes.forEach(logType => params.append('log_type', logType));
    } else {
      params.set('log_type', '');
    }
    filterGrid.querySelectorAll('[data-log-filter]').forEach(input => {
      if (input.value.trim()) {
        params.set('filter_' + input.dataset.logFilter, input.value.trim());
      }
    });
    return params;
  };
  const tabulatorColumns = () => selectedColumns().map(id => {
    const col = state.columnsById[id];
    const savedWidth = Number(state.widths[id] || 0);
    return {
      title: col.label,
      field: col.id,
      width: savedWidth > 0 ? savedWidth : undefined,
      minWidth: defaultColumnMinWidth(col),
      maxInitialWidth: widthForChars(maxInitialColumnChars),
      resizable: true,
      sorter: col.sorter || 'string',
      headerSort: false,
      tooltip: true,
    };
  });
  const saveViewSettings = async () => {
    const body = new URLSearchParams();
    body.set('csrf_token', csrfTokenValue());
    body.set('visible_columns', selectedColumns().join(','));
    body.set('column_widths', JSON.stringify(state.widths || {}));
    body.set('page_size', String(state.pageSize));
    await fetchJSON('/api/logs/request/view-settings', {
      method: 'POST',
      headers: {'Content-Type': 'application/x-www-form-urlencoded'},
      body,
    });
  };
  const columnPickerOrderedColumns = () => [...state.columns];
  const renderColumnPicker = () => {
    const byCategory = new Map();
    columnPickerOrderedColumns().forEach(col => {
      if (!byCategory.has(col.category)) byCategory.set(col.category, []);
      byCategory.get(col.category).push(col);
    });
    columnOptions.innerHTML = '';
    byCategory.forEach((columns, category) => {
      const section = document.createElement('section');
      section.className = 'column-category';
      const heading = document.createElement('h4');
      heading.textContent = category;
      const options = document.createElement('div');
      options.className = 'column-options';
      columns.forEach(col => {
        const label = document.createElement('label');
        const checkbox = document.createElement('input');
        checkbox.type = 'checkbox';
        checkbox.value = col.id;
        checkbox.checked = state.visible.includes(col.id);
        label.appendChild(checkbox);
        label.appendChild(document.createTextNode(col.label));
        options.appendChild(label);
      });
      section.appendChild(heading);
      section.appendChild(options);
      columnOptions.appendChild(section);
    });
  };
  const setColumnPickerSelection = columns => {
    const selected = new Set(columns);
    columnOptions.querySelectorAll('input[type="checkbox"]').forEach(input => {
      input.checked = selected.has(input.value);
    });
  };
  const selectedColumnPickerValues = () => [...columnOptions.querySelectorAll('input[type="checkbox"]:checked')].map(input => input.value);
  const defaultAwareColumnOrder = columns => {
    const selected = new Set(columns);
    const ordered = defaultVisibleColumns.filter(id => selected.has(id));
    const seen = new Set(ordered);
    columnPickerOrderedColumns().forEach(col => {
      if (selected.has(col.id) && !seen.has(col.id)) {
        ordered.push(col.id);
        seen.add(col.id);
      }
    });
    return ordered;
  };
  const selectedColumnsInPendingOrder = columns => {
    const selected = new Set(columns);
    const baseOrder = pendingColumnOrder && pendingColumnOrder.length ? pendingColumnOrder : state.visible;
    const ordered = baseOrder.filter(id => selected.has(id) && state.columnsById[id]);
    const seen = new Set(ordered);
    columnPickerOrderedColumns().forEach(col => {
      if (selected.has(col.id) && !seen.has(col.id)) {
        ordered.push(col.id);
        seen.add(col.id);
      }
    });
    return ordered;
  };
  const renderRegexFilters = () => {
    filterGrid.innerHTML = '';
    selectedColumns().forEach(id => {
      const col = state.columnsById[id];
      if (!col || !col.regexp_filter) return;
      const label = document.createElement('label');
      label.textContent = col.label;
      const input = document.createElement('input');
      input.type = 'text';
      input.placeholder = 'Regular expression';
      input.dataset.logFilter = col.id;
      label.appendChild(input);
      filterGrid.appendChild(label);
    });
  };
  const renderLogTypeFilters = () => {
    if (!logTypeFilters) return;
    logTypeFilters.innerHTML = '';
    state.logTypes.forEach(logType => {
      const label = document.createElement('label');
      label.className = 'log-type-filter';
      const checkbox = document.createElement('input');
      checkbox.type = 'checkbox';
      checkbox.value = logType.id;
      checkbox.dataset.logType = logType.id;
      checkbox.checked = true;
      checkbox.addEventListener('change', reloadTable);
      const text = document.createElement('span');
      text.textContent = logType.label;
      const help = document.createElement('span');
      help.className = 'risk-help';
      help.tabIndex = 0;
      help.setAttribute('role', 'img');
      help.setAttribute('aria-label', logType.description || logType.label);
      help.dataset.tooltip = logType.description || logType.label;
      help.textContent = '(?)';
      label.appendChild(checkbox);
      label.appendChild(text);
      label.appendChild(help);
      logTypeFilters.appendChild(label);
    });
  };
  const allExportableColumns = () => state.columns.filter(col => col.exportable !== false).map(col => col.id);
  const labelForColumn = id => (state.columnsById[id] || {}).label || id;
  const projectRowForColumns = (row, columns) => {
    const out = {};
    columns.forEach(id => { out[labelForColumn(id)] = row[id] === undefined || row[id] === null ? '' : row[id]; });
    return out;
  };
  const detailText = (row, columns) => JSON.stringify(projectRowForColumns(row, columns), null, 2);
  const copyText = async text => {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return;
    }
    const area = document.createElement('textarea');
    area.value = text;
    area.setAttribute('readonly', '');
    area.style.position = 'fixed';
    area.style.left = '-9999px';
    document.body.appendChild(area);
    area.select();
    document.execCommand('copy');
    area.remove();
  };
  let activeDetailRow = null;
  const closeDetail = () => {
    if (!detail) return;
    detail.hidden = true;
    document.body.classList.remove('modal-open');
    activeDetailRow = null;
  };
  const showDetail = row => {
    if (!detail || !detailGrid) return;
    activeDetailRow = row;
    detail.hidden = false;
    document.body.classList.add('modal-open');
    if (detailBody) detailBody.scrollTop = 0;
    detailGrid.innerHTML = '';
    state.columns.forEach(col => {
      const dt = document.createElement('dt');
      const dd = document.createElement('dd');
      dt.textContent = col.label;
      dd.textContent = row[col.id] === undefined || row[col.id] === null ? '' : String(row[col.id]);
      detailGrid.appendChild(dt);
      detailGrid.appendChild(dd);
    });
  };
  containScrollWithin(detailBody);
  const closeContextMenu = () => {
    if (contextMenu) contextMenu.hidden = true;
  };
  const csvEscape = value => {
    const text = value === undefined || value === null ? '' : String(value);
    return /[",\n\r]/.test(text) ? '"' + text.replace(/"/g, '""') + '"' : text;
  };
  const rowsToCSV = (rows, columns) => {
    const header = columns.map(id => csvEscape(labelForColumn(id))).join(',');
    const body = rows.map(row => columns.map(id => csvEscape(row[id])).join(','));
    return [header].concat(body).join('\n');
  };
  const rowsToJSONL = (rows, columns) => rows.map(row => JSON.stringify(projectRowForColumns(row, columns))).join('\n');
  const downloadText = (filename, mimeType, text) => {
    const blob = new Blob([text], {type: mimeType});
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  };
  const selectedRowsData = () => state.table ? state.table.getSelectedRows().map(row => row.getData()) : [];
  const showContextMenu = (event, items) => {
    if (!contextMenu) return;
    contextMenu.innerHTML = '';
    items.forEach(item => {
      const button = document.createElement('button');
      button.type = 'button';
      button.textContent = item.label;
      button.addEventListener('click', () => {
        closeContextMenu();
        item.action();
      });
      contextMenu.appendChild(button);
    });
    contextMenu.hidden = false;
    const width = contextMenu.offsetWidth;
    const height = contextMenu.offsetHeight;
    contextMenu.style.left = Math.min(event.clientX, window.innerWidth - width - 8) + 'px';
    contextMenu.style.top = Math.min(event.clientY, window.innerHeight - height - 8) + 'px';
  };
  closeDetailButtons.forEach(button => button.addEventListener('click', closeDetail));
  copyDetailButton?.addEventListener('click', () => {
    if (activeDetailRow) copyText(detailText(activeDetailRow, allExportableColumns())).catch(err => setStatus(err.message));
  });
  detail?.addEventListener('click', event => {
    if (event.target === detail) closeDetail();
  });
  const reloadTable = () => {
    if (state.table) {
      state.table.setPage(1);
    }
  };
  window.aiwpReloadRequestLogTable = reloadTable;
  const syncLogDateRange = changed => {
    if (!startInput || !endInput) return;
    const startKey = startInput.dataset.value || '';
    const endKey = endInput.dataset.value || '';
    if (startKey && endKey && compareDateKeys(startKey, endKey) > 0) {
      if (changed === 'start') {
        setLogDateInput(endInput, startKey);
      } else {
        setLogDateInput(startInput, endKey);
      }
    }
  };
  if (startInput) {
    buildLogDatePicker(startInput, () => {
      syncLogDateRange('start');
      reloadTable();
    });
  }
  if (endInput) {
    buildLogDatePicker(endInput, () => {
      syncLogDateRange('end');
      reloadTable();
    });
  }
  document.addEventListener('click', event => {
    if (activeLogDatePicker && !event.target.closest('.log-date-field')) {
      activeLogDatePicker.hidden = true;
      activeLogDatePicker = null;
    }
    if (contextMenu && !event.target.closest('#logRowContextMenu')) {
      closeContextMenu();
    }
  });
  document.addEventListener('keydown', event => {
    if (event.key === 'Escape' && activeLogDatePicker) {
      activeLogDatePicker.hidden = true;
      activeLogDatePicker = null;
    }
    if (event.key === 'Escape') {
      closeContextMenu();
      closeDetail();
    }
  });
  Promise.all([
    fetchJSON('/api/logs/request/columns'),
    fetchJSON('/api/logs/request/view-settings'),
  ]).then(([catalog, view]) => {
    state.columns = catalog.columns || [];
    state.columns.forEach(col => { state.columnsById[col.id] = col; });
    state.logTypes = catalog.log_types || [];
    defaultVisibleColumns = (catalog.default_columns || []).filter(id => state.columnsById[id]);
    state.visible = (view.visible_columns || defaultVisibleColumns).filter(id => state.columnsById[id]);
    state.widths = view.column_widths || {};
    state.pageSize = Number(view.page_size || 100);
    if (pageSizeSelect) pageSizeSelect.value = String(state.pageSize);
    renderLogTypeFilters();
    renderColumnPicker();
    renderRegexFilters();
    syncLogTableScrollWidth();
    state.table = new Tabulator(tableEl, {
      height: '520px',
      layout: 'fitDataTable',
      movableColumns: true,
      resizableColumnFit: false,
      resizableColumnGuide: false,
      selectableRows: true,
      selectableRowsPersistence: true,
      selectableRowsRangeMode: 'click',
      columnDefaults: {
        resizable: true,
      },
      pagination: true,
      paginationMode: 'remote',
      paginationSize: state.pageSize,
      paginationCounter: 'rows',
      ajaxURL: '/api/logs/request',
      ajaxURLGenerator: function(url, config, params){
        return url + '?' + queryParams(params.page || 1).toString();
      },
      ajaxResponse: function(url, params, response){
        setStatus(formatMatchingLogEntries(response.total || 0));
        return response;
      },
      columns: tabulatorColumns(),
      placeholder: 'No activity logs match the current filters.',
    });
    state.table.on('rowDblClick', function(e, row){ showDetail(row.getData()); });
    state.table.on('rowContext', function(e, row){
      e.preventDefault();
      const selectedRows = state.table.getSelectedRows();
      const clickedIsSelected = selectedRows.some(selected => selected === row);
      if (selectedRows.length > 1 && clickedIsSelected) {
        const rows = selectedRowsData();
        showContextMenu(e, [
          {label: 'Export in CSV (view only)', action: () => downloadText('selected-activity-logs-view.csv', 'text/csv;charset=utf-8', rowsToCSV(rows, selectedColumns()))},
          {label: 'Export in CSV (all details)', action: () => downloadText('selected-activity-logs-full.csv', 'text/csv;charset=utf-8', rowsToCSV(rows, allExportableColumns()))},
          {label: 'Export in JSONL (view only)', action: () => downloadText('selected-activity-logs-view.jsonl', 'application/x-ndjson;charset=utf-8', rowsToJSONL(rows, selectedColumns()))},
          {label: 'Export in JSONL (all details)', action: () => downloadText('selected-activity-logs-full.jsonl', 'application/x-ndjson;charset=utf-8', rowsToJSONL(rows, allExportableColumns()))},
        ]);
        return;
      }
      state.table.deselectRow();
      row.select();
      const data = row.getData();
      showContextMenu(e, [
        {label: 'Open entry details', action: () => showDetail(data)},
        {label: 'Copy details (view only)', action: () => copyText(detailText(data, selectedColumns())).catch(err => setStatus(err.message))},
        {label: 'Copy details (full)', action: () => copyText(detailText(data, allExportableColumns())).catch(err => setStatus(err.message))},
      ]);
    });
    state.table.on('columnMoved', function(){
      state.visible = state.table.getColumns().map(col => col.getField()).filter(Boolean);
      syncLogTableScrollWidth();
      saveViewSettings().catch(err => setStatus(err.message));
    });
    state.table.on('columnResized', function(column){
      const field = column.getField();
      if (field) {
        state.widths[field] = Math.round(column.getWidth());
        syncLogTableScrollWidth();
        saveViewSettings().catch(err => setStatus(err.message));
      }
    });
    state.table.on('dataLoaded', function(){
      window.requestAnimationFrame(syncLogTableScrollWidth);
    });
    if (window.ResizeObserver) {
      const observer = new ResizeObserver(redrawLogTable);
      if (tableEl.parentElement) observer.observe(tableEl.parentElement);
    }
    window.addEventListener('resize', redrawLogTable);
  }).catch(err => setStatus(err.message));
  document.getElementById('toggleLogColumns')?.addEventListener('click', () => {
    columnPanel.hidden = !columnPanel.hidden;
  });
  document.getElementById('toggleLogFilters')?.addEventListener('click', () => {
    filterPanel.hidden = !filterPanel.hidden;
  });
  document.getElementById('applyLogColumns')?.addEventListener('click', () => {
    const checked = selectedColumnPickerValues();
    if (!checked.length) {
      setStatus('Select at least one column.');
      return;
    }
    const next = selectedColumnsInPendingOrder(checked);
    state.visible = next;
    pendingColumnOrder = null;
    renderRegexFilters();
    syncLogTableScrollWidth();
    state.table.setColumns(tabulatorColumns());
    saveViewSettings().then(() => {
      columnPanel.hidden = true;
      reloadTable();
    }).catch(err => setStatus(err.message));
  });
  document.getElementById('resetLogColumnSelection')?.addEventListener('click', () => {
    setColumnPickerSelection(defaultVisibleColumns);
    pendingColumnOrder = defaultVisibleColumns.slice();
  });
  document.getElementById('resetLogColumnOrder')?.addEventListener('click', () => {
    window.alert('Column order will be reset once the changes are applied.');
    pendingColumnOrder = defaultAwareColumnOrder(selectedColumnPickerValues());
  });
  document.getElementById('applyLogFilters')?.addEventListener('click', reloadTable);
  document.getElementById('resetLogFilters')?.addEventListener('click', () => {
    filterGrid.querySelectorAll('input').forEach(input => { input.value = ''; });
    reloadTable();
  });
  pageSizeSelect?.addEventListener('change', () => {
    state.pageSize = Number(pageSizeSelect.value || 100);
    if (state.table) state.table.setPageSize(state.pageSize);
    saveViewSettings().then(reloadTable).catch(err => setStatus(err.message));
  });
  const exportLogs = (format, scope) => {
    const params = scope === 'all' ? new URLSearchParams() : queryParams(1);
    if (scope === 'all') {
      params.set('start_date', startInput ? startInput.dataset.value || '' : '');
      params.set('end_date', endInput ? endInput.dataset.value || '' : '');
      const agentID = new URLSearchParams(window.location.search).get('agent_id') || '';
      if (agentID) params.set('agent_id', agentID);
    }
    params.set('format', format);
    params.set('scope', scope);
    window.location = '/api/logs/request/export?' + params.toString();
  };
  document.getElementById('exportCurrentJSONL')?.addEventListener('click', () => exportLogs('jsonl', 'current'));
  document.getElementById('exportCurrentCSV')?.addEventListener('click', () => exportLogs('csv', 'current'));
  document.getElementById('exportAllJSONL')?.addEventListener('click', () => exportLogs('jsonl', 'all'));
  document.getElementById('exportAllCSV')?.addEventListener('click', () => exportLogs('csv', 'all'));
}

function htmlToElement(html){
  const template = document.createElement('template');
  template.innerHTML = String(html || '').trim();
  return template.content.firstElementChild;
}
function sidebarNavigationSignature(sidebar){
  if (!sidebar) return '';
  return [...sidebar.querySelectorAll('a.workspace-link,button.sidebar-connect')].map(el => {
    return [el.tagName, el.getAttribute('href') || '', el.id || '', el.textContent.trim()].join('|');
  }).join('\n');
}
function syncSidebarFromPartial(nextSidebar){
  const sidebar = document.querySelector('.sidebar');
  if (!sidebar || !nextSidebar) return;
  if (sidebarNavigationSignature(sidebar) !== sidebarNavigationSignature(nextSidebar)) {
    sidebar.replaceWith(nextSidebar);
    initSidebarScrollContain();
    initWorkspaceDrag();
    return;
  }
  const nextLinks = [...nextSidebar.querySelectorAll('a.workspace-link')];
  const links = [...sidebar.querySelectorAll('a.workspace-link')];
  links.forEach((link, index) => {
    const next = nextLinks[index];
    if (!next) return;
    link.className = next.className;
    link.setAttribute('aria-current', next.getAttribute('aria-current') || '');
    if (!next.hasAttribute('aria-current')) link.removeAttribute('aria-current');
    const badge = link.querySelector('.agent-skill-stale-badge');
    const nextBadge = next.querySelector('.agent-skill-stale-badge');
    if (badge && nextBadge) badge.hidden = nextBadge.hidden;
  });
}
function shouldHandlePartialNavigation(link, event){
  if (!link || event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return false;
  if (link.target || link.hasAttribute('download') || link.dataset.noPartialNavigation === 'true') return false;
  const url = new URL(link.href, window.location.href);
  if (url.origin !== window.location.origin) return false;
  if (url.hash && url.pathname === window.location.pathname && url.search === window.location.search) return false;
  if (url.pathname.startsWith('/auth/') || url.pathname.startsWith('/api/') || url.pathname.startsWith('/static/')) return false;
  return link.matches('.workspace-link, .topbar-brand-link');
}
async function loadPartialNavigation(url, pushState){
  const response = await fetch(url, {
    method: 'GET',
    credentials: 'same-origin',
    headers: {
      'Accept': 'application/json',
      'X-AIWP-Partial-Navigation': '1',
    },
  });
  const contentType = response.headers.get('Content-Type') || '';
  if (!response.ok || !contentType.includes('application/json')) {
    window.location.href = url;
    return;
  }
  const data = await response.json();
  const crumb = htmlToElement(data.topbar_crumb);
  const pageHeader = htmlToElement(data.page_header);
  const pageContent = htmlToElement(data.page_content);
  const nextSidebar = htmlToElement(data.sidebar);
  if (!crumb || !pageHeader || !pageContent) {
    window.location.href = url;
    return;
  }
  document.querySelector('.topbar-crumb')?.replaceWith(crumb);
  document.querySelector('.main-page-header')?.replaceWith(pageHeader);
  document.querySelector('.main-page-content')?.replaceWith(pageContent);
  syncSidebarFromPartial(nextSidebar);
  document.body.classList.toggle('admin-mode', Boolean(data.admin_mode));
  if (data.active_section) document.body.dataset.activeSection = data.active_section;
  if (data.page_title) document.title = data.page_title + ' - ' + appNameValue();
  if (pushState) history.pushState({aiwpPartial: true}, '', url);
  initPageBehaviors();
}
function navigateWithinApp(url){
  loadPartialNavigation(url, true).catch(() => {
    window.location.href = url;
  });
}
function initPartialNavigation(){
  if (document.body.dataset.aiwpPartialNavigationInitialized === 'true') return;
  document.body.dataset.aiwpPartialNavigationInitialized = 'true';
  document.addEventListener('click', event => {
    const link = event.target.closest('a');
    if (!shouldHandlePartialNavigation(link, event)) return;
    event.preventDefault();
    navigateWithinApp(link.href);
  });
  window.addEventListener('popstate', () => {
    loadPartialNavigation(window.location.href, false).catch(() => {
      window.location.reload();
    });
  });
}
function initPageBehaviors(){
  initAdminUsersPage();
  initAutoSizedSelects();
  initTrackedForms();
  initRequiredForms();
  initSavedCurrentTimeClock();
  initSessionTimeLeftClock();
  initAjaxForms();
  syncPolicyReviewControls();
  document.querySelectorAll('[data-policy-capability="true"]').forEach(input => {
    input.addEventListener('change', syncPolicyReviewControls);
  });
  syncAgentSkillDownloadButton();
  initRequestLogConsole();
}
document.addEventListener('DOMContentLoaded', () => {
  initSidebarScrollContain();
  initWorkspaceDrag();
  initPartialNavigation();
  initPageBehaviors();
});
