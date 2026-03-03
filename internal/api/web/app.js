// ui-disk-serve-enabled
const state = {
  projects: [],
  selectedProjectId: null,
  selectedProject: null,
  selectedPathCheckId: null,
  selectedPathTarget: "",
  activeView: "dashboardView",
  activeWindow: "1h",
  refreshInFlight: false,
  pendingRefresh: false,
  pollTimer: null,
  patternTarget: null,
  uptimeHoverIndex: null,
  uptimeRenderToken: 0,
  uptimeResizeObserver: null,
  data: {
    checks: [],
    logs: [],
    incidents: [],
    runs: [],
    fixes: [],
    uptime: null,
    pathHealth: [],
    pathRuns: [],
    smtpProfiles: [],
  },
};

const LAST_VIEW_COOKIE = "kraken_last_view";
const VIEW_COOKIE_DAYS = 180;

const errorTemplates = [
  { label: "Connection Refused", value: "connection refused|dial tcp" },
  {
    label: "HTTP 5xx",
    value: "status code 5[0-9]{2}|expected status .* got 5[0-9]{2}",
  },
  { label: "Timeout", value: "timeout|context deadline exceeded|i/o timeout" },
  {
    label: "DNS Error",
    value:
      "no such host|server misbehaving|temporary failure in name resolution",
  },
  { label: "TLS/Cert", value: "tls|certificate|x509|handshake" },
  { label: "Connection Reset", value: "connection reset|broken pipe|EOF" },
];

const el = {
  navBtns: Array.from(document.querySelectorAll(".nav-btn")),
  views: Array.from(document.querySelectorAll(".view")),
  projectSelect: document.getElementById("projectSelect"),
  openCreateBtn: document.getElementById("openCreateBtn"),
  closeCreateBtn: document.getElementById("closeCreateBtn"),
  createPanel: document.getElementById("createPanel"),
  createProjectForm: document.getElementById("createProjectForm"),
  refreshBtn: document.getElementById("refreshBtn"),
  runNowBtn: document.getElementById("runNowBtn"),
  toggleAutofixBtn: document.getElementById("toggleAutofixBtn"),
  deleteProjectBtn: document.getElementById("deleteProjectBtn"),
  projectTitle: document.getElementById("projectTitle"),
  projectSubTitle: document.getElementById("projectSubTitle"),
  statusBadge: document.getElementById("statusBadge"),
  metricIncidents: document.getElementById("metricIncidents"),
  metricChecks: document.getElementById("metricChecks"),
  metricAutofix: document.getElementById("metricAutofix"),
  metricLastCheck: document.getElementById("metricLastCheck"),
  metricFixes: document.getElementById("metricFixes"),
  metricFixAutofix: document.getElementById("metricFixAutofix"),
  checksList: document.getElementById("checksList"),
  incidentsList: document.getElementById("incidentsList"),
  fixesList: document.getElementById("fixesList"),
  pathsHealthList: document.getElementById("pathsHealthList"),
  pathLogsTitle: document.getElementById("pathLogsTitle"),
  pathLogsList: document.getElementById("pathLogsList"),
  fixForm: document.getElementById("fixForm"),
  fixUploadForm: document.getElementById("fixUploadForm"),
  fixPattern: document.getElementById("fixPattern"),
  uploadFixPattern: document.getElementById("uploadFixPattern"),
  templateList:
    document.getElementById("templateList") ||
    document.getElementById("fixTemplateList"),
  uptimeRecent: document.getElementById("uptimeRecent"),
  uptimeTotal: document.getElementById("uptimeTotal"),
  healthyRuns: document.getElementById("healthyRuns"),
  failedRuns: document.getElementById("failedRuns"),
  uptimeCanvasWrap: document.getElementById("uptimeCanvasWrap"),
  uptimeCanvas: document.getElementById("uptimeCanvas"),
  uptimeTooltip: document.getElementById("uptimeTooltip"),
  runsList: document.getElementById("runsList"),
  logsList: document.getElementById("logsList"),
  liveDot: document.getElementById("liveDot"),
  liveText: document.getElementById("liveText"),
  lastUpdatedText: document.getElementById("lastUpdatedText"),
  globalError: document.getElementById("globalError"),
  toast: document.getElementById("toast"),
  rangeBtns: Array.from(document.querySelectorAll(".range-btn")),
  dashboardView: document.getElementById("dashboardView"),
  fixesView: document.getElementById("fixesView"),
  uptimeView: document.getElementById("uptimeView"),
  settingsForm: document.getElementById("settingsForm"),
  settingsName: document.getElementById("settingsName"),
  settingsDomain: document.getElementById("settingsDomain"),
  settingsInterval: document.getElementById("settingsInterval"),
  settingsThreshold: document.getElementById("settingsThreshold"),
  settingsSMTP: document.getElementById("settingsSMTP"),
  smtpProfilesList: document.getElementById("smtpProfilesList"),
  smtpHost: document.getElementById("smtpHost"),
  smtpPort: document.getElementById("smtpPort"),
  smtpUser: document.getElementById("smtpUser"),
  smtpPass: document.getElementById("smtpPass"),
  smtpFrom: document.getElementById("smtpFrom"),
  createSMTPBtn: document.getElementById("createSMTPBtn"),
  settingsAutofix: document.getElementById("settingsAutofix"),
  settingsMaxRetries: document.getElementById("settingsMaxRetries"),
  settingsEmails: document.getElementById("settingsEmails"),
  settingsChecksRows: document.getElementById("settingsChecksRows"),
  addSettingsCheckBtn: document.getElementById("addSettingsCheckBtn"),
  settingsModal: document.getElementById("settingsModal"),
  openSettingsBtn: document.getElementById("openSettingsBtn"),
  closeSettingsX: document.getElementById("closeSettingsX"),
  settingsDiscardBtn: document.getElementById("settingsDiscardBtn"),
  settingsSaveBtn: document.getElementById("settingsSaveBtn"),
  sidebar: document.getElementById("sidebar"),
  closeSidebarBtn: document.getElementById("closeSidebarBtn"),
  openSidebarBtn: document.getElementById("openSidebarBtn"),
  editFixModal: document.getElementById("editFixModal"),
  editFixForm: document.getElementById("editFixForm"),
  editFixId: document.getElementById("editFixId"),
  editFixName: document.getElementById("editFixName"),
  editFixType: document.getElementById("editFixType"),
  editFixScriptPath: document.getElementById("editFixScriptPath"),
  editFixTimeout: document.getElementById("editFixTimeout"),
  editFixPattern: document.getElementById("editFixPattern"),
  closeEditFixX: document.getElementById("closeEditFixX"),
  editFixDiscardBtn: document.getElementById("editFixDiscardBtn"),
  editFixSaveBtn: document.getElementById("editFixSaveBtn"),
  uptimeFixesList: document.getElementById("uptimeFixesList"),
};

function setCookie(name, value, days) {
  if (typeof document === "undefined") return;
  const maxAge = Math.max(60, Math.floor(days * 24 * 60 * 60));
  document.cookie = `${encodeURIComponent(name)}=${encodeURIComponent(value)}; Max-Age=${maxAge}; Path=/; SameSite=Lax`;
}

function getCookie(name) {
  if (typeof document === "undefined" || !document.cookie) return "";
  const key = `${encodeURIComponent(name)}=`;
  const parts = document.cookie.split(";");
  for (const part of parts) {
    const trimmed = part.trim();
    if (trimmed.startsWith(key)) {
      return decodeURIComponent(trimmed.slice(key.length));
    }
  }
  return "";
}

function getPersistedView() {
  const view = getCookie(LAST_VIEW_COOKIE);
  if (!view) return "";
  return el.views.some((v) => v.id === view) ? view : "";
}

async function api(path, opts = {}) {
  const headers = { ...(opts.headers || {}) };
  if (!(opts.body instanceof FormData) && !headers["content-type"]) {
    headers["content-type"] = "application/json";
  }

  const res = await fetch(path, { ...opts, headers });
  const raw = await res.text();
  let parsed = null;
  if (raw) {
    try {
      parsed = JSON.parse(raw);
    } catch (_) {
      parsed = null;
    }
  }

  if (!res.ok) {
    const msg = parsed?.error || `request failed (${res.status})`;
    throw new Error(msg);
  }
  return parsed;
}

