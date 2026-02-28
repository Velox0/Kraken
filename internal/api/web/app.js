const state = {
  projects: [],
  selectedProjectId: null,
  selectedProject: null,
  data: {
    checks: [],
    logs: [],
    incidents: [],
    runs: [],
    fixes: [],
  },
  activeView: "dashboardView",
  refreshInFlight: false,
  pendingRefresh: false,
  pollTimer: null,
  lastUpdated: null,
  patternTarget: null,
};

const templatePatterns = [
  { label: "Connection Refused", value: "connection refused|dial tcp" },
  {
    label: "HTTP 5xx",
    value: "status code 5[0-9]{2}|expected status .* got 5[0-9]{2}",
  },
  { label: "Timeout", value: "timeout|context deadline exceeded|i/o timeout" },
  {
    label: "DNS Errors",
    value:
      "no such host|server misbehaving|temporary failure in name resolution",
  },
  { label: "TLS/Handshake", value: "tls|certificate|x509|handshake" },
  { label: "Connection Reset", value: "connection reset|broken pipe|EOF" },
];

const el = {
  navBtns: Array.from(document.querySelectorAll(".nav-btn")),
  projectSelect: document.getElementById("projectSelect"),
  openCreateBtn: document.getElementById("openCreateBtn"),
  closeCreateBtn: document.getElementById("closeCreateBtn"),
  createPanel: document.getElementById("createPanel"),
  createProjectForm: document.getElementById("createProjectForm"),

  refreshBtn: document.getElementById("refreshBtn"),
  runNowBtn: document.getElementById("runNowBtn"),
  toggleAutofixBtn: document.getElementById("toggleAutofixBtn"),
  deleteProjectBtn: document.getElementById("deleteProjectBtn"),

  dashboardView: document.getElementById("dashboardView"),
  fixesView: document.getElementById("fixesView"),
  uptimeView: document.getElementById("uptimeView"),

  projectTitle: document.getElementById("projectTitle"),
  projectSubTitle: document.getElementById("projectSubTitle"),
  statusBadge: document.getElementById("statusBadge"),

  metricIncidents: document.getElementById("metricIncidents"),
  metricChecks: document.getElementById("metricChecks"),
  metricAutofix: document.getElementById("metricAutofix"),
  metricLastCheck: document.getElementById("metricLastCheck"),

  metricFixes: document.getElementById("metricFixes"),
  metricFixAutofix: document.getElementById("metricFixAutofix"),
  fixTemplateList: document.getElementById("fixTemplateList"),

  uptimeWidget: document.getElementById("uptimeWidget"),
  widgetUptimePct: document.getElementById("widgetUptimePct"),
  widgetTimeline: document.getElementById("widgetTimeline"),

  checksList: document.getElementById("checksList"),
  incidentsList: document.getElementById("incidentsList"),
  fixesList: document.getElementById("fixesList"),

  fixForm: document.getElementById("fixForm"),
  fixUploadForm: document.getElementById("fixUploadForm"),
  fixPattern: document.getElementById("fixPattern"),
  uploadFixPattern: document.getElementById("uploadFixPattern"),

  templateList: document.getElementById("fixTemplateList"),

  uptimeRecent: document.getElementById("uptimeRecent"),
  uptimeTotal: document.getElementById("uptimeTotal"),
  healthyRuns: document.getElementById("healthyRuns"),
  failedRuns: document.getElementById("failedRuns"),
  timeline: document.getElementById("timeline"),
  runsList: document.getElementById("runsList"),
  logsList: document.getElementById("logsList"),

  liveDot: document.getElementById("liveDot"),
  liveText: document.getElementById("liveText"),
  lastUpdatedText: document.getElementById("lastUpdatedText"),

  globalError: document.getElementById("globalError"),
  toast: document.getElementById("toast"),
};

async function api(path, opts = {}) {
  const headers = { ...(opts.headers || {}) };
  if (!(opts.body instanceof FormData) && !headers["content-type"]) {
    headers["content-type"] = "application/json";
  }

  const res = await fetch(path, { ...opts, headers });
  const bodyText = await res.text();
  let parsed = null;
  if (bodyText) {
    try {
      parsed = JSON.parse(bodyText);
    } catch (_) {}
  }

  if (!res.ok) {
    const error = (parsed && parsed.error) || `request failed (${res.status})`;
    throw new Error(error);
  }

  return parsed;
}

