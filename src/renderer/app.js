(() => {
  const bridge = window.jisu;
  const byId = (id) => document.getElementById(id);
  const labels = {
    all: "全部任务",
    downloading: "下载中",
    queued: "排队中",
    completed: "已完成",
    paused: "已暂停",
    failed: "下载失败",
  };
  const primaryActions = {
    downloading: { method: "pause", label: "暂停", icon: "pause" },
    queued: { method: "pause", label: "暂停", icon: "pause" },
    paused: { method: "resume", label: "继续下载", icon: "play" },
    failed: { method: "resume", label: "重试", icon: "retry" },
    completed: { method: "reveal", label: "在文件夹中显示", icon: "folder" },
  };

  const form = byId("download-form");
  const urlInput = byId("download-url");
  const addButton = byId("add-download");
  const downloadError = byId("download-error");
  const settingsButton = byId("open-settings");
  const settingsDialog = byId("settings-dialog");
  const directoryButton = byId("choose-directory");
  const settingError = byId("settings-error");
  const concurrency = byId("concurrency");
  const filterButtons = [...document.querySelectorAll("[data-filter]")];
  const taskList = byId("task-list");
  const taskTemplate = byId("task-template");
  const removeDialog = byId("remove-dialog");
  const notice = byId("notice");

  const rows = new Map();
  const pending = new Set();
  let snapshot;
  let filter = "all";
  let adding = false;
  let setting = false;
  let removing = null;
  let noticeTimer;
  let unsubscribe;
  let fallbackDialog;
  const hasState = (state) => Object.prototype.hasOwnProperty.call(primaryActions, state);

  function setText(element, value) {
    if (element.textContent !== value) element.textContent = value;
  }

  function setAttribute(element, name, value) {
    if (element.getAttribute(name) !== value) element.setAttribute(name, value);
  }

  function openDialog(dialog) {
    if (typeof dialog.showModal === "function") {
      dialog.showModal();
      return;
    }
    const opener = document.activeElement;
    fallbackDialog = { dialog, opener };
    document.body.classList.add("fallback-modal");
    dialog.setAttribute("open", "");
    dialog.setAttribute("aria-modal", "true");
    dialog.querySelector("[autofocus], button").focus();
    document.querySelector(".app-shell").setAttribute("aria-hidden", "true");
  }

  function closeDialog(dialog) {
    if (typeof dialog.close === "function") {
      dialog.close();
      return;
    }
    dialog.removeAttribute("open");
    dialog.removeAttribute("aria-modal");
    document.body.classList.remove("fallback-modal");
    document.querySelector(".app-shell").removeAttribute("aria-hidden");
    fallbackDialog?.opener.focus();
    fallbackDialog = null;
    dialog.dispatchEvent(new Event("close"));
  }

  function inlineMessage(element, message) {
    setText(element, message);
    element.hidden = !message;
  }

  function notify(message, isError = false) {
    window.clearTimeout(noticeTimer);
    notice.dataset.error = String(isError);
    setText(notice, message);
    if (message) {
      noticeTimer = window.setTimeout(() => setText(notice, ""), isError ? 10000 : 4000);
    }
  }

  function errorMessage(label, error) {
    const detail = typeof error?.message === "string" ? error.message : "";
    return detail ? `${label}：${detail}` : label;
  }

  function nonnegative(number) {
    return Number.isFinite(number) && number > 0 ? number : 0;
  }

  function bytes(number) {
    const units = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];
    let value = nonnegative(number);
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
      value /= 1024;
      unit += 1;
    }
    const precision = unit && value < 10 ? 1 : 0;
    return `${Number(value.toFixed(precision))} ${units[unit]}`;
  }

  function httpURL(text) {
    if (!/^https?:\/\//i.test(text)) return null;
    try {
      const url = new URL(text);
      return url.hostname ? url : null;
    } catch {
      return null;
    }
  }

  function jobName(job, url = httpURL(job.url)) {
    if (job.name) return job.name;
    const segment = url?.pathname.split("/").filter(Boolean).pop();
    if (!segment) return url?.hostname || "未命名文件";
    try { return decodeURIComponent(segment); } catch { return segment; }
  }

  function makeRow() {
    const node = taskTemplate.content.firstElementChild.cloneNode(true);
    const find = (className) => node.querySelector(`.${className}`);
    const row = {
      node,
      name: find("task-name"),
      source: find("task-source"),
      error: find("task-error"),
      state: find("task-state"),
      progress: node.querySelector("progress"),
      percentage: find("task-percentage"),
      size: find("task-size"),
      speed: find("task-speed"),
      connections: find("task-connections"),
      primary: find("task-primary"),
      remove: find("task-remove"),
    };
    row.icon = row.primary.querySelector("use");
    row.primary.addEventListener("click", () => {
      const action = hasState(row.job.state) && primaryActions[row.job.state];
      if (action) void jobCommand(row.job.id, action.method, action.label);
    });
    row.remove.addEventListener("click", () => {
      if (pending.has(row.job.id)) return;
      removing = row.job.id;
      renderRemoval();
      openDialog(removeDialog);
    });
    return row;
  }

  function renderRow(row, job) {
    row.job = job;
    const state = hasState(job.state) ? job.state : "";
    const action = primaryActions[state];
    const url = httpURL(job.url);
    const name = jobName(job, url);
    const active = state === "downloading";
    const done = state === "completed";
    const received = nonnegative(job.received);
    const total = Number.isFinite(job.total) && job.total >= 0 ? job.total : null;
    const progress = done ? 1 : total > 0 ? Math.min(received / total, 1) : 0;

    setAttribute(row.node, "data-state", state);
    setText(row.name, name);
    row.name.title = name;
    setText(row.source, url?.host || "—");
    row.source.title = job.directory || url?.host || "";
    const taskError = state === "failed" ? (job.error || "下载未完成") : "";
    inlineMessage(row.error, taskError);
    row.error.title = taskError;
    setText(row.state, labels[state] || "未知状态");
    setText(row.percentage, done || total > 0 ? `${Math.floor(progress * 100)}%` : "");
    setText(row.size, done ? bytes(received) : total === null ? `已下载 ${bytes(received)}` : `${bytes(received)} / ${bytes(total)}`);
    row.progress.hidden = !(done || active || total !== null);
    if (active && total === null) {
      row.progress.removeAttribute("value");
    } else if (!row.progress.hasAttribute("value") || row.progress.value !== progress) {
      row.progress.value = progress;
    }
    setText(row.speed, active ? `${bytes(job.bytesPerSecond)}/s` : "—");
    const connections = Math.floor(nonnegative(job.connections));
    setText(row.connections, active ? `${connections} 个连接` : "");

    row.primary.hidden = !action;
    if (action) {
      const href = `#icon-${action.icon}`;
      setAttribute(row.icon, "href", href);
      setAttribute(row.primary, "aria-label", `${action.label}：${name}`);
      row.primary.title = action.label;
    }
    setAttribute(row.remove, "aria-label", `移除任务：${name}`);
    const busy = String(pending.has(job.id));
    setAttribute(row.primary, "aria-disabled", busy);
    setAttribute(row.remove, "aria-disabled", busy);
  }

  function renderRemoval() {
    if (removing === null) return;
    const job = snapshot.jobs.find(({ id }) => id === removing);
    if (!job) {
      closeDialog(removeDialog);
      removing = null;
      return;
    }
    setText(byId("remove-name"), jobName(job));
    setText(byId("remove-note"), job.state === "completed"
      ? "仅移除下载记录，已下载的文件会保留。"
      : "已下载的部分文件也会被删除，之后无法继续此任务。");
  }

  function renderJobs() {
    const jobs = snapshot.jobs.filter((job) => filter === "all" || job.state === filter);
    const visible = new Set(jobs.map(({ id }) => id));
    let returnFocus = false;
    for (const [id, row] of rows) {
      if (visible.has(id)) continue;
      returnFocus ||= row.node.contains(document.activeElement);
      row.node.remove();
      rows.delete(id);
    }
    let cursor = taskList.firstElementChild;
    for (const job of jobs) {
      if (!rows.has(job.id)) rows.set(job.id, makeRow());
      const row = rows.get(job.id);
      renderRow(row, job);
      if (cursor !== row.node) {
        taskList.insertBefore(row.node, cursor);
      } else {
        cursor = cursor.nextElementSibling;
      }
    }

    setText(byId("tasks-title"), labels[filter]);
    setText(byId("tasks-count"), `${jobs.length} 项`);
    byId("task-columns").hidden = jobs.length === 0;
    taskList.hidden = jobs.length === 0;
    byId("empty-state").hidden = jobs.length !== 0;
    setText(byId("empty-title"), filter === "all" ? "还没有下载任务" : "此视图中没有任务");
    setText(byId("empty-description"), filter === "all" ? "在上方粘贴 HTTP 或 HTTPS 链接开始下载。" : "可从侧栏切换视图查看其他任务。");
    if (returnFocus) byId("tasks-title").focus();
  }

  function renderSettings() {
    const directory = snapshot.downloadDirectory || "未设置";
    setText(byId("settings-directory"), directory);
    setText(byId("sidebar-directory"), directory);
    byId("sidebar-directory").title = directory;
    if (!setting) concurrency.value = String(snapshot.concurrency);
  }

  function renderState(next) {
    snapshot = { ...next, jobs: next.jobs || [] };
    const counts = { all: snapshot.jobs.length };
    let speed = 0;
    for (const job of snapshot.jobs) {
      if (!hasState(job.state)) continue;
      counts[job.state] = (counts[job.state] || 0) + 1;
      if (job.state === "downloading") speed += nonnegative(job.bytesPerSecond);
    }
    for (const button of filterButtons) {
      setText(button.querySelector(".filter-count"), String(counts[button.dataset.filter] || 0));
    }
    setText(byId("metric-speed"), `${bytes(speed)}/s`);
    setText(byId("metric-active"), String(counts.downloading || 0));
    setText(byId("metric-completed"), String(counts.completed || 0));
    renderSettings();
    renderRemoval();
    renderJobs();
    addButton.disabled = adding;
    settingsButton.disabled = false;
  }

  function selectFilter(next) {
    filter = next;
    for (const button of filterButtons) {
      button.setAttribute("aria-pressed", String(button.dataset.filter === filter));
    }
    if (snapshot) renderJobs();
  }

  async function jobCommand(id, method, label) {
    if (pending.has(id)) return;
    pending.add(id);
    renderJobs();
    notify("");
    try {
      await bridge[method](id);
      if (method === "remove") notify("已移除下载任务");
    } catch (error) {
      notify(errorMessage(`${label}失败`, error), true);
    } finally {
      pending.delete(id);
      renderJobs();
    }
  }

  async function settingCommand(label, command) {
    if (setting) return;
    setting = true;
    directoryButton.disabled = true;
    concurrency.disabled = true;
    inlineMessage(settingError, "");
    try {
      await command();
    } catch (error) {
      inlineMessage(settingError, errorMessage(label, error));
    } finally {
      setting = false;
      directoryButton.disabled = false;
      concurrency.disabled = false;
      renderSettings();
    }
  }

  for (const button of filterButtons) {
    button.addEventListener("click", () => selectFilter(button.dataset.filter));
  }
  urlInput.addEventListener("input", () => {
    urlInput.removeAttribute("aria-invalid");
    inlineMessage(downloadError, "");
  });
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!snapshot || adding) return;
    const entry = urlInput.value;
    const url = entry.trim();
    if (!httpURL(url)) {
      urlInput.setAttribute("aria-invalid", "true");
      inlineMessage(downloadError, "请输入有效的 HTTP 或 HTTPS 链接。");
      urlInput.focus();
      return;
    }
    adding = true;
    addButton.disabled = true;
    inlineMessage(downloadError, "");
    try {
      await bridge.add(url);
      if (urlInput.value === entry) urlInput.value = "";
      selectFilter("all");
      notify("已添加下载任务");
      if (document.activeElement === document.body) urlInput.focus();
    } catch (error) {
      inlineMessage(downloadError, errorMessage("无法添加下载", error));
    } finally {
      adding = false;
      addButton.disabled = false;
    }
  });
  settingsButton.addEventListener("click", () => {
    if (!setting) inlineMessage(settingError, "");
    openDialog(settingsDialog);
  });
  byId("close-settings").addEventListener("click", () => closeDialog(settingsDialog));
  directoryButton.addEventListener("click", () => {
    void settingCommand("无法更改保存位置", async () => {
      const chosen = await bridge.chooseDirectory();
      if (chosen) snapshot.downloadDirectory = chosen;
    });
  });
  concurrency.addEventListener("change", () => {
    const count = Number(concurrency.value);
    const previous = snapshot.concurrency;
    void settingCommand("无法更改同时下载数", async () => {
      await bridge.setConcurrency(count);
      if (snapshot.concurrency === previous) snapshot.concurrency = count;
    });
  });
  byId("cancel-remove").addEventListener("click", () => closeDialog(removeDialog));
  removeDialog.addEventListener("close", () => { removing = null; });
  byId("confirm-remove").addEventListener("click", () => {
    if (removing === null) return;
    const id = removing;
    closeDialog(removeDialog);
    removing = null;
    void jobCommand(id, "remove", "移除");
  });
  for (const type of ["dragover", "drop"]) {
    window.addEventListener(type, (event) => {
      if (event.target !== urlInput) event.preventDefault();
    });
  }
  window.addEventListener("keydown", (event) => {
    if (!fallbackDialog) return;
    const { dialog } = fallbackDialog;
    if (event.key === "Escape") {
      event.preventDefault();
      closeDialog(dialog);
    } else if (event.key === "Tab") {
      const controls = [...dialog.querySelectorAll("button:not(:disabled), select:not(:disabled)")];
      const first = controls[0];
      const last = controls[controls.length - 1];
      const active = document.activeElement;
      if (!controls.includes(active) || (event.shiftKey ? first === active : last === active)) {
        event.preventDefault();
        (event.shiftKey ? last : first).focus();
      }
    }
  });

  async function connect() {
    let receivedEvent = false;
    try {
      if (!bridge) throw new Error("请在极速桌面应用中打开此界面。");
      unsubscribe = bridge.onState((next) => {
        receivedEvent = true;
        renderState(next);
      });
      const initial = await bridge.getState();
      if (!receivedEvent) renderState(initial);
    } catch (error) {
      if (receivedEvent) return;
      setText(byId("empty-title"), "无法读取下载任务");
      setText(byId("empty-description"), errorMessage("请重新打开应用", error));
    }
  }

  window.addEventListener("pagehide", () => unsubscribe?.(), { once: true });
  void connect();
})();
