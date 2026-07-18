(() => {
  const state = {
    token: localStorage.getItem("rune.web.token") || "",
    kind: "",
    state: "",
    query: "",
    runes: [],
    selectedId: "",
    detail: null,
    sync: null,
    conflicts: [],
    runs: [],
    loading: false,
  };

  const $ = (id) => document.getElementById(id);
  const tokenInput = $("token-input");
  tokenInput.value = state.token;

  function api(path, options = {}) {
    const headers = new Headers(options.headers || {});
    headers.set("Accept", "application/json");
    if (options.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
    if (state.token) headers.set("Authorization", `Bearer ${state.token}`);
    return fetch(path, { ...options, headers }).then(async (response) => {
      const payload = await response.json().catch(() => ({}));
      if (!response.ok) {
        if (response.status === 401) setConnection(false, "Token required");
        throw new Error(payload.error || `Request failed (${response.status})`);
      }
      return payload;
    });
  }

  function setConnection(online, label = online ? "connected" : "offline") {
    const pill = $("connection-pill");
    pill.textContent = label;
    pill.classList.toggle("online", online);
    pill.classList.toggle("offline", !online);
  }

  let toastTimer;
  function toast(message, error = false) {
    const node = $("toast");
    node.textContent = message;
    node.classList.toggle("error", error);
    node.classList.add("visible");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => node.classList.remove("visible"), 3200);
  }

  function displayID(id) { return id && id.length > 8 ? id.slice(0, 8) : id; }
  function stateLabel(value) { return (value || "draft").replaceAll("_", " "); }
  function dateLabel(value) {
    if (!value) return "";
    const date = new Date(value);
    return Number.isNaN(date.valueOf()) ? "" : date.toLocaleDateString(undefined, { month: "short", day: "numeric" });
  }

  function make(tag, className, text) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined) node.textContent = text;
    return node;
  }

  function renderList() {
    const list = $("rune-list");
    list.replaceChildren();
    $("rune-count").textContent = String(state.runes.length);
    if (!state.runes.length) {
      const empty = make("div", "empty-list");
      empty.append(make("strong", "", "Nothing here yet."));
      empty.append(make("p", "", "Capture a note or task above, then let it evolve."));
      list.append(empty);
      return;
    }
    for (const rune of state.runes) {
      const item = make("button", "rune-item");
      item.type = "button";
      item.classList.toggle("selected", rune.id === state.selectedId);
      item.classList.toggle("deleted", Boolean(rune.deleted_at));
      item.addEventListener("click", () => selectRune(rune.id));
      const top = make("div", "rune-item-top");
      top.append(make("span", "rune-item-title", rune.title));
      top.append(make("span", "rune-id", displayID(rune.id)));
      const snippet = make("div", "rune-item-snippet", rune.body || "No body yet — open this Rune to add the useful version.");
      const bottom = make("div", "rune-item-bottom");
      const kind = make("span", "kind-mark", rune.kind);
      const lifecycle = rune.deleted_at ? "deleted" : stateLabel(rune.state || rune.status);
      bottom.append(kind, make("span", "state-mark", lifecycle), make("span", "", dateLabel(rune.updated_at)));
      item.append(top, snippet, bottom);
      list.append(item);
    }
  }

  function renderDetail() {
    const form = $("detail-form");
    const empty = $("detail-empty");
    if (!state.detail) {
      form.classList.add("hidden");
      empty.classList.remove("hidden");
      return;
    }
    form.classList.remove("hidden");
    empty.classList.add("hidden");
    const rune = state.detail.rune;
    $("detail-kind").textContent = `${rune.kind.toUpperCase()} · ${displayID(rune.id)}`;
    $("detail-title").value = rune.title || "";
    $("detail-body").value = rune.body || "";
    $("detail-state").value = rune.state || "draft";
    $("detail-revision").textContent = `rev ${rune.revision}`;
    $("detail-project").textContent = rune.project ? `project:${rune.project}` : "workspace note";
    $("detail-updated").textContent = dateLabel(rune.updated_at);
    $("detail-deleted").classList.toggle("hidden", !rune.deleted_at);
    $("detail-state").disabled = Boolean(rune.deleted_at);
    $("detail-title").disabled = Boolean(rune.deleted_at);
    $("detail-body").disabled = Boolean(rune.deleted_at);
    $("save-detail").disabled = Boolean(rune.deleted_at);
    const deleteButton = $("delete-detail");
    deleteButton.textContent = rune.deleted_at ? "Restore Rune" : "Tombstone";
    deleteButton.classList.toggle("button-danger", !rune.deleted_at);
    deleteButton.classList.toggle("button-quiet", Boolean(rune.deleted_at));
    $("queue-run").classList.toggle("hidden", rune.kind !== "task" || Boolean(rune.deleted_at));
    const links = $("detail-links");
    links.replaceChildren();
    if (state.detail.links && state.detail.links.length) {
      links.append(make("div", "eyebrow", "Connections"));
      for (const link of state.detail.links) {
        links.append(make("div", "", `${link.kind} · ${displayID(link.from_id)} → ${displayID(link.to_id)}`));
      }
    }
  }

  function renderRuns() {
    const list = $("run-list");
    list.replaceChildren();
    if (!state.runs.length) {
      list.append(make("div", "empty-runs", "No agent runs yet."));
      return;
    }
    for (const run of state.runs.slice(0, 6)) {
      const row = make("div", "run-row");
      row.append(make("span", "run-id", displayID(run.id)));
      row.append(make("span", "run-title", run.summary || `Task ${displayID(run.task_id)}`));
      row.append(make("span", "run-status", run.status));
      list.append(row);
    }
  }

  function renderSync() {
    const envelope = state.sync || {};
    const sync = envelope.status || envelope;
    const configured = Boolean(envelope.remote_configured) || sync.remote_state === "configured";
    $("sync-card").classList.toggle("connected", configured);
    $("sync-title").textContent = configured ? "Remote connected" : "Local-first";
    $("sync-detail").textContent = configured ? (envelope.remote_id || sync.remote_id || "Remote ready") : "Connect a remote through the Rune server to move this workspace between devices.";
    $("sync-counters").textContent = `${sync.pending_changes || 0} pending · ${sync.open_conflicts || 0} conflicts`;
  }

  async function refresh() {
    if (!state.token) {
      setConnection(false, "token needed");
      return;
    }
    state.loading = true;
    try {
      const query = new URLSearchParams();
      if (state.kind) query.set("kind", state.kind);
      if (state.state) query.set("state", state.state);
      if (state.query) query.set("query", state.query);
      const [runes, status, runs] = await Promise.all([
        api(`/v1/runes?${query}`),
        api("/v1/status"),
        api("/v1/runs"),
      ]);
      state.runes = runes.runes || [];
      state.sync = status.sync;
      state.conflicts = status.conflicts || [];
      state.runs = runs.runs || [];
      $("workspace-name").textContent = status.workspace_id || "local";
      setConnection(true);
      $("auth-hint").textContent = "Connected. The token stays in this browser only.";
      if (state.selectedId && state.runes.some((rune) => rune.id === state.selectedId)) {
        await loadDetail(state.selectedId, false);
      } else if (state.runes.length) {
        await loadDetail(state.runes[0].id, false);
      } else {
        state.selectedId = "";
        state.detail = null;
      }
      renderList();
      renderDetail();
      renderRuns();
      renderSync();
    } catch (error) {
      setConnection(false, "offline");
      toast(error.message, true);
    } finally {
      state.loading = false;
    }
  }

  async function loadDetail(id, redraw = true) {
    state.selectedId = id;
    try {
      state.detail = await api(`/v1/runes/${encodeURIComponent(id)}`);
      if (redraw) {
        renderList();
        renderDetail();
      }
    } catch (error) {
      toast(error.message, true);
    }
  }

  async function selectRune(id) { await loadDetail(id); }

  $("connect").addEventListener("click", () => {
    state.token = tokenInput.value.trim();
    if (!state.token) return toast("Paste the server token first.", true);
    localStorage.setItem("rune.web.token", state.token);
    refresh();
  });
  tokenInput.addEventListener("keydown", (event) => { if (event.key === "Enter") $("connect").click(); });
  $("refresh").addEventListener("click", refresh);
  $("sync-now").addEventListener("click", async () => {
    try {
      $("sync-now").disabled = true;
      await api("/v1/sync", { method: "POST", body: "{}" });
      toast("Synced shared state.");
      await refresh();
    } catch (error) { toast(error.message, true); }
    finally { $("sync-now").disabled = false; }
  });

  document.querySelectorAll("[data-kind]").forEach((button) => button.addEventListener("click", () => {
    state.kind = button.dataset.kind;
    document.querySelectorAll("[data-kind]").forEach((other) => other.classList.toggle("active", other === button));
    refresh();
  }));
  $("state-filter").addEventListener("change", (event) => { state.state = event.target.value; refresh(); });
  let searchTimer;
  $("search").addEventListener("input", (event) => {
    clearTimeout(searchTimer);
    state.query = event.target.value.trim();
    searchTimer = setTimeout(refresh, 180);
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "/" && document.activeElement.tagName !== "INPUT" && document.activeElement.tagName !== "TEXTAREA") {
      event.preventDefault(); $("search").focus();
    }
  });

  $("capture-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const title = $("capture-title").value.trim();
    if (!title) return;
    try {
      const response = await api("/v1/runes", {
        method: "POST",
        body: JSON.stringify({ kind: $("capture-kind").value, title, body: $("capture-body").value }),
      });
      $("capture-title").value = "";
      $("capture-body").value = "";
      state.selectedId = response.rune.id;
      toast(`Captured ${response.rune.kind}.`);
      await refresh();
    } catch (error) { toast(error.message, true); }
  });

  $("detail-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!state.detail) return;
    const rune = state.detail.rune;
    try {
      const response = await api(`/v1/runes/${encodeURIComponent(rune.id)}`, {
        method: "PATCH",
        body: JSON.stringify({
          expected_revision: rune.revision,
          title: $("detail-title").value,
          body: $("detail-body").value,
          state: $("detail-state").value,
        }),
      });
      state.selectedId = response.rune.id;
      toast("Rune updated.");
      await refresh();
    } catch (error) { toast(error.message, true); }
  });

  $("delete-detail").addEventListener("click", async () => {
    if (!state.detail) return;
    const rune = state.detail.rune;
    try {
      if (rune.deleted_at) {
        const response = await api(`/v1/runes/${encodeURIComponent(rune.id)}/restore`, { method: "POST", body: JSON.stringify({ expected_revision: rune.revision }) });
        state.selectedId = response.rune.id;
        toast("Rune restored.");
      } else {
        if (!window.confirm("Tombstone this Rune? It can be restored later.")) return;
        const response = await api(`/v1/runes/${encodeURIComponent(rune.id)}`, { method: "DELETE", body: JSON.stringify({ expected_revision: rune.revision }) });
        state.selectedId = response.rune.id;
        toast("Rune tombstoned.");
      }
      await refresh();
    } catch (error) { toast(error.message, true); }
  });

  $("queue-run").addEventListener("click", async () => {
    if (!state.detail) return;
    try {
      await api(`/v1/runes/${encodeURIComponent(state.detail.rune.id)}/queue`, { method: "POST", body: "{}" });
      toast("Queued a local agent run.");
      await refresh();
    } catch (error) { toast(error.message, true); }
  });

  refresh();
})();