function showBanner(message) {
  if (!message) {
    el.globalError.classList.add("hidden");
    el.globalError.textContent = "";
    return;
  }
  el.globalError.textContent = message;
  el.globalError.classList.remove("hidden");
}

function toast(message, type = "ok") {
  el.toast.textContent = message;
  el.toast.classList.remove("hidden");
  if (type === "error") {
    el.toast.style.borderColor = "rgba(255,93,122,0.45)";
    el.toast.style.background = "rgba(68,18,33,0.95)";
  } else {
    el.toast.style.borderColor = "rgba(104,136,255,0.4)";
    el.toast.style.background = "rgba(17,26,52,0.95)";
  }
  setTimeout(() => el.toast.classList.add("hidden"), 2600);
}

function formatTime(ts) {
  if (!ts) return "-";
  return new Date(ts).toLocaleString();
}

function clampText(value, max = 220) {
  const s = String(value || "");
  return s.length > max ? `${s.slice(0, max)}...` : s;
}

function setLiveState(mode) {
  el.liveDot.classList.remove("syncing", "error");
  if (mode === "syncing") el.liveDot.classList.add("syncing");
  if (mode === "error") el.liveDot.classList.add("error");

  if (mode === "syncing") el.liveText.textContent = "Syncing data";
  else if (mode === "error") el.liveText.textContent = "Live updates degraded";
  else el.liveText.textContent = "Live updates on";
}

function renderProjectSelect() {
  if (!state.projects.length) {
    el.projectSelect.innerHTML = `<option value="">No projects</option>`;
    el.projectSelect.disabled = true;
    return;
  }
  el.projectSelect.disabled = false;
  el.projectSelect.innerHTML = state.projects
    .map((p) => {
      const selected = p.id === state.selectedProjectId ? "selected" : "";
      return `<option value="${p.id}" ${selected}>${escapeHtml(p.name)} (${escapeHtml(p.domain)})</option>`;
    })
    .join("");
}

function ensureSelectedProject() {
  if (
    state.selectedProjectId &&
    !state.projects.some((p) => p.id === state.selectedProjectId)
  ) {
    state.selectedProjectId = null;
  }
  if (!state.selectedProjectId && state.projects.length) {
    state.selectedProjectId = state.projects[0].id;
  }
  state.selectedProject =
    state.projects.find((p) => p.id === state.selectedProjectId) || null;
}

async function loadProjects() {
  state.projects = (await api("/v1/projects")) || [];
  ensureSelectedProject();
  renderProjectSelect();
  setControlsEnabled(Boolean(state.selectedProject));
}

function setControlsEnabled(enabled) {
  el.runNowBtn.disabled = !enabled;
  el.toggleAutofixBtn.disabled = !enabled;
  el.deleteProjectBtn.disabled = !enabled;
}

function renderTemplateButtons() {
  el.fixTemplateList.innerHTML = templatePatterns
    .map(
      (tpl, idx) =>
        `<button class="template-btn" data-template-index="${idx}">${escapeHtml(tpl.label)}</button>`,
    )
    .join("");
}

function selectView(viewId) {
  state.activeView = viewId;
  document.cookie = `kraken_view=${viewId};path=/;max-age=31536000;SameSite=Lax`;
  [el.dashboardView, el.fixesView, el.uptimeView].forEach((v) =>
    v.classList.remove("active"),
  );
  el[viewId].classList.add("active");
  el.navBtns.forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.view === viewId);
  });
}

function setEmptyState() {
  el.projectTitle.textContent = "None";
  el.projectSubTitle.textContent = "Create or select a project.";
  setStatusBadge("unknown", "Unknown");
  el.metricIncidents.textContent = "0";
  el.metricChecks.textContent = "0";
  el.metricAutofix.textContent = "Off";
  el.metricLastCheck.textContent = "-";
  el.checksList.innerHTML = "";
  el.incidentsList.innerHTML = "";
  el.fixesList.innerHTML = "";
  el.timeline.innerHTML = "";
  el.runsList.innerHTML = "";
  el.logsList.innerHTML = "";
  el.uptimeRecent.textContent = "0%";
  el.uptimeTotal.textContent = "0%";
  el.healthyRuns.textContent = "0";
  el.failedRuns.textContent = "0";
  el.metricFixes.textContent = "0";
  el.metricFixAutofix.textContent = "Off";
  el.widgetUptimePct.textContent = "-";
  el.widgetTimeline.innerHTML = "";
}

