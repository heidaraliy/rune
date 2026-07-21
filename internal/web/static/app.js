(() => {
  const state = {
    token: localStorage.getItem("rune.web.token") || "",
    view: "home",
    recordKind: "task",
    recordParent: "",
    filter: "active",
    sort: "updated_at",
    query: "",
    runes: [],
    selectedId: "",
    detail: null,
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
    const serverStatus = $("server-status");
    pill.textContent = label;
    pill.classList.toggle("online", online);
    pill.classList.toggle("offline", !online);
    serverStatus.textContent = online ? "Connected" : "Disconnected";
    document.querySelectorAll(".server-card .status-dot, .workspace-select .status-dot").forEach((dot) => {
      dot.classList.toggle("online", online);
    });
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

  function runeIcon(rune) {
    if (rune.kind === "task") return "✓";
    if (rune.kind === "idea") return "✦";
    return "▤";
  }

  function visibleRunes() {
    return state.runes.filter((rune) => {
      if (state.filter === "tombstoned") return Boolean(rune.deleted_at);
      if (state.filter === "all") return true;
      if (rune.deleted_at) return false;
      return !["task", "idea", "note"].includes(state.filter) || rune.kind === state.filter;
    });
  }

  function renderListInto(list, runes) {
    if (!list) return;
    list.replaceChildren();
    if (!runes.length) {
      const empty = make("div", "empty-list");
      const tombstones = state.filter === "tombstoned";
      empty.append(make("strong", "", tombstones ? "No tombstoned Runes." : "Nothing here yet."));
      empty.append(make("p", "", tombstones ? "Tombstones remain reversible and can be restored from this filter." : "Record a thought above, then let it evolve."));
      list.append(empty);
      return;
    }
    for (const rune of runes) {
      const item = make("button", "rune-item");
      item.type = "button";
      item.classList.toggle("selected", rune.id === state.selectedId);
      item.classList.toggle("deleted", Boolean(rune.deleted_at));
      item.addEventListener("click", () => selectRune(rune.id));

      const top = make("div", "rune-item-top");
      const main = make("div", "rune-item-main");
      const icon = make("span", `rune-item-icon kind-${rune.kind}`, runeIcon(rune));
      main.append(icon, make("span", "rune-item-title", rune.title));
      top.append(main, make("span", "rune-id", displayID(rune.id)), make("span", "rune-item-more", "⋮"));

      const snippet = make("div", "rune-item-snippet", rune.body || "No information yet — open this Rune to add the useful version.");
      const bottom = make("div", "rune-item-bottom");
      const lifecycle = rune.deleted_at ? "tombstoned" : stateLabel(rune.state || rune.status);
      bottom.append(make("span", "kind-mark", rune.kind), make("span", "state-mark", lifecycle));
      if (rune.parent_id) bottom.append(make("span", "parent-mark", `child of ${displayID(rune.parent_id)}`));
      bottom.append(make("span", "rune-date", dateLabel(rune.updated_at)));
      item.append(top, snippet, bottom);
      list.append(item);
    }
  }

  function renderList() {
    const runes = visibleRunes();
    $("rune-count").textContent = String(runes.length);
    $("runes-count").textContent = String(runes.length);
    renderListInto($("rune-list"), runes);
    renderListInto($("runes-list"), runes);
  }

  function renderRecordKind() {
    document.querySelectorAll("[data-record-kind]").forEach((button) => {
      button.classList.toggle("active", button.dataset.recordKind === state.recordKind);
    });
  }

  function renderRecordTarget() {
    const selected = state.detail && !state.detail.rune.deleted_at ? state.detail.rune : null;
    const selectedTarget = $("record-selected-target");
    selectedTarget.textContent = selected ? `to selected Rune: ${selected.title}` : "to selected Rune";
    selectedTarget.dataset.recordParent = selected ? selected.id : "";
    selectedTarget.disabled = !selected;
    if (!selected || (state.recordParent && state.recordParent !== selected.id)) state.recordParent = "";
    document.querySelectorAll("[data-record-parent]").forEach((button) => {
      button.classList.toggle("active", button.dataset.recordParent === state.recordParent);
    });
  }

  function renderChildren(children) {
    const list = $("child-list");
    const values = children || [];
    $("child-count").textContent = String(values.length);
    list.replaceChildren();
    if (!values.length) {
      list.append(make("div", "empty-children", "No child Runes yet. Use the composer above to add one."));
      return;
    }
    for (const child of values) {
      const row = make("button", "child-row");
      row.type = "button";
      row.addEventListener("click", () => selectRune(child.id));
      row.append(make("span", `child-icon kind-${child.kind}`, runeIcon(child)));
      const copy = make("span", "child-copy");
      copy.append(make("strong", "", child.title));
      copy.append(make("small", "", child.body || "No information yet."));
      row.append(copy, make("span", "child-state", child.deleted_at ? "tombstoned" : stateLabel(child.state || child.status)));
      list.append(row);
    }
  }

  function renderDetail() {
    const form = $("detail-form");
    const empty = $("detail-empty");
    if (!state.detail) {
      form.classList.add("hidden");
      empty.classList.remove("hidden");
      $("child-list").replaceChildren();
      return;
    }
    form.classList.remove("hidden");
    empty.classList.add("hidden");
    const rune = state.detail.rune;
    const deleted = Boolean(rune.deleted_at);
    $("detail-kind").textContent = `${rune.kind.toUpperCase()} · ${displayID(rune.id)}`;
    $("detail-title").value = rune.title || "";
    $("detail-body").value = rune.body || "";
    $("detail-state").value = rune.state || "draft";
    $("detail-revision").textContent = `rev ${rune.revision}`;
    $("detail-id").textContent = displayID(rune.id);
    $("detail-project").textContent = rune.project ? `project:${rune.project}` : "workspace";
    $("detail-updated").textContent = dateLabel(rune.updated_at);
    $("detail-deleted").classList.toggle("hidden", !deleted);
    $("detail-state").disabled = deleted;
    $("detail-title").disabled = deleted;
    $("detail-body").disabled = deleted;
    $("save-detail").disabled = deleted;
    $("new-child").disabled = deleted;

    const deleteButton = $("delete-detail");
    deleteButton.textContent = deleted ? "Restore Rune" : "Tombstone";
    deleteButton.classList.toggle("button-danger", !deleted);
    deleteButton.classList.toggle("button-quiet", deleted);
    $("queue-run").classList.toggle("hidden", rune.kind !== "task" || deleted);
    renderChildren(state.detail.children);

    const links = $("detail-links");
    links.replaceChildren();
    if (state.detail.links && state.detail.links.length) {
      links.append(make("div", "eyebrow", "Connections"));
      for (const link of state.detail.links) {
        links.append(make("div", "connection-row", `${link.kind} · ${displayID(link.from_id)} → ${displayID(link.to_id)}`));
      }
    }
    renderRecordTarget();
  }

  function renderRuns() {
    const list = $("run-list");
    list.replaceChildren();
    if (!state.runs.length) {
      const empty = make("div", "empty-runs");
      empty.append(make("span", "activity-glyph", "⌁"));
      empty.append(make("strong", "", "No agent runs yet."));
      empty.append(make("p", "", "Queue a Rune to see execution activity here."));
      list.append(empty);
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

  function renderView() {
    const placeholder = {
      agents: ["Agents", "Agent execution will live here once the Rune run contract is expanded."],
      activity: ["Activity", "A complete workspace timeline is planned for a later slice."],
      settings: ["Settings", "Workspace and server settings will become editable here."],
    };
    document.querySelectorAll(".app-view").forEach((view) => {
      const active = view.id === `${state.view}-view` || (!["home", "runes"].includes(state.view) && view.id === "placeholder-view");
      view.classList.toggle("hidden", !active);
    });
    document.querySelectorAll("[data-view]").forEach((button) => {
      button.classList.toggle("active", button.dataset.view === state.view);
    });
    if (state.view !== "home" && state.view !== "runes") {
      const [title, copy] = placeholder[state.view] || placeholder.agents;
      $("placeholder-title").textContent = title;
      $("placeholder-copy").textContent = copy;
      $("placeholder-eyebrow").textContent = "Coming next";
    }
  }

  function syncToolbarValues() {
    $("sort-filter").value = state.sort;
    $("runes-sort-filter").value = state.sort;
    $("filter-by").value = state.filter;
    $("runes-filter-by").value = state.filter;
    $("search").value = state.query;
    $("runes-search").value = state.query;
  }

  async function refresh() {
    if (!state.token) {
      setConnection(false, "token needed");
      return;
    }
    state.loading = true;
    try {
      const query = new URLSearchParams();
      if (["task", "idea", "note"].includes(state.filter)) query.set("kind", state.filter);
      query.set("include_deleted", state.filter === "tombstoned" || state.filter === "all" ? "true" : "false");
      if (state.sort) query.set("sort", state.sort);
      if (state.query) query.set("query", state.query);
      const [runeResponse, status, runs] = await Promise.all([
        api(`/v1/runes?${query}`),
        api("/v1/status"),
        api("/v1/runs"),
      ]);
      state.runes = runeResponse.runes || [];
      state.runs = runs.runs || [];
      $("workspace-name").textContent = status.workspace_id || "local";
      setConnection(true);
      $("auth-hint").textContent = "Connected. The token stays in this browser only.";
      const runes = visibleRunes();
      if (state.selectedId && runes.some((rune) => rune.id === state.selectedId)) {
        await loadDetail(state.selectedId, false);
      } else if (runes.length) {
        state.selectedId = runes[0].id;
        await loadDetail(runes[0].id, false);
      } else {
        state.selectedId = "";
        state.detail = null;
      }
      syncToolbarValues();
      renderList();
      renderDetail();
      renderRuns();
      renderView();
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

  async function selectRune(id) {
    state.view = "home";
    renderView();
    await loadDetail(id);
  }

  function chooseView(view) {
    state.view = view;
    renderView();
  }

  function setFilter(value) {
    state.filter = value;
    refresh();
  }

  function setSort(value) {
    state.sort = value;
    refresh();
  }

  $("connect").addEventListener("click", () => {
    state.token = tokenInput.value.trim();
    if (!state.token) return toast("Paste the server token first.", true);
    localStorage.setItem("rune.web.token", state.token);
    refresh();
  });
  tokenInput.addEventListener("keydown", (event) => { if (event.key === "Enter") $("connect").click(); });
  $("copy-token").addEventListener("click", async () => {
    if (!state.token) return toast("There is no token to copy.", true);
    try {
      await navigator.clipboard.writeText(state.token);
      toast("Server token copied.");
    } catch (error) {
      toast("The browser did not allow copying the token.", true);
    }
  });
  $("refresh").addEventListener("click", refresh);
  $("sync-now").addEventListener("click", async () => {
    try {
      $("sync-now").disabled = true;
      await api("/v1/sync", { method: "POST", body: "{}" });
      toast("Workspace synced.");
      await refresh();
    } catch (error) { toast(error.message, true); }
    finally { $("sync-now").disabled = false; }
  });

  document.querySelectorAll("[data-view]").forEach((button) => button.addEventListener("click", () => chooseView(button.dataset.view)));
  document.querySelectorAll("[data-record-kind]").forEach((button) => button.addEventListener("click", () => {
    state.recordKind = button.dataset.recordKind;
    renderRecordKind();
  }));
  document.querySelectorAll("[data-record-parent]").forEach((button) => button.addEventListener("click", () => {
    if (button.disabled) return;
    state.recordParent = button.dataset.recordParent || "";
    renderRecordTarget();
  }));
  $("sort-filter").addEventListener("change", (event) => setSort(event.target.value));
  $("runes-sort-filter").addEventListener("change", (event) => setSort(event.target.value));
  $("filter-by").addEventListener("change", (event) => setFilter(event.target.value));
  $("runes-filter-by").addEventListener("change", (event) => setFilter(event.target.value));

  let searchTimer;
  function updateSearch(value) {
    clearTimeout(searchTimer);
    state.query = value.trim();
    searchTimer = setTimeout(refresh, 180);
  }
  $("search").addEventListener("input", (event) => {
    $("runes-search").value = event.target.value;
    updateSearch(event.target.value);
  });
  $("runes-search").addEventListener("input", (event) => {
    $("search").value = event.target.value;
    updateSearch(event.target.value);
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "/" && !["INPUT", "TEXTAREA", "SELECT"].includes(document.activeElement.tagName)) {
      event.preventDefault();
      $(state.view === "runes" ? "runes-search" : "search").focus();
    }
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s" && state.detail) {
      event.preventDefault();
      $("detail-form").requestSubmit();
    }
  });

  $("new-rune").addEventListener("click", () => {
    state.view = "home";
    state.recordParent = "";
    renderView();
    renderRecordTarget();
    $("capture-title").focus();
    $("capture-title").scrollIntoView({ behavior: "smooth", block: "center" });
  });
  $("new-child").addEventListener("click", () => {
    if (!state.detail || state.detail.rune.deleted_at) return;
    state.view = "home";
    state.recordParent = state.detail.rune.id;
    renderView();
    renderRecordTarget();
    $("capture-title").focus();
    $("capture-title").scrollIntoView({ behavior: "smooth", block: "center" });
  });

  $("capture-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const title = $("capture-title").value.trim();
    if (!title) return;
    const payload = { kind: state.recordKind, title, body: $("capture-body").value };
    if (state.recordParent) payload.parent_id = state.recordParent;
    try {
      const response = await api("/v1/runes", { method: "POST", body: JSON.stringify(payload) });
      $("capture-title").value = "";
      $("capture-body").value = "";
      state.selectedId = response.rune.id;
      state.recordParent = "";
      toast(`Recorded ${response.rune.kind}.`);
      await refresh();
    } catch (error) { toast(error.message, true); }
  });

  $("detail-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!state.detail || state.detail.rune.deleted_at) return;
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
        state.filter = "active";
        toast("Rune restored.");
      } else {
        if (!window.confirm("Tombstone this Rune? It can be restored later.")) return;
        const response = await api(`/v1/runes/${encodeURIComponent(rune.id)}`, { method: "DELETE", body: JSON.stringify({ expected_revision: rune.revision }) });
        state.selectedId = response.rune.id;
        state.filter = "tombstoned";
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

  renderRecordKind();
  renderRecordTarget();
  renderView();
  refresh();
})();