function showToast(message, type = "ok") {
  el.toast.textContent = message;
  el.toast.classList.remove("hidden");
  if (type === "error") {
    el.toast.style.background = "rgba(56, 18, 32, 0.95)";
    el.toast.style.borderColor = "rgba(232, 80, 106, 0.45)";
  } else {
    el.toast.style.background = "rgba(14, 19, 36, 0.96)";
    el.toast.style.borderColor = "rgba(90, 120, 224, 0.3)";
  }
  setTimeout(() => el.toast.classList.add("hidden"), 2600);
}

function setBanner(message) {
  if (!message) {
    el.globalError.classList.add("hidden");
    el.globalError.textContent = "";
    return;
  }
  el.globalError.textContent = message;
  el.globalError.classList.remove("hidden");
}

function setLiveState(mode) {
  el.liveDot.classList.remove("syncing", "error");
  if (mode === "syncing") {
    el.liveDot.classList.add("syncing");
    el.liveText.textContent = "Syncing data";
    return;
  }
  if (mode === "error") {
    el.liveDot.classList.add("error");
    el.liveText.textContent = "Live updates degraded";
    return;
  }
  el.liveText.textContent = "Live updates on";
}

function fmt(ts) {
  if (!ts) return "-";
  return new Date(ts).toLocaleString();
}