function setStatusBadge(kind, label) {
  el.statusBadge.className = `status-badge ${kind}`;
  el.statusBadge.textContent = label;
}

function computeStatus() {
  const openIncidents = state.data.incidents.filter((i) => i.status === "open");
  const latestRun = state.data.runs[0] || null;

  if (!state.selectedProject) {
    return { kind: "unknown", label: "Unknown" };
  }
  if (openIncidents.length > 0) {
    return { kind: "down", label: "Down" };
  }
  if (!latestRun) {
    return { kind: "unknown", label: "No Data" };
  }
  if (latestRun.status === "failed") {
    return { kind: "warn", label: "Warning" };
  }
  return { kind: "up", label: "Healthy" };
}

function renderDashboard() {
  if (!state.selectedProject) {
    setEmptyState();
    return;
  }

  const status = computeStatus();
  const latestRun = state.data.runs[0] || null;
  const openIncidents = state.data.incidents.filter((i) => i.status === "open");

  el.projectTitle.textContent = state.selectedProject.name;
  el.projectSubTitle.textContent = `${state.selectedProject.domain} | interval ${state.selectedProject.check_interval_sec}s | threshold ${state.selectedProject.failure_threshold}`;
  setStatusBadge(status.kind, status.label);

  el.metricIncidents.textContent = String(openIncidents.length);
  el.metricChecks.textContent = String(state.data.checks.length);
  el.metricAutofix.textContent = state.selectedProject.autofix_enabled
    ? "On"
    : "Off";
  el.metricLastCheck.textContent = latestRun
    ? formatTime(latestRun.created_at)
    : "-";

  renderChecks();
  renderIncidents();
  renderFixes();
  renderFixesView();
  renderUptimeWidget();
}

function renderChecks() {
  if (!state.data.checks.length) {
    el.checksList.innerHTML = `<div class="list-item"><div class="main">No checks configured</div></div>`;
    return;
  }

  el.checksList.innerHTML = state.data.checks
    .map((c) => {
      const type = c.type.toUpperCase();
      return `
      <div class="list-item">
        <div class="main">
          <strong>${escapeHtml(type)}</strong>
          <span class="meta">${escapeHtml(c.target)}</span>
          <span class="meta">timeout ${c.timeout_ms}ms | expected ${c.expected_status ?? "2xx/3xx"}</span>
        </div>
      </div>`;
    })
    .join("");
}

function renderIncidents() {
  if (!state.data.incidents.length) {
    el.incidentsList.innerHTML = `<div class="list-item"><div class="main">No incidents</div></div>`;
    return;
  }

  el.incidentsList.innerHTML = state.data.incidents
    .map((i) => {
      const kind = i.status === "open" ? "down" : "up";
      return `
      <div class="list-item">
        <div class="main">
          <strong class="status-${kind}">${escapeHtml(i.status.toUpperCase())}</strong>
          <span>${escapeHtml(clampText(i.error_message, 170))}</span>
          <span class="meta">started ${formatTime(i.started_at)}${i.resolved_at ? ` | resolved ${formatTime(i.resolved_at)}` : ""}</span>
        </div>
      </div>`;
    })
    .join("");
}

function renderFixes() {
  if (!state.data.fixes.length) {
    el.fixesList.innerHTML = `<div class="list-item"><div class="main">No fixes attached</div></div>`;
    return;
  }

  el.fixesList.innerHTML = state.data.fixes
    .map((f) => {
      return `
      <div class="list-item">
        <div class="main">
          <strong>${escapeHtml(f.name)}</strong>
          <span class="meta">${escapeHtml(f.type)} | ${escapeHtml(f.script_path)} | timeout ${f.timeout_sec}s</span>
          <span class="meta">pattern: ${escapeHtml(clampText(f.supported_error_pattern, 120))}</span>
        </div>
        <div class="inline-actions">
          <button class="btn secondary" data-run-fix-id="${f.id}">Run</button>
        </div>
      </div>`;
    })
    .join("");
}

function renderFixesView() {
  if (!state.selectedProject) return;
  el.metricFixes.textContent = String(state.data.fixes.length);
  el.metricFixAutofix.textContent = state.selectedProject.autofix_enabled
    ? "On"
    : "Off";
}

function renderUptimeWidget() {
  const runs = state.data.runs;
  if (!runs.length) {
    el.widgetUptimePct.textContent = "-";
    el.widgetUptimePct.className = "uptime-widget-pct";
    el.widgetTimeline.innerHTML = "";
    return;
  }

  const slice = runs.slice(0, 60);
  const healthy = slice.filter((r) => r.status === "healthy").length;
  const pct = ((healthy / slice.length) * 100).toFixed(1);

  el.widgetUptimePct.textContent = `${pct}%`;
  if (parseFloat(pct) >= 95) {
    el.widgetUptimePct.className = "uptime-widget-pct status-ok";
  } else if (parseFloat(pct) >= 80) {
    el.widgetUptimePct.className = "uptime-widget-pct status-warn";
  } else {
    el.widgetUptimePct.className = "uptime-widget-pct status-error";
  }

  el.widgetTimeline.innerHTML = slice
    .slice(0, 60)
    .reverse()
    .map(
      (r) =>
        `<span class="tick ${r.status === "healthy" ? "healthy" : "failed"}" title="${escapeHtml(r.status)} ${escapeHtml(formatTime(r.created_at))}"></span>`,
    )
    .join("");
}

function renderUptime() {
  const runs = state.data.runs;
  if (!runs.length) {
    el.uptimeRecent.textContent = "0%";
    el.uptimeTotal.textContent = "0%";
    el.healthyRuns.textContent = "0";
    el.failedRuns.textContent = "0";
    el.timeline.innerHTML = `<div class="list-item">No runs yet</div>`;
    el.runsList.innerHTML = "";
    el.logsList.innerHTML = "";
    return;
  }

  const total = runs.length;
  const healthy = runs.filter((r) => r.status === "healthy").length;
  const failed = total - healthy;
  const recentSlice = runs.slice(0, Math.min(30, runs.length));
  const recentHealthy = recentSlice.filter(
    (r) => r.status === "healthy",
  ).length;

  const pctTotal = ((healthy / total) * 100).toFixed(1);
  const pctRecent = ((recentHealthy / recentSlice.length) * 100).toFixed(1);

  el.uptimeTotal.textContent = `${pctTotal}%`;
  el.uptimeRecent.textContent = `${pctRecent}%`;
  el.healthyRuns.textContent = String(healthy);
  el.failedRuns.textContent = String(failed);

  el.timeline.innerHTML = runs
    .slice(0, 120)
    .reverse()
    .map(
      (r) =>
        `<span class="tick ${r.status === "healthy" ? "healthy" : "failed"}" title="${escapeHtml(r.status)} ${escapeHtml(formatTime(r.created_at))}"></span>`,
    )
    .join("");

  el.runsList.innerHTML = runs
    .slice(0, 120)
    .map((r) => {
      const kind = r.status === "healthy" ? "ok" : "error";
      return `
      <div class="list-item">
        <div class="main">
          <strong class="status-${kind}">${escapeHtml(r.status.toUpperCase())}</strong>
          <span>check ${r.check_id} | ${r.response_time_ms ?? "-"}ms${r.error_message ? ` | ${escapeHtml(clampText(r.error_message, 120))}` : ""}</span>
          <span class="meta">${formatTime(r.created_at)}</span>
        </div>
      </div>`;
    })
    .join("");

  el.logsList.innerHTML = state.data.logs
    .slice(0, 150)
    .map((l) => {
      const levelClass =
        l.level === "error" ? "error" : l.level === "warn" ? "warn" : "ok";
      return `
      <div class="list-item">
        <div class="main">
          <strong class="status-${levelClass}">[${escapeHtml(l.level)}]</strong>
          <span>${escapeHtml(clampText(l.message, 170))}</span>
          <span class="meta">${formatTime(l.timestamp)}</span>
        </div>
      </div>`;
    })
    .join("");
}