function escapeHtml(v) {
  return String(v)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

function clampText(value, max = 220) {
  const s = String(value || "");
  return s.length > max ? `${s.slice(0, max)}...` : s;
}

function setStatusBadge(kind, label) {
  el.statusBadge.className = `status-badge ${kind}`;
  el.statusBadge.textContent = label;
}

function pickSelectedProject() {
  if (
    state.selectedProjectId &&
    !state.projects.some((p) => p.id === state.selectedProjectId)
  ) {
    state.selectedProjectId = null;
  }
  if (!state.selectedProjectId && state.projects.length > 0) {
    state.selectedProjectId = state.projects[0].id;
  }
  state.selectedProject =
    state.projects.find((p) => p.id === state.selectedProjectId) || null;
}

function renderProjectSelect() {
  if (state.projects.length === 0) {
    el.projectSelect.innerHTML = `<option value="">No projects</option>`;
    el.projectSelect.disabled = true;
    return;
  }

  el.projectSelect.disabled = false;
  el.projectSelect.innerHTML = state.projects
    .map(
      (p) =>
        `<option value="${p.id}" ${p.id === state.selectedProjectId ? "selected" : ""}>${escapeHtml(p.name)} (${escapeHtml(p.domain)})</option>`,
    )
    .join("");
}

function renderTemplateButtons() {
  if (!el.templateList) return;
  el.templateList.innerHTML = errorTemplates
    .map(
      (tpl, idx) =>
        `<button class="template-btn" data-template-index="${idx}">${escapeHtml(tpl.label)}</button>`,
    )
    .join("");
}

function setActionButtons(enabled) {
  if (el.runNowBtn) el.runNowBtn.disabled = !enabled;
  if (el.toggleAutofixBtn) el.toggleAutofixBtn.disabled = !enabled;
  if (el.deleteProjectBtn) el.deleteProjectBtn.disabled = !enabled;
  if (el.addSettingsCheckBtn) el.addSettingsCheckBtn.disabled = !enabled;
}

function hideUptimeTooltip() {
  if (!el.uptimeTooltip) return;
  el.uptimeTooltip.classList.add("hidden");
}

function scheduleUptimeRender() {
  state.uptimeRenderToken += 1;
  const token = state.uptimeRenderToken;

  const attemptRender = (triesLeft) => {
    if (token !== state.uptimeRenderToken) return;
    const canvas = el.uptimeCanvas;
    if (!canvas) return;

    const rect = canvas.getBoundingClientRect();
    if ((rect.width < 40 || rect.height < 100) && triesLeft > 0) {
      requestAnimationFrame(() => attemptRender(triesLeft - 1));
      return;
    }
    renderUptimeCanvas();
  };

  requestAnimationFrame(() => requestAnimationFrame(() => attemptRender(10)));
}

function pickUptimeHoverIndex(clientX) {
  const canvas = el.uptimeCanvas;
  const points = state.data.uptime?.points || [];
  if (!canvas || points.length === 0) {
    return null;
  }

  const rect = canvas.getBoundingClientRect();
  if (rect.width < 40 || rect.height < 100) {
    return null;
  }

  const padX = 28;
  const chartW = rect.width - padX * 2;
  if (chartW <= 0) {
    return null;
  }

  const x = clientX - rect.left;
  const step = points.length > 1 ? chartW / (points.length - 1) : chartW;
  const raw = step > 0 ? Math.round((x - padX) / step) : 0;
  const idx = Math.max(0, Math.min(points.length - 1, raw));
  return Number.isFinite(idx) ? idx : null;
}

function syncSelectedPath(pathHealth) {
  if (!Array.isArray(pathHealth) || pathHealth.length === 0) {
    state.selectedPathCheckId = null;
    state.selectedPathTarget = "";
    return;
  }

  const match = pathHealth.find(
    (p) => p.check_id === state.selectedPathCheckId,
  );
  if (match) {
    state.selectedPathTarget = match.target;
    return;
  }
  state.selectedPathCheckId = pathHealth[0].check_id;
  state.selectedPathTarget = pathHealth[0].target;
}

function statusToneForPath(path) {
  if (!path) return "unknown";
  if (path.last_status === "failed") return "down";
  if (path.last_status === "healthy") return "up";
  return "unknown";
}

function renderPathHealthPanel() {
  if (!el.pathsHealthList || !el.pathLogsList || !el.pathLogsTitle) return;

  if (!state.selectedProject) {
    el.pathsHealthList.innerHTML = `<div class="list-item"><div class="main">No project selected</div></div>`;
    el.pathLogsTitle.textContent = "Select a path to view health logs.";
    el.pathLogsList.innerHTML = `<div class="list-item"><div class="main">No path logs</div></div>`;
    return;
  }

  const paths = state.data.pathHealth || [];
  if (!paths.length) {
    el.pathsHealthList.innerHTML = `<div class="list-item"><div class="main">No paths configured</div></div>`;
    el.pathLogsTitle.textContent = "Select a path to view health logs.";
    el.pathLogsList.innerHTML = `<div class="list-item"><div class="main">No path logs</div></div>`;
    return;
  }

  el.pathsHealthList.innerHTML = paths
    .map((p) => {
      const tone = statusToneForPath(p);
      const successPct = `${((p.success_rate_1h || 0) * 100).toFixed(1)}%`;
      const active = p.check_id === state.selectedPathCheckId ? "active" : "";
      return `
        <div class="list-item ${active}">
          <div class="main">
            <strong class="status-${tone}">${escapeHtml((p.last_status || "unknown").toUpperCase())}</strong>
            <span>${escapeHtml(p.target)}</span>
            <span class="meta">${escapeHtml((p.type || "").toUpperCase())} | 1h success ${successPct} | runs ${p.runs_1h ?? 0}</span>
            <span class="meta">last checked ${fmt(p.last_checked_at)}${p.last_error_message ? ` | ${escapeHtml(clampText(p.last_error_message, 90))}` : ""}</span>
          </div>
          <div class="inline-actions">
            <button class="btn secondary" data-path-check-id="${p.check_id}">View Logs</button>
          </div>
        </div>
      `;
    })
    .join("");

  const selectedPath = paths.find(
    (p) => p.check_id === state.selectedPathCheckId,
  );
  if (selectedPath) {
    el.pathLogsTitle.textContent = `Logs: ${selectedPath.target}`;
  } else {
    el.pathLogsTitle.textContent = "Select a path to view health logs.";
  }

  const runs = state.data.pathRuns || [];
  el.pathLogsList.innerHTML = runs.length
    ? runs
        .map(
          (run) => `
          <div class="list-item">
            <div class="main">
              <strong class="status-${run.status === "healthy" ? "ok" : "error"}">${escapeHtml(run.status.toUpperCase())}</strong>
              <span>response ${run.response_time_ms ?? "-"}ms${run.error_message ? ` | ${escapeHtml(clampText(run.error_message, 120))}` : ""}</span>
              <span class="meta">${fmt(run.created_at)}</span>
            </div>
          </div>
        `,
        )
        .join("")
    : `<div class="list-item"><div class="main">No logs for selected path</div></div>`;
}

function selectView(viewId) {
  const safeViewId = el[viewId] ? viewId : "dashboardView";
  state.activeView = safeViewId;
  el.views.forEach((v) => v.classList.remove("active"));
  if (el[safeViewId]) {
    el[safeViewId].classList.add("active");
  }
  el.navBtns.forEach((btn) =>
    btn.classList.toggle("active", btn.dataset.view === safeViewId),
  );

  setCookie(LAST_VIEW_COOKIE, safeViewId, VIEW_COOKIE_DAYS);

  if (safeViewId === "uptimeView") {
    scheduleUptimeRender();
  } else {
    state.uptimeHoverIndex = null;
    hideUptimeTooltip();
  }
}

function openSettingsModal() {
  if (el.settingsModal) {
    el.settingsModal.classList.remove("hidden");
    loadProjectSettings().catch((err) => showToast(err.message, "error"));
  }
}

function closeSettingsModal() {
  if (el.settingsModal) {
    el.settingsModal.classList.add("hidden");
  }
}

function toggleSidebar(open) {
  if (!el.sidebar) return;
  if (open) {
    el.sidebar.classList.remove("closed");
    if (el.openSidebarBtn) el.openSidebarBtn.classList.add("hidden");
  } else {
    el.sidebar.classList.add("closed");
    if (el.openSidebarBtn) el.openSidebarBtn.classList.remove("hidden");
  }
}

function setRange(range) {
  state.activeWindow = range;
  if (el.rangeBtns) {
    el.rangeBtns.forEach((btn) =>
      btn.classList.toggle("active", btn.dataset.range === range),
    );
  }
}

async function loadProjects() {
  state.projects = (await api("/v1/projects")) || [];
  pickSelectedProject();
  renderProjectSelect();
  setActionButtons(Boolean(state.selectedProject));
}

function renderDashboard() {
  if (!state.selectedProject) {
    el.projectTitle.textContent = "None";
    el.projectSubTitle.textContent = "Create or select a project.";
    setStatusBadge("unknown", "Unknown");
    el.metricIncidents.textContent = "0";
    el.metricChecks.textContent = "0";
    el.metricAutofix.textContent = "Off";
    el.metricLastCheck.textContent = "-";
    if (el.metricFixes) el.metricFixes.textContent = "0";
    if (el.metricFixAutofix) el.metricFixAutofix.textContent = "Off";
    el.checksList.innerHTML = "";
    el.incidentsList.innerHTML = "";
    el.fixesList.innerHTML = "";
    state.selectedPathCheckId = null;
    state.data.pathHealth = [];
    state.data.pathRuns = [];
    renderPathHealthPanel();
    return;
  }

  const openIncidents = state.data.incidents.filter(
    (i) => i.status === "open",
  ).length;
  const latestRun = state.data.runs[0] || null;

  if (openIncidents > 0) setStatusBadge("down", "Down");
  else if (!latestRun) setStatusBadge("unknown", "No Data");
  else if (latestRun.status === "failed") setStatusBadge("warn", "Warning");
  else setStatusBadge("up", "Healthy");

  el.projectTitle.textContent = state.selectedProject.name;
  el.projectSubTitle.textContent = `${state.selectedProject.domain} | interval ${state.selectedProject.check_interval_sec}s | threshold ${state.selectedProject.failure_threshold}`;
  el.metricIncidents.textContent = String(openIncidents);
  el.metricChecks.textContent = String(state.data.checks.length);
  el.metricAutofix.textContent = state.selectedProject.autofix_enabled
    ? "On"
    : "Off";
  el.metricLastCheck.textContent = latestRun ? fmt(latestRun.created_at) : "-";
  if (el.metricFixes)
    el.metricFixes.textContent = String(state.data.fixes.length);
  if (el.metricFixAutofix)
    el.metricFixAutofix.textContent = state.selectedProject.autofix_enabled
      ? "On"
      : "Off";

  el.checksList.innerHTML = state.data.checks.length
    ? state.data.checks
        .map(
          (c) => `
          <div class="list-item">
            <div class="main">
              <strong>${escapeHtml(c.type.toUpperCase())}</strong>
              <span class="meta">${escapeHtml(c.target)}</span>
              <span class="meta">timeout ${c.timeout_ms}ms | expected ${c.expected_status ?? "2xx/3xx"}</span>
            </div>
          </div>`,
        )
        .join("")
    : `<div class="list-item"><div class="main">No checks configured</div></div>`;

  el.incidentsList.innerHTML = state.data.incidents.length
    ? state.data.incidents
        .map(
          (i) => `
          <div class="list-item">
            <div class="main">
              <strong class="status-${i.status === "open" ? "down" : "up"}">${escapeHtml(i.status.toUpperCase())}</strong>
              <span>${escapeHtml(clampText(i.error_message, 180))}</span>
              <span class="meta">started ${fmt(i.started_at)}${i.resolved_at ? ` | resolved ${fmt(i.resolved_at)}` : ""}</span>
            </div>
          </div>`,
        )
        .join("")
    : `<div class="list-item"><div class="main">No incidents</div></div>`;

  el.fixesList.innerHTML = state.data.fixes.length
    ? state.data.fixes
        .map(
          (f) => `
          <div class="list-item">
            <div class="main">
              <strong>${escapeHtml(f.name)}</strong>
              <span class="meta">${escapeHtml(f.type)} | ${escapeHtml(f.script_path)} | timeout ${f.timeout_sec}s</span>
              <span class="meta">pattern: ${escapeHtml(clampText(f.supported_error_pattern, 120))}</span>
            </div>
            <div class="inline-actions">
              <button class="btn secondary" data-edit-fix-id="${f.id}">Edit</button>
              <button class="btn danger" data-delete-fix-id="${f.id}" data-fix-name="${escapeHtml(f.name)}">Delete</button>
              <button class="btn secondary" data-run-fix-id="${f.id}">Run</button>
            </div>
          </div>`,
        )
        .join("")
    : `<div class="list-item"><div class="main">No fixes attached</div></div>`;

  renderPathHealthPanel();
}

function renderUptimeCanvas() {
  const canvas = el.uptimeCanvas;
  if (!canvas) return;

  const points = state.data.uptime?.points || [];
  const rect = canvas.getBoundingClientRect();
  if (rect.width < 40 || rect.height < 100) {
    hideUptimeTooltip();
    return;
  }

  const dpr = window.devicePixelRatio || 1;
  const width = Math.max(300, Math.floor(rect.width * dpr));
  const height = Math.max(180, Math.floor(rect.height * dpr));
  if (canvas.width !== width || canvas.height !== height) {
    canvas.width = width;
    canvas.height = height;
  }

  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  ctx.clearRect(0, 0, width, height);

  const bg = ctx.createLinearGradient(0, 0, 0, height);
  bg.addColorStop(0, "rgba(12, 18, 36, 0.98)");
  bg.addColorStop(1, "rgba(8, 12, 24, 0.96)");
  ctx.fillStyle = bg;
  ctx.fillRect(0, 0, width, height);

  const padX = 28 * dpr;
  const padY = 16 * dpr;
  const chartW = width - padX * 2;
  const chartH = height - padY * 2;

  ctx.strokeStyle = "rgba(137,159,246,0.18)";
  ctx.lineWidth = 1;
  for (let i = 0; i <= 4; i++) {
    const y = padY + (chartH * i) / 4;
    ctx.beginPath();
    ctx.moveTo(padX, y);
    ctx.lineTo(padX + chartW, y);
    ctx.stroke();
  }

  if (points.length === 0) {
    hideUptimeTooltip();
    ctx.fillStyle = "rgba(156,168,214,0.9)";
    ctx.font = `${12 * dpr}px JetBrains Mono`;
    ctx.fillText("No uptime data yet", padX, height / 2);
    return;
  }

  const step = points.length > 1 ? chartW / (points.length - 1) : chartW;
  const hasAnyData = points.some(
    (p) => (p.up_seconds || 0) + (p.down_seconds || 0) > 0,
  );
  if (!hasAnyData) {
    hideUptimeTooltip();
    ctx.fillStyle = "rgba(156,168,214,0.9)";
    ctx.font = `${12 * dpr}px JetBrains Mono`;
    ctx.fillText("No recent data", padX, height / 2);
    return;
  }

  const drawRoundedRect = (x, y, w, h, r) => {
    const radius = Math.max(0, Math.min(r, w / 2, h / 2));
    ctx.beginPath();
    ctx.moveTo(x + radius, y);
    ctx.lineTo(x + w - radius, y);
    ctx.quadraticCurveTo(x + w, y, x + w, y + radius);
    ctx.lineTo(x + w, y + h - radius);
    ctx.quadraticCurveTo(x + w, y + h, x + w - radius, y + h);
    ctx.lineTo(x + radius, y + h);
    ctx.quadraticCurveTo(x, y + h, x, y + h - radius);
    ctx.lineTo(x, y + radius);
    ctx.quadraticCurveTo(x, y, x + radius, y);
    ctx.closePath();
  };

  const barW = Math.max(2 * dpr, Math.floor((chartW / points.length) * 0.92));
  let hoverDraw = null;
  points.forEach((p, idx) => {
    const known = (p.up_seconds || 0) + (p.down_seconds || 0);
    if (known <= 0) {
      return;
    }
    const ratio = Math.max(0, Math.min(1, p.uptime_ratio ?? 0));
    const x = padX + idx * step - barW / 2;
    const y = padY + (1 - ratio) * chartH;
    const h = chartH - (y - padY);

    let color = "rgba(32,232,138,0.82)";
    if (p.down_seconds > 0) color = "rgba(232,80,106,0.85)";

    const roundedX = Math.floor(x);
    const roundedY = Math.floor(y);
    const roundedW = Math.ceil(barW);
    const roundedH = Math.max(1, Math.ceil(h));
    const radius = Math.min(2.5 * dpr, roundedW / 2, roundedH / 2);

    drawRoundedRect(roundedX, roundedY, roundedW, roundedH, radius);
    ctx.fillStyle = color;
    ctx.fill();

    if (state.uptimeHoverIndex === idx) {
      hoverDraw = {
        idx,
        point: p,
        ratio,
        x,
        y,
      };
      ctx.strokeStyle = "rgba(221, 236, 255, 0.95)";
      ctx.lineWidth = Math.max(1, Math.floor(dpr));
      drawRoundedRect(roundedX, roundedY, roundedW, roundedH, radius);
      ctx.stroke();
    }
  });

  ctx.fillStyle = "rgba(156,168,214,0.9)";
  ctx.font = `${10 * dpr}px JetBrains Mono`;
  ctx.fillText("100%", 2 * dpr, padY + 10 * dpr);
  ctx.fillText("0%", 10 * dpr, padY + chartH);

  const first = points[0];
  const last = points[points.length - 1];
  if (first && last) {
    const firstLabel = new Date(first.start).toLocaleString();
    const lastLabel = new Date(last.end).toLocaleString();
    const labelY = height - 4 * dpr;
    ctx.fillStyle = "rgba(156,168,214,0.85)";
    ctx.font = `${10 * dpr}px JetBrains Mono`;
    ctx.fillText(firstLabel, padX, labelY);
    const lastWidth = ctx.measureText(lastLabel).width;
    ctx.fillText(lastLabel, padX + chartW - lastWidth, labelY);
  }

  if (!hoverDraw) {
    hideUptimeTooltip();
    return;
  }

  if (el.uptimeTooltip && el.uptimeCanvasWrap) {
    const p = hoverDraw.point;
    const known = (p.up_seconds || 0) + (p.down_seconds || 0);
    const status = p.down_seconds > 0 ? "DOWN" : "UP";
    const pct =
      known > 0 ? ((100 * (p.up_seconds || 0)) / known).toFixed(2) : "0.00";
    const startLabel = new Date(p.start).toLocaleString();
    const endLabel = new Date(p.end).toLocaleString();
    el.uptimeTooltip.innerHTML = `${status} ${pct}%<br>${startLabel}<br>${endLabel}<br>up ${p.up_seconds || 0}s | down ${p.down_seconds || 0}s`;
    el.uptimeTooltip.classList.remove("hidden");

    const padXCss = 28;
    const padYCss = 16;
    const chartWCss = rect.width - padXCss * 2;
    const chartHCss = rect.height - padYCss * 2;
    const stepCss =
      points.length > 1 ? chartWCss / (points.length - 1) : chartWCss;
    const xCss = padXCss + hoverDraw.idx * stepCss;
    const yCss = padYCss + (1 - hoverDraw.ratio) * chartHCss;

    const tipW = el.uptimeTooltip.offsetWidth || 200;
    const tipH = el.uptimeTooltip.offsetHeight || 70;
    const left = Math.max(8, Math.min(rect.width - tipW - 8, xCss + 12));
    const top = Math.max(8, Math.min(rect.height - tipH - 8, yCss - tipH - 12));
    el.uptimeTooltip.style.left = `${left}px`;
    el.uptimeTooltip.style.top = `${top}px`;
  }
}

function renderUptimePanel() {
  const uptime = state.data.uptime;
  if (!uptime || !uptime.points || uptime.points.length === 0) {
    el.uptimeRecent.textContent = "0%";
    el.uptimeTotal.textContent = "0%";
    el.healthyRuns.textContent = "0";
    el.failedRuns.textContent = "0";
    el.runsList.innerHTML = `<div class="list-item"><div class="main">No check runs yet</div></div>`;
    el.logsList.innerHTML = `<div class="list-item"><div class="main">No logs yet</div></div>`;
    if (state.activeView === "uptimeView") {
      scheduleUptimeRender();
    } else {
      state.uptimeHoverIndex = null;
      hideUptimeTooltip();
    }
    return;
  }

  let upSeconds = 0;
  let downSeconds = 0;
  let knownSeconds = 0;
  let totalSeconds = 0;

  uptime.points.forEach((p) => {
    const up = Number(p.up_seconds || 0);
    const down = Number(p.down_seconds || 0);
    const duration = Math.max(
      0,
      Math.round(
        (new Date(p.end).getTime() - new Date(p.start).getTime()) / 1000,
      ),
    );

    upSeconds += up;
    downSeconds += down;
    knownSeconds += up + down;
    totalSeconds += duration;
  });

  const uptimePct = knownSeconds > 0 ? (100 * upSeconds) / knownSeconds : 0;
  const coveragePctRaw =
    totalSeconds > 0 ? (100 * knownSeconds) / totalSeconds : 0;
  const coveragePct = Math.max(0, Math.min(100, coveragePctRaw));

  el.uptimeRecent.textContent = `${uptimePct.toFixed(2)}%`;
  el.uptimeTotal.textContent = `${coveragePct.toFixed(2)}%`;
  el.healthyRuns.textContent = String(upSeconds);
  el.failedRuns.textContent = String(downSeconds);

  el.runsList.innerHTML = state.data.runs.length
    ? state.data.runs
        .slice(0, 120)
        .map(
          (r) => `
          <div class="list-item">
            <div class="main">
              <strong class="status-${r.status === "healthy" ? "ok" : "error"}">${escapeHtml(r.status.toUpperCase())}</strong>
              <span>check ${r.check_id} | ${r.response_time_ms ?? "-"}ms${r.error_message ? ` | ${escapeHtml(clampText(r.error_message, 130))}` : ""}</span>
              <span class="meta">${fmt(r.created_at)}</span>
            </div>
          </div>`,
        )
        .join("")
    : `<div class="list-item"><div class="main">No check runs yet</div></div>`;

  el.logsList.innerHTML = state.data.logs.length
    ? state.data.logs
        .slice(0, 150)
        .map(
          (l) => `
          <div class="list-item">
            <div class="main">
              <strong class="status-${l.level === "error" ? "error" : l.level === "warn" ? "warn" : "ok"}">[${escapeHtml(l.level)}]</strong>
              <span>${escapeHtml(clampText(l.message, 190))}</span>
              <span class="meta">${fmt(l.timestamp)}</span>
            </div>
          </div>`,
        )
        .join("")
    : `<div class="list-item"><div class="main">No logs yet</div></div>`;

  if (state.activeView === "uptimeView") {
    scheduleUptimeRender();
  } else {
    state.uptimeHoverIndex = null;
    hideUptimeTooltip();
  }

  renderUptimeFixesWidget();
}

function updateProjectInState(project) {
  if (!project) return;
  const idx = state.projects.findIndex((p) => p.id === project.id);
  if (idx >= 0) state.projects[idx] = project;
  state.selectedProject = project;
  state.selectedProjectId = project.id;
}

function renderSettingsChecksRows(checks) {
  if (!el.settingsChecksRows) return;
  const rows = Array.isArray(checks) ? checks : [];
  el.settingsChecksRows.innerHTML = rows.length
    ? rows
        .map((check) => {
          const expected = check.expected_status ?? "";
          const selectedType = (type) =>
            check.type === type ? "selected" : "";
          return `
            <div class="settings-check-row" data-check-id="${check.id || ""}">
              <div class="cell">
                <span class="label">Type</span>
                <select data-check-field="type">
                  <option value="http" ${selectedType("http")}>http</option>
                  <option value="tcp" ${selectedType("tcp")}>tcp</option>
                  <option value="ping" ${selectedType("ping")}>ping</option>
                </select>
              </div>
              <div class="cell">
                <span class="label">Target</span>
                <input data-check-field="target" class="path-target" value="${escapeHtml(check.target || "")}" placeholder="http://localhost:3000/" />
              </div>
              <div class="cell">
                <span class="label">Timeout (ms)</span>
                <input data-check-field="timeout_ms" type="number" min="100" value="${check.timeout_ms || 5000}" />
              </div>
              <div class="cell">
                <span class="label">Expected</span>
                <input data-check-field="expected_status" type="number" min="100" max="599" value="${expected}" placeholder="200" />
              </div>
              <div class="cell">
                <span class="label">Action</span>
                <button type="button" class="btn danger" data-remove-check="1">Remove</button>
              </div>
            </div>
          `;
        })
        .join("")
    : "";
}

function addSettingsCheckRow(defaults = {}) {
  if (!el.settingsChecksRows) return;
  const selectedType = (type) =>
    (defaults.type || "http") === type ? "selected" : "";
  const row = document.createElement("div");
  row.className = "settings-check-row";
  row.dataset.checkId = defaults.id ? String(defaults.id) : "";
  row.innerHTML = `
    <div class="cell">
      <span class="label">Type</span>
      <select data-check-field="type">
        <option value="http" ${selectedType("http")}>http</option>
        <option value="tcp" ${selectedType("tcp")}>tcp</option>
        <option value="ping" ${selectedType("ping")}>ping</option>
      </select>
    </div>
    <div class="cell">
      <span class="label">Target</span>
      <input data-check-field="target" class="path-target" value="${escapeHtml(defaults.target || "")}" placeholder="http://localhost:3000/" />
    </div>
    <div class="cell">
      <span class="label">Timeout (ms)</span>
      <input data-check-field="timeout_ms" type="number" min="100" value="${defaults.timeout_ms || 5000}" />
    </div>
    <div class="cell">
      <span class="label">Expected</span>
      <input data-check-field="expected_status" type="number" min="100" max="599" value="${defaults.expected_status ?? ""}" placeholder="200" />
    </div>
    <div class="cell">
      <span class="label">Action</span>
      <button type="button" class="btn danger" data-remove-check="1">Remove</button>
    </div>
  `;
  el.settingsChecksRows.appendChild(row);
}

function renderSettingsForm() {
  if (!el.settingsForm) return;

  if (!state.selectedProject) {
    el.settingsForm.reset();
    if (el.settingsChecksRows) el.settingsChecksRows.innerHTML = "";
    if (el.settingsSMTP)
      el.settingsSMTP.innerHTML = `<option value="">No SMTP profiles</option>`;
    if (el.smtpProfilesList) {
      el.smtpProfilesList.innerHTML = `<div class="list-item"><div class="main">No SMTP profiles</div></div>`;
    }
    return;
  }

  el.settingsName.value = state.selectedProject.name || "";
  el.settingsDomain.value = state.selectedProject.domain || "";
  el.settingsInterval.value = String(
    state.selectedProject.check_interval_sec || 30,
  );
  el.settingsThreshold.value = String(
    state.selectedProject.failure_threshold || 3,
  );
  el.settingsAutofix.checked = Boolean(state.selectedProject.autofix_enabled);
  el.settingsMaxRetries.value = String(
    state.selectedProject.max_autofix_retries ?? 3,
  );
  el.settingsEmails.value = (state.selectedProject.alert_emails || []).join(
    ", ",
  );

  const smtpProfiles = state.data.smtpProfiles || [];
  const selectedSMTP = state.selectedProject.smtp_profile_id;
  const smtpOptions = [`<option value="">Use ENV default</option>`];
  smtpProfiles.forEach((profile) => {
    smtpOptions.push(
      `<option value="${profile.id}" ${selectedSMTP === profile.id ? "selected" : ""}>#${profile.id} ${escapeHtml(profile.host)}:${profile.port} (${escapeHtml(profile.from_email)})</option>`,
    );
  });
  el.settingsSMTP.innerHTML = smtpOptions.join("");
  if (el.smtpProfilesList) {
    el.smtpProfilesList.innerHTML = smtpProfiles.length
      ? smtpProfiles
          .map(
            (profile) => `
          <div class="list-item">
            <div class="main">
              <strong>#${profile.id} ${escapeHtml(profile.from_email)}</strong>
              <span class="meta">${escapeHtml(profile.host)}:${profile.port} | ${escapeHtml(profile.username)}</span>
            </div>
          </div>`,
          )
          .join("")
      : `<div class="list-item"><div class="main">No SMTP profiles saved</div></div>`;
  }

  renderSettingsChecksRows(state.data.checks || []);
}

async function createSMTPProfileFromSettings() {
  const host = (el.smtpHost?.value || "").trim();
  const port = Number((el.smtpPort?.value || "").trim());
  const username = (el.smtpUser?.value || "").trim();
  const password = (el.smtpPass?.value || "").trim();
  const fromEmail = (el.smtpFrom?.value || "").trim();

  if (!host || !port || !username || !password || !fromEmail) {
    showToast("SMTP host, port, user, pass and from email are required", "error");
    return;
  }

  try {
    const created = await api("/v1/smtp_profiles", {
      method: "POST",
      body: JSON.stringify({
        host,
        port,
        username,
        password,
        from_email: fromEmail,
      }),
    });

    if (el.smtpPass) el.smtpPass.value = "";
    if (el.smtpHost) el.smtpHost.value = "";
    if (el.smtpPort) el.smtpPort.value = "587";
    if (el.smtpUser) el.smtpUser.value = "";
    if (el.smtpFrom) el.smtpFrom.value = "";

    await loadProjectSettings();
    if (el.settingsSMTP && created?.id) {
      el.settingsSMTP.value = String(created.id);
    }
    showToast("SMTP profile created");
  } catch (err) {
    showToast(err.message, "error");
  }
}

function collectSettingsChecks() {
  if (!el.settingsChecksRows) return [];
  const rows = Array.from(
    el.settingsChecksRows.querySelectorAll(".settings-check-row"),
  );
  return rows
    .map((row) => {
      const checkIDRaw = row.dataset.checkId
        ? Number(row.dataset.checkId)
        : null;
      const type =
        row.querySelector('[data-check-field="type"]')?.value?.trim() || "http";
      const target =
        row.querySelector('[data-check-field="target"]')?.value?.trim() || "";
      const timeoutMs = Number(
        row.querySelector('[data-check-field="timeout_ms"]')?.value || "5000",
      );
      const expectedRaw =
        row
          .querySelector('[data-check-field="expected_status"]')
          ?.value?.trim() || "";
      const expected = expectedRaw ? Number(expectedRaw) : null;
      if (!target) {
        return null;
      }
      return {
        id: checkIDRaw && checkIDRaw > 0 ? checkIDRaw : undefined,
        type,
        target,
        timeout_ms:
          Number.isFinite(timeoutMs) && timeoutMs > 0 ? timeoutMs : 5000,
        expected_status:
          expected && Number.isFinite(expected) ? expected : null,
      };
    })
    .filter(Boolean);
}

async function loadProjectSettings() {
  if (!state.selectedProject) {
    renderSettingsForm();
    return;
  }
  const projectID = state.selectedProject.id;
  const settings = await api(`/v1/projects/${projectID}/settings`);
  if (!state.selectedProject || state.selectedProject.id !== projectID) return;

  updateProjectInState(settings.project);
  state.data.checks = settings.checks || [];
  state.data.smtpProfiles = settings.smtp_profiles || [];
  renderProjectSelect();
  renderDashboard();
  renderSettingsForm();
}

async function saveSettings(event) {
  if (event) event.preventDefault();
  if (!state.selectedProject) return;

  const checks = collectSettingsChecks();
  const smtpRaw = el.settingsSMTP.value.trim();
  const smtpProfileID = smtpRaw ? Number(smtpRaw) : null;
  const emails = (el.settingsEmails.value || "")
    .split(",")
    .map((v) => v.trim())
    .filter(Boolean);

  const payload = {
    name: el.settingsName.value.trim(),
    domain: el.settingsDomain.value.trim(),
    check_interval_sec: Number(el.settingsInterval.value),
    failure_threshold: Number(el.settingsThreshold.value),
    autofix_enabled: Boolean(el.settingsAutofix.checked),
    max_autofix_retries: Number(el.settingsMaxRetries.value),
    smtp_profile_id: smtpProfileID && smtpProfileID > 0 ? smtpProfileID : null,
    alert_emails: emails,
    checks,
  };

  if (!payload.name || !payload.domain) {
    showToast("Name and domain are required", "error");
    return;
  }

  try {
    const result = await api(
      `/v1/projects/${state.selectedProject.id}/settings`,
      {
        method: "PUT",
        body: JSON.stringify(payload),
      },
    );
    updateProjectInState(result.project);
    state.data.checks = result.checks || [];
    state.data.smtpProfiles = result.smtp_profiles || state.data.smtpProfiles;
    renderProjectSelect();
    renderDashboard();
    renderSettingsForm();
    await refreshSelectedProject();
    closeSettingsModal();
    showToast("Project settings updated");
  } catch (err) {
    showToast(err.message, "error");
  }
}

async function refreshSelectedPathRuns() {
  if (!state.selectedProject || !state.selectedPathCheckId) {
    state.data.pathRuns = [];
    renderPathHealthPanel();
    return;
  }

  const projectID = state.selectedProject.id;
  const checkID = state.selectedPathCheckId;
  try {
    const runs = await api(
      `/v1/projects/${projectID}/checks/${checkID}/runs?limit=180`,
    );
    if (
      !state.selectedProject ||
      state.selectedProject.id !== projectID ||
      state.selectedPathCheckId !== checkID
    ) {
      return;
    }
    state.data.pathRuns = runs || [];
    renderPathHealthPanel();
  } catch (err) {
    setBanner(`Path log refresh error: ${err.message}`);
  }
}

async function refreshSelectedProject() {
  if (!state.selectedProject) {
    renderDashboard();
    renderPathHealthPanel();
    renderUptimePanel();
    return;
  }

  if (state.refreshInFlight) {
    state.pendingRefresh = true;
    return;
  }
  state.refreshInFlight = true;
  setLiveState("syncing");

  const projectID = state.selectedProject.id;
  try {
    const [checks, logs, incidents, runs, fixes, uptime, pathHealth] =
      await Promise.all([
        api(`/v1/projects/${projectID}/checks`),
        api(`/v1/projects/${projectID}/logs?limit=220`),
        api(`/v1/projects/${projectID}/incidents?limit=80`),
        api(`/v1/projects/${projectID}/check-runs?limit=300`),
        api(`/v1/projects/${projectID}/fixes`),
        api(
          `/v1/projects/${projectID}/uptime?window=${encodeURIComponent(state.activeWindow)}`,
        ),
        api(`/v1/projects/${projectID}/paths/health`),
      ]);

    if (!state.selectedProject || state.selectedProject.id !== projectID) {
      return;
    }

    syncSelectedPath(pathHealth || []);

    let pathRuns = [];
    if (state.selectedPathCheckId) {
      pathRuns = await api(
        `/v1/projects/${projectID}/checks/${state.selectedPathCheckId}/runs?limit=180`,
      );
      if (!state.selectedProject || state.selectedProject.id !== projectID) {
        return;
      }
    }

    state.data = {
      checks: checks || [],
      logs: logs || [],
      incidents: incidents || [],
      runs: runs || [],
      fixes: fixes || [],
      uptime: uptime || null,
      pathHealth: pathHealth || [],
      pathRuns: pathRuns || [],
      smtpProfiles: state.data.smtpProfiles || [],
    };

    renderDashboard();
    renderPathHealthPanel();
    renderUptimePanel();
    el.lastUpdatedText.textContent = `Last updated: ${new Date().toLocaleTimeString()}`;
    setLiveState("ok");
    setBanner("");
  } catch (err) {
    setLiveState("error");
    setBanner(`Live refresh error: ${err.message}`);
  } finally {
    state.refreshInFlight = false;
    if (state.pendingRefresh) {
      state.pendingRefresh = false;
      refreshSelectedProject();
    }
  }
}

async function runChecksNow() {
  if (!state.selectedProject) return;
  try {
    const res = await api(`/v1/projects/${state.selectedProject.id}/run-now`, {
      method: "POST",
    });
    showToast(`Queued ${res.queued} checks`);
    setTimeout(refreshSelectedProject, 700);
  } catch (err) {
    showToast(err.message, "error");
  }
}

async function toggleAutofix() {
  if (!state.selectedProject) return;
  const next = !state.selectedProject.autofix_enabled;
  try {
    await api(`/v1/projects/${state.selectedProject.id}/autofix`, {
      method: "PATCH",
      body: JSON.stringify({ enabled: next }),
    });
    state.selectedProject.autofix_enabled = next;
    const inList = state.projects.find(
      (p) => p.id === state.selectedProject.id,
    );
    if (inList) inList.autofix_enabled = next;
    renderDashboard();
    showToast(`Autofix ${next ? "enabled" : "disabled"}`);
  } catch (err) {
    showToast(err.message, "error");
  }
}

async function deleteProject() {
  if (!state.selectedProject) return;
  const ok = window.confirm(`Delete project "${state.selectedProject.name}"?`);
  if (!ok) return;

  try {
    await api(`/v1/projects/${state.selectedProject.id}`, { method: "DELETE" });
    showToast("Project deleted");
    await loadProjects();
    await refreshSelectedProject();
  } catch (err) {
    showToast(err.message, "error");
  }
}

function normalizeTarget(domain, path) {
  if (path.startsWith("http://") || path.startsWith("https://")) return path;
  if (path.startsWith("/")) return `http://${domain}${path}`;
  return `http://${domain}/${path}`;
}

async function createProject(event) {
  event.preventDefault();
  const name = document.getElementById("createName").value.trim();
  const domain = document.getElementById("createDomain").value.trim();
  const interval = Number(document.getElementById("createInterval").value);
  const threshold = Number(document.getElementById("createThreshold").value);
  const autofixEnabled = document.getElementById("createAutofix").checked;
  const emails = document
    .getElementById("createEmails")
    .value.split(",")
    .map((v) => v.trim())
    .filter(Boolean);
  const paths = document
    .getElementById("createPaths")
    .value.split("\n")
    .map((v) => v.trim())
    .filter(Boolean);

  try {
    const project = await api("/v1/projects", {
      method: "POST",
      body: JSON.stringify({
        name,
        domain,
        check_interval_sec: interval,
        failure_threshold: threshold,
        autofix_enabled: autofixEnabled,
        alert_emails: emails,
      }),
    });

    for (const path of paths) {
      await api(`/v1/projects/${project.id}/checks`, {
        method: "POST",
        body: JSON.stringify({
          type: "http",
          target: normalizeTarget(domain, path),
          timeout_ms: 5000,
          expected_status: 200,
        }),
      });
    }

    el.createProjectForm.reset();
    document.getElementById("createInterval").value = "30";
    document.getElementById("createThreshold").value = "3";
    document.getElementById("createAutofix").checked = true;

    state.selectedProjectId = project.id;
    await loadProjects();
    await refreshSelectedProject();
    toggleCreatePanel(false);
    showToast("Project created");
  } catch (err) {
    showToast(err.message, "error");
  }
}

async function createFix(event) {
  event.preventDefault();
  if (!state.selectedProject) {
    showToast("Select a project first", "error");
    return;
  }

  const payload = {
    name: document.getElementById("fixName").value.trim(),
    type: document.getElementById("fixType").value,
    script_path: document.getElementById("fixScriptPath").value.trim(),
    timeout_sec: Number(document.getElementById("fixTimeout").value),
    supported_error_pattern: document.getElementById("fixPattern").value.trim(),
  };

  try {
    await api(`/v1/projects/${state.selectedProject.id}/fixes`, {
      method: "POST",
      body: JSON.stringify(payload),
    });
    el.fixForm.reset();
    document.getElementById("fixType").value = "http";
    document.getElementById("fixTimeout").value = "60";
    await refreshSelectedProject();
    showToast("Fix added");
  } catch (err) {
    showToast(err.message, "error");
  }
}

async function uploadFix(event) {
  event.preventDefault();
  if (!state.selectedProject) {
    showToast("Select a project first", "error");
    return;
  }

  const fileInput = document.getElementById("uploadFixFile");
  if (!fileInput.files || fileInput.files.length === 0) {
    showToast("Select a .sh file", "error");
    return;
  }

  const form = new FormData();
  form.append("name", document.getElementById("uploadFixName").value.trim());
  form.append("type", document.getElementById("uploadFixType").value);
  form.append("timeout_sec", document.getElementById("uploadFixTimeout").value);
  form.append(
    "supported_error_pattern",
    document.getElementById("uploadFixPattern").value.trim(),
  );
  form.append("file", fileInput.files[0]);

  try {
    await api(`/v1/projects/${state.selectedProject.id}/fixes/upload`, {
      method: "POST",
      body: form,
    });
    el.fixUploadForm.reset();
    document.getElementById("uploadFixType").value = "http";
    document.getElementById("uploadFixTimeout").value = "60";
    await refreshSelectedProject();
    showToast("Fix uploaded");
  } catch (err) {
    showToast(err.message, "error");
  }
}

async function runFix(fixID) {
  if (!state.selectedProject) return;
  try {
    await api(`/v1/projects/${state.selectedProject.id}/fixes/${fixID}/run`, {
      method: "POST",
      body: JSON.stringify({ requested_by: "ui" }),
    });
    showToast("Fix queued");
    setTimeout(refreshSelectedProject, 800);
  } catch (err) {
    showToast(err.message, "error");
  }
}

function openEditFixModal(fixID) {
  const fix = (state.data.fixes || []).find((f) => f.id === fixID);
  if (!fix) {
    showToast("Fix not found", "error");
    return;
  }
  el.editFixId.value = String(fix.id);
  el.editFixName.value = fix.name || "";
  el.editFixType.value = fix.type || "any";
  el.editFixScriptPath.value = fix.script_path || "";
  el.editFixTimeout.value = String(fix.timeout_sec || 60);
  el.editFixPattern.value = fix.supported_error_pattern || "";
  if (el.editFixModal) el.editFixModal.classList.remove("hidden");
}

function closeEditFixModal() {
  if (el.editFixModal) el.editFixModal.classList.add("hidden");
}

async function saveEditFix() {
  if (!state.selectedProject) return;
  const fixID = Number(el.editFixId.value);
  if (!fixID) return;

  const payload = {
    name: el.editFixName.value.trim(),
    type: el.editFixType.value,
    script_path: el.editFixScriptPath.value.trim(),
    timeout_sec: Number(el.editFixTimeout.value),
    supported_error_pattern: el.editFixPattern.value.trim(),
  };

  if (
    !payload.name ||
    !payload.script_path ||
    !payload.supported_error_pattern
  ) {
    showToast("Name, script path and error pattern are required", "error");
    return;
  }

  try {
    await api(`/v1/projects/${state.selectedProject.id}/fixes/${fixID}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    });
    closeEditFixModal();
    showToast("Fix updated");
    await refreshSelectedProject();
  } catch (err) {
    showToast(err.message, "error");
  }
}

async function deleteFix(fixID, fixName) {
  if (!state.selectedProject) return;
  const ok = window.confirm(
    `Delete fix "${fixName || fixID}"? This will remove the fix script and detach it from the project.`,
  );
  if (!ok) return;

  try {
    await api(`/v1/projects/${state.selectedProject.id}/fixes/${fixID}`, {
      method: "DELETE",
    });
    showToast("Fix deleted");
    await refreshSelectedProject();
  } catch (err) {
    showToast(err.message, "error");
  }
}

function renderUptimeFixesWidget() {
  if (!el.uptimeFixesList) return;
  const fixes = state.data.fixes || [];
  if (!state.selectedProject || fixes.length === 0) {
    el.uptimeFixesList.innerHTML = `<div class="list-item"><div class="main">No fixes attached</div></div>`;
    return;
  }
  el.uptimeFixesList.innerHTML = fixes
    .map(
      (f) => `
      <div class="list-item">
        <div class="main">
          <strong>${escapeHtml(f.name)}</strong>
          <span class="meta">${escapeHtml(f.type)} | ${escapeHtml(f.script_path)}</span>
          <span class="meta">pattern: ${escapeHtml(clampText(f.supported_error_pattern, 80))}</span>
        </div>
        <div class="inline-actions">
          <button class="btn secondary" data-run-fix-id="${f.id}">Run</button>
        </div>
      </div>`,
    )
    .join("");
}

function applyTemplate(idx) {
  const tpl = errorTemplates[idx];
  if (!tpl) return;

  if (state.patternTarget && state.patternTarget instanceof HTMLInputElement) {
    state.patternTarget.value = tpl.value;
  } else {
    el.fixPattern.value = tpl.value;
    el.uploadFixPattern.value = tpl.value;
  }
  showToast(`Template applied: ${tpl.label}`);
}

function toggleCreatePanel(show) {
  if (!el.createPanel) return;
  el.createPanel.classList.toggle("hidden", !show);
  if (!show && el.createProjectForm) el.createProjectForm.reset();
}

function bindPatternInputs() {
  [el.fixPattern, el.uploadFixPattern, el.editFixPattern].forEach((input) => {
    if (!input) return;
    input.addEventListener("focus", () => {
      state.patternTarget = input;
    });
  });
}

function startPolling() {
  stopPolling();
  state.pollTimer = window.setInterval(() => {
    if (document.hidden) return;
    refreshSelectedProject();
  }, 5000);
}

function stopPolling() {
  if (!state.pollTimer) return;
  clearInterval(state.pollTimer);
  state.pollTimer = null;
}

function attachEvents() {
  (el.navBtns || []).forEach((btn) => {
    btn.addEventListener("click", async () => {
      const view = btn.dataset.view;
      selectView(view);
    });
  });

  (el.rangeBtns || []).forEach((btn) => {
    btn.addEventListener("click", async () => {
      const range = btn.dataset.range;
      setRange(range);
      await refreshSelectedProject();
    });
  });

  if (el.projectSelect) {
    el.projectSelect.addEventListener("change", async () => {
      const id = Number(el.projectSelect.value);
      if (!id) return;
      state.selectedProjectId = id;
      state.selectedProject = state.projects.find((p) => p.id === id) || null;
      state.selectedPathCheckId = null;
      state.selectedPathTarget = "";
      setActionButtons(Boolean(state.selectedProject));
      await refreshSelectedProject();
    });
  }

  if (el.openCreateBtn)
    el.openCreateBtn.addEventListener("click", () => toggleCreatePanel(true));
  if (el.closeCreateBtn)
    el.closeCreateBtn.addEventListener("click", () => toggleCreatePanel(false));
  const createDiscardBtn = document.getElementById("createDiscardBtn");
  if (createDiscardBtn)
    createDiscardBtn.addEventListener("click", () => toggleCreatePanel(false));
  const createSubmitBtn = document.getElementById("createSubmitBtn");
  if (createSubmitBtn)
    createSubmitBtn.addEventListener("click", () => {
      if (el.createProjectForm && el.createProjectForm.reportValidity()) {
        createProject(new Event("submit", { cancelable: true }));
      }
    });
  if (el.createProjectForm)
    el.createProjectForm.addEventListener("submit", createProject);
  if (el.createPanel) {
    el.createPanel.addEventListener("click", (e) => {
      if (e.target === el.createPanel) toggleCreatePanel(false);
    });
  }

  if (el.refreshBtn) {
    el.refreshBtn.addEventListener("click", async () => {
      try {
        await loadProjects();
        await refreshSelectedProject();
        showToast("Refreshed");
      } catch (err) {
        showToast(err.message, "error");
      }
    });
  }

  if (el.runNowBtn) el.runNowBtn.addEventListener("click", runChecksNow);
  if (el.toggleAutofixBtn)
    el.toggleAutofixBtn.addEventListener("click", toggleAutofix);
  if (el.deleteProjectBtn)
    el.deleteProjectBtn.addEventListener("click", deleteProject);

  if (el.fixForm) el.fixForm.addEventListener("submit", createFix);
  if (el.fixUploadForm) el.fixUploadForm.addEventListener("submit", uploadFix);

  // Settings modal
  if (el.openSettingsBtn)
    el.openSettingsBtn.addEventListener("click", openSettingsModal);
  if (el.closeSettingsX)
    el.closeSettingsX.addEventListener("click", closeSettingsModal);
  if (el.settingsDiscardBtn)
    el.settingsDiscardBtn.addEventListener("click", () => {
      renderSettingsForm();
      closeSettingsModal();
    });
  if (el.settingsSaveBtn)
    el.settingsSaveBtn.addEventListener("click", () => saveSettings());
  if (el.createSMTPBtn)
    el.createSMTPBtn.addEventListener("click", createSMTPProfileFromSettings);
  if (el.settingsModal) {
    el.settingsModal.addEventListener("click", (event) => {
      if (event.target === el.settingsModal) closeSettingsModal();
    });
  }

  // Sidebar toggle
  if (el.closeSidebarBtn)
    el.closeSidebarBtn.addEventListener("click", () => toggleSidebar(false));
  if (el.openSidebarBtn)
    el.openSidebarBtn.addEventListener("click", () => toggleSidebar(true));

  if (el.fixesList) {
    el.fixesList.addEventListener("click", async (event) => {
      const runBtn = event.target.closest("button[data-run-fix-id]");
      if (runBtn) {
        await runFix(Number(runBtn.dataset.runFixId));
        return;
      }
      const editBtn = event.target.closest("button[data-edit-fix-id]");
      if (editBtn) {
        openEditFixModal(Number(editBtn.dataset.editFixId));
        return;
      }
      const deleteBtn = event.target.closest("button[data-delete-fix-id]");
      if (deleteBtn) {
        await deleteFix(
          Number(deleteBtn.dataset.deleteFixId),
          deleteBtn.dataset.fixName || "",
        );
        return;
      }
    });
  }

  if (el.uptimeFixesList) {
    el.uptimeFixesList.addEventListener("click", async (event) => {
      const runBtn = event.target.closest("button[data-run-fix-id]");
      if (!runBtn) return;
      await runFix(Number(runBtn.dataset.runFixId));
    });
  }

  // Edit fix modal
  if (el.closeEditFixX)
    el.closeEditFixX.addEventListener("click", closeEditFixModal);
  if (el.editFixDiscardBtn)
    el.editFixDiscardBtn.addEventListener("click", closeEditFixModal);
  if (el.editFixSaveBtn)
    el.editFixSaveBtn.addEventListener("click", saveEditFix);
  if (el.editFixModal) {
    el.editFixModal.addEventListener("click", (event) => {
      if (event.target === el.editFixModal) closeEditFixModal();
    });
  }
  if (el.editFixForm) {
    el.editFixForm.addEventListener("submit", (e) => {
      e.preventDefault();
      saveEditFix();
    });
  }

  if (el.pathsHealthList) {
    el.pathsHealthList.addEventListener("click", async (event) => {
      const button = event.target.closest("button[data-path-check-id]");
      if (!button) return;
      state.selectedPathCheckId = Number(button.dataset.pathCheckId);
      const selected = (state.data.pathHealth || []).find(
        (p) => p.check_id === state.selectedPathCheckId,
      );
      state.selectedPathTarget = selected?.target || "";
      await refreshSelectedPathRuns();
    });
  }

  if (el.settingsForm) {
    el.settingsForm.addEventListener("submit", (e) => {
      e.preventDefault();
      saveSettings();
    });
  }

  if (el.addSettingsCheckBtn) {
    el.addSettingsCheckBtn.addEventListener("click", () => {
      addSettingsCheckRow();
    });
  }

  if (el.settingsChecksRows) {
    el.settingsChecksRows.addEventListener("click", (event) => {
      const removeBtn = event.target.closest("button[data-remove-check]");
      if (!removeBtn) return;
      const row = removeBtn.closest(".settings-check-row");
      if (row) {
        row.remove();
      }
    });
  }

  if (el.templateList) {
    el.templateList.addEventListener("click", (event) => {
      const button = event.target.closest("button[data-template-index]");
      if (!button) return;
      applyTemplate(Number(button.dataset.templateIndex));
    });
  }

  if (el.uptimeCanvas) {
    el.uptimeCanvas.addEventListener("mousemove", (event) => {
      if (state.activeView !== "uptimeView") return;
      const idx = pickUptimeHoverIndex(event.clientX);
      if (idx === null) {
        if (state.uptimeHoverIndex !== null) {
          state.uptimeHoverIndex = null;
          renderUptimeCanvas();
        }
        return;
      }
      if (state.uptimeHoverIndex !== idx) {
        state.uptimeHoverIndex = idx;
        renderUptimeCanvas();
      }
    });

    el.uptimeCanvas.addEventListener("click", (event) => {
      if (state.activeView !== "uptimeView") return;
      const idx = pickUptimeHoverIndex(event.clientX);
      if (idx === null) return;
      state.uptimeHoverIndex = idx;
      renderUptimeCanvas();
    });

    el.uptimeCanvas.addEventListener("mouseleave", () => {
      if (state.uptimeHoverIndex !== null) {
        state.uptimeHoverIndex = null;
        renderUptimeCanvas();
      }
      hideUptimeTooltip();
    });
  }

  window.addEventListener("resize", () => {
    if (state.activeView === "uptimeView") {
      scheduleUptimeRender();
    }
  });

  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) {
      refreshSelectedProject();
    }
  });

  bindPatternInputs();
}

async function boot() {
  renderTemplateButtons();
  setRange("1h");
  const initialView = getPersistedView() || "dashboardView";
  const safeInitialView =
    initialView === "settingsView" ? "dashboardView" : initialView;
  selectView(safeInitialView);
  attachEvents();

  if (
    typeof window !== "undefined" &&
    "ResizeObserver" in window &&
    el.uptimeCanvasWrap &&
    !state.uptimeResizeObserver
  ) {
    state.uptimeResizeObserver = new ResizeObserver(() => {
      if (state.activeView === "uptimeView") {
        scheduleUptimeRender();
      }
    });
    state.uptimeResizeObserver.observe(el.uptimeCanvasWrap);
  }

  try {
    await loadProjects();
    await refreshSelectedProject();
  } catch (err) {
    setBanner(`Startup error: ${err.message}`);
    setLiveState("error");
  }

  startPolling();
}

boot();