async function refreshSelectedProject() {
  if (!state.selectedProject) {
    setEmptyState();
    return;
  }

  if (state.refreshInFlight) {
    state.pendingRefresh = true;
    return;
  }

  state.refreshInFlight = true;
  const selectedID = state.selectedProject.id;
  setLiveState("syncing");

  try {
    const [checks, logs, incidents, runs, fixes] = await Promise.all([
      api(`/v1/projects/${selectedID}/checks`),
      api(`/v1/projects/${selectedID}/logs?limit=200`),
      api(`/v1/projects/${selectedID}/incidents?limit=80`),
      api(`/v1/projects/${selectedID}/check-runs?limit=300`),
      api(`/v1/projects/${selectedID}/fixes`),
    ]);

    if (!state.selectedProject || state.selectedProject.id !== selectedID) {
      return;
    }

    state.data = {
      checks: checks || [],
      logs: logs || [],
      incidents: incidents || [],
      runs: runs || [],
      fixes: fixes || [],
    };

    state.lastUpdated = new Date();
    el.lastUpdatedText.textContent = `Last updated: ${state.lastUpdated.toLocaleTimeString()}`;
    showBanner("");
    renderDashboard();
    renderUptime();
    setLiveState("ok");
  } catch (err) {
    setLiveState("error");
    showBanner(`Live refresh error: ${err.message}`);
  } finally {
    state.refreshInFlight = false;
    if (state.pendingRefresh) {
      state.pendingRefresh = false;
      refreshSelectedProject();
    }
  }
}

async function runNow() {
  if (!state.selectedProject) return;
  try {
    const res = await api(`/v1/projects/${state.selectedProject.id}/run-now`, {
      method: "POST",
    });
    toast(`Queued ${res.queued} checks`);
    setTimeout(refreshSelectedProject, 800);
  } catch (err) {
    toast(err.message, "error");
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
    const project = state.projects.find(
      (p) => p.id === state.selectedProject.id,
    );
    if (project) project.autofix_enabled = next;
    renderDashboard();
    toast(`Autofix ${next ? "enabled" : "disabled"}`);
  } catch (err) {
    toast(err.message, "error");
  }
}

async function deleteProject() {
  if (!state.selectedProject) return;
  const confirmDelete = window.confirm(
    `Delete project "${state.selectedProject.name}"?`,
  );
  if (!confirmDelete) return;

  try {
    await api(`/v1/projects/${state.selectedProject.id}`, { method: "DELETE" });
    toast("Project deleted");
    await loadProjects();
    if (!state.selectedProject) {
      setEmptyState();
    }
    await refreshSelectedProject();
  } catch (err) {
    toast(err.message, "error");
  }
}

async function createProject(event) {
  event.preventDefault();

  const name = document.getElementById("createName").value.trim();
  const domain = document.getElementById("createDomain").value.trim();
  const interval = Number(document.getElementById("createInterval").value);
  const threshold = Number(document.getElementById("createThreshold").value);
  const autofix = document.getElementById("createAutofix").checked;
  const emails = document
    .getElementById("createEmails")
    .value.split(",")
    .map((s) => s.trim())
    .filter(Boolean);
  const paths = document
    .getElementById("createPaths")
    .value.split("\n")
    .map((s) => s.trim())
    .filter(Boolean);

  try {
    const project = await api("/v1/projects", {
      method: "POST",
      body: JSON.stringify({
        name,
        domain,
        check_interval_sec: interval,
        failure_threshold: threshold,
        autofix_enabled: autofix,
        alert_emails: emails,
      }),
    });

    for (const path of paths) {
      const target = toHTTPPath(domain, path);
      await api(`/v1/projects/${project.id}/checks`, {
        method: "POST",
        body: JSON.stringify({
          type: "http",
          target,
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
    toast("Project created");
  } catch (err) {
    toast(err.message, "error");
  }
}

function toHTTPPath(domain, path) {
  if (path.startsWith("http://") || path.startsWith("https://")) return path;
  if (path.startsWith("/")) return `http://${domain}${path}`;
  return `http://${domain}/${path}`;
}

async function createFix(event) {
  event.preventDefault();
  if (!state.selectedProject) {
    toast("Select a project first", "error");
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
    toast("Fix added");
  } catch (err) {
    toast(err.message, "error");
  }
}

async function uploadFix(event) {
  event.preventDefault();
  if (!state.selectedProject) {
    toast("Select a project first", "error");
    return;
  }

  const fileInput = document.getElementById("uploadFixFile");
  if (!fileInput.files || fileInput.files.length === 0) {
    toast("Select a .sh file", "error");
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
    toast("Fix uploaded");
  } catch (err) {
    toast(err.message, "error");
  }
}

async function runFix(fixId) {
  if (!state.selectedProject) return;
  try {
    await api(`/v1/projects/${state.selectedProject.id}/fixes/${fixId}/run`, {
      method: "POST",
      body: JSON.stringify({ requested_by: "ui" }),
    });
    toast("Fix queued");
    setTimeout(refreshSelectedProject, 900);
  } catch (err) {
    toast(err.message, "error");
  }
}

function applyTemplate(index) {
  const template = templatePatterns[index];
  if (!template) return;

  const target = state.patternTarget;
  if (target && target instanceof HTMLInputElement) {
    target.value = template.value;
  } else {
    el.fixPattern.value = template.value;
    el.uploadFixPattern.value = template.value;
  }
  toast(`Template applied: ${template.label}`);
}

function toggleCreatePanel(show) {
  if (show) {
    el.createPanel.classList.remove("hidden");
  } else {
    el.createPanel.classList.add("hidden");
  }
}

function startPolling() {
  stopPolling();
  state.pollTimer = window.setInterval(() => {
    if (document.hidden || !state.selectedProject) return;
    refreshSelectedProject();
  }, 5000);
}

function stopPolling() {
  if (state.pollTimer) {
    clearInterval(state.pollTimer);
    state.pollTimer = null;
  }
}

function bindPatternTargets() {
  [el.fixPattern, el.uploadFixPattern].forEach((input) => {
    input.addEventListener("focus", () => {
      state.patternTarget = input;
    });
  });
}

function attachEvents() {
  el.navBtns.forEach((btn) => {
    btn.addEventListener("click", () => selectView(btn.dataset.view));
  });

  el.projectSelect.addEventListener("change", async () => {
    const id = Number(el.projectSelect.value);
    if (!id) return;
    state.selectedProjectId = id;
    state.selectedProject = state.projects.find((p) => p.id === id) || null;
    setControlsEnabled(Boolean(state.selectedProject));
    await refreshSelectedProject();
  });

  el.openCreateBtn.addEventListener("click", () => toggleCreatePanel(true));
  el.closeCreateBtn.addEventListener("click", () => toggleCreatePanel(false));
  el.createProjectForm.addEventListener("submit", createProject);

  el.refreshBtn.addEventListener("click", async () => {
    try {
      await loadProjects();
      await refreshSelectedProject();
      toast("Refreshed");
    } catch (err) {
      toast(err.message, "error");
    }
  });

  el.runNowBtn.addEventListener("click", runNow);
  el.toggleAutofixBtn.addEventListener("click", toggleAutofix);
  el.deleteProjectBtn.addEventListener("click", deleteProject);

  el.fixForm.addEventListener("submit", createFix);
  el.fixUploadForm.addEventListener("submit", uploadFix);

  el.fixesList.addEventListener("click", async (event) => {
    const button = event.target.closest("button[data-run-fix-id]");
    if (!button) return;
    await runFix(Number(button.dataset.runFixId));
  });

  el.uptimeWidget.addEventListener("click", () => selectView("uptimeView"));
  el.uptimeWidget.addEventListener("keydown", (e) => {
    if (e.key === "Enter" || e.key === " ") selectView("uptimeView");
  });

  el.fixTemplateList.addEventListener("click", (event) => {
    const button = event.target.closest("button[data-template-index]");
    if (!button) return;
    applyTemplate(Number(button.dataset.templateIndex));
  });

  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) {
      refreshSelectedProject();
    }
  });

  bindPatternTargets();
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

async function boot() {
  renderTemplateButtons();
  attachEvents();

  const savedView = (document.cookie.match(/(?:^|; )kraken_view=([^;]*)/) ||
    [])[1];
  const validViews = ["dashboardView", "fixesView", "uptimeView"];
  selectView(validViews.includes(savedView) ? savedView : "dashboardView");

  try {
    await loadProjects();
    if (state.selectedProject) {
      await refreshSelectedProject();
    } else {
      setEmptyState();
    }
  } catch (err) {
    showBanner(`Startup error: ${err.message}`);
  }

  startPolling();
}

boot();
