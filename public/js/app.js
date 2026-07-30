const SAVED_KEY = "railnow.savedRoutes";
const RECENT_KEY = "railnow.recentStations";
const RECENT_ROUTE_KEY = "railnow.recentRoutes";
let allowedDestinations = null;
let destinationRequest = 0;
let destinationTimer = null;
let savedRequest = 0;
let savedRefreshPromise = null;

function debounce(callback, delay) {
  let timer = null;
  return function () {
    const args = arguments;
    window.clearTimeout(timer);
    timer = window.setTimeout(() => callback.apply(null, args), delay);
  };
}

function scheduleDestinationOptions(from) {
  window.clearTimeout(destinationTimer);
  destinationTimer = window.setTimeout(() => updateDestinationOptions(from), 180);
}

function jakartaTime(includeSeconds) {
  const options = {
    timeZone: "Asia/Jakarta",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  };
  if (includeSeconds) options.second = "2-digit";
  try {
    return new Intl.DateTimeFormat("en-GB", options);
  } catch (error) {
    const fallback = {
      hour: "2-digit",
      minute: "2-digit",
      hourCycle: "h23",
    };
    if (includeSeconds) fallback.second = "2-digit";
    return new Intl.DateTimeFormat("en-GB", fallback);
  }
}
function jakartaSeconds() {
  const parts = jakartaTime(true).formatToParts();
  const value = (type) => {
    const part = parts.find((candidate) => candidate.type === type);
    return Number(part ? part.value : 0);
  };
  return value("hour") * 3600 + value("minute") * 60 + value("second");
}
function parseClock(value) {
  const match = String(value || "")
    .trim()
    .match(/^(\d{1,2}):([0-5]\d)(?::[0-5]\d)?$/);
  if (!match) return null;
  const hour = Number(match[1]);
  if (hour > 47) return null;
  return {
    hour,
    minute: Number(match[2]),
    label: `${String(hour).padStart(2, "0")}:${match[2]}`,
  };
}
function tick() {
  const clock = document.querySelector("#live-clock");
  if (clock) clock.textContent = `${jakartaTime(false).format()} WIB`;
  document.querySelectorAll("[data-clock-time]").forEach((el) => {
    const parsed = parseClock(el.textContent);
    if (parsed) el.textContent = parsed.label;
  });
  document.querySelectorAll("[data-countdown]").forEach((el) => {
    const parsed = parseClock(el.dataset.countdown);
    if (!parsed) {
      el.textContent = "—";
      return;
    }
    if (!el.dataset.targetEpoch) {
      let initialDiff =
        (parsed.hour % 24) * 3600 +
        parsed.minute * 60 -
        jakartaSeconds();
      const dayOffset = Math.max(0, Number(el.dataset.dayOffset || 0));
      initialDiff += dayOffset * 24 * 3600;
      if (dayOffset === 0 && initialDiff < 0) initialDiff = 0;
      el.dataset.targetEpoch = String(Date.now() + initialDiff * 1000);
    }
    const diff = Math.max(
      0,
      Math.ceil((Number(el.dataset.targetEpoch) - Date.now()) / 1000),
    );
    if (diff <= 0) {
      el.textContent = "Sekarang";
      const card = el.closest(".saved-route-card");
      if (card && card.dataset.expired !== "true") {
        card.dataset.expired = "true";
        window.setTimeout(() => refreshSavedSchedules(), 1500);
      }
      return;
    }
    const hours = Math.floor(diff / 3600);
    const minutes = Math.floor((diff % 3600) / 60);
    el.textContent = hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`;
  });
}
function read(key) {
  try {
    const value = JSON.parse(localStorage.getItem(key));
    return Array.isArray(value) ? value : [];
  } catch (error) {
    return [];
  }
}
function write(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch (error) {
    // Opera private mode may deny storage. The current interaction still works.
  }
}
function stationName(id) {
  const station = document.querySelector(`[data-station-id="${id}"]`);
  return station && station.dataset.stationName ? station.dataset.stationName : id;
}
function setStation(target, id, name) {
  document.querySelector(`#${target}`).value = id;
  const trigger = document.querySelector(`#${target}-trigger`);
  if (trigger) trigger.textContent = name;
  document.querySelector(`#${target}-query`).value = "";
  document.querySelector(`#${target}-options`).hidden = true;
  if (trigger) trigger.setAttribute("aria-expanded", "false");
  document.querySelectorAll(`#${target}-options [data-station-id]`).forEach((option) => {
    option.setAttribute("aria-selected", String(option.dataset.stationId === String(id)));
  });
  if (target === "from") scheduleDestinationOptions(id);
}
async function updateDestinationOptions(from) {
  const requestID = ++destinationRequest;
  const trigger = document.querySelector("#to-trigger");
  const swap = document.querySelector("#swap-route");
  const originalLabel = trigger ? trigger.textContent : "";
  const controller = typeof AbortController === "undefined" ? null : new AbortController();
  const timeoutID = controller
    ? window.setTimeout(() => controller.abort(), 8000)
    : null;
  if (trigger) {
    trigger.disabled = true;
    trigger.textContent = "Memuat tujuan…";
  }
  if (swap) swap.disabled = true;
  try {
    const response = await fetch(
      `/stations/destination-options?from=${encodeURIComponent(from)}`,
      controller ? { signal: controller.signal } : undefined,
    );
    if (!response.ok) return;
    const destinations = await response.json();
    if (requestID !== destinationRequest || !Array.isArray(destinations)) return;
    const allowed = new Set(destinations.map((station) => String(station.ID)));
    allowedDestinations = allowed;
    document
      .querySelectorAll("#to-options [data-station-id]")
      .forEach((option) => {
        option.hidden = !allowed.has(option.dataset.stationId);
      });
    const selected = document.querySelector("#to");
    if (!allowed.has(selected.value)) {
      const first = destinations[0];
      if (first) setStation("to", String(first.ID), first.Name);
    }
  } catch (error) {
    /* Keep the currently rendered options when offline. */
  } finally {
    if (timeoutID) window.clearTimeout(timeoutID);
    if (requestID !== destinationRequest) return;
    if (trigger) {
      trigger.disabled = false;
      if (trigger.textContent === "Memuat tujuan…")
        trigger.textContent = originalLabel || "Pilih stasiun";
    }
    if (swap) swap.disabled = false;
  }
}
function initStations() {
  const normalize = (value) =>
    value.toLocaleLowerCase().replace(/[^a-z0-9]/g, "");
  document.querySelectorAll(".station-trigger").forEach((trigger) => {
    const target = trigger.dataset.target;
    const selected = document.querySelector(`#${target}`).value;
    if (selected) {
      trigger.textContent = stationName(selected);
      document.querySelectorAll(`#${target}-options [data-station-id]`).forEach((option) => {
        option.setAttribute("aria-selected", String(option.dataset.stationId === String(selected)));
      });
    }
    trigger.addEventListener("click", () => {
      document.querySelectorAll(".station-options").forEach((panel) => {
        if (panel.id !== `${target}-options`) {
          panel.hidden = true;
          const otherTarget = panel.id.replace("-options", "");
          document.querySelector(`#${otherTarget}-trigger`)?.setAttribute("aria-expanded", "false");
        }
      });
      const panel = document.querySelector(`#${target}-options`);
      panel.hidden = !panel.hidden;
      trigger.setAttribute("aria-expanded", String(!panel.hidden));
      if (!panel.hidden) document.querySelector(`#${target}-query`).focus();
    });
    if (target === "from" && selected) scheduleDestinationOptions(selected);
  });
  document.querySelectorAll(".station-query").forEach((input) => {
    const target = input.dataset.target;
    const filterOptions = () => {
      const term = normalize(input.value);
      const options = document.querySelector(`#${target}-options`);
      document
        .querySelectorAll(`#${target}-options [data-station-id]`)
        .forEach((option) => {
          const searchable = normalize(
            `${option.dataset.stationName} ${option.textContent}`,
          );
          const notAllowed =
            target === "to" &&
            allowedDestinations &&
            !allowedDestinations.has(option.dataset.stationId);
          option.hidden =
            notAllowed || (term.length >= 2 && !searchable.includes(term));
        });
      options.hidden = false;
    };
    input.addEventListener("input", filterOptions);
    input.addEventListener("keydown", (event) => {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        document.querySelector(`#${target}-options [data-station-id]:not([hidden])`)?.focus();
      }
      if (event.key === "Escape") {
        document.querySelector(`#${target}-options`).hidden = true;
        document.querySelector(`#${target}-trigger`).setAttribute("aria-expanded", "false");
        document.querySelector(`#${target}-trigger`).focus();
      }
    });
  });
  document.querySelectorAll(".station-options").forEach((panel) => panel.addEventListener("keydown", (event) => {
    const options = [...panel.querySelectorAll("[data-station-id]:not([hidden])")];
    const index = options.indexOf(document.activeElement);
    const target = panel.id.replace("-options", "");
    if (event.key === "Escape") {
      event.preventDefault();
      panel.hidden = true;
      document.querySelector(`#${target}-trigger`).setAttribute("aria-expanded", "false");
      document.querySelector(`#${target}-trigger`).focus();
    }
    if ((event.key === "ArrowDown" || event.key === "ArrowUp") && options.length) {
      event.preventDefault();
      const next = event.key === "ArrowDown" ? Math.min(index + 1, options.length - 1) : Math.max(index - 1, 0);
      options[next < 0 ? 0 : next].focus();
    }
  }));
  document.addEventListener("click", (event) => {
    const option = event.target.closest("[data-station-id]");
    if (!option) return;
    const target = option
      .closest(".station-options")
      .id.replace("-options", "");
    setStation(target, option.dataset.stationId, option.dataset.stationName);
  });
  document.addEventListener("click", (event) => {
    if (event.target.closest(".station-options, .station-trigger")) return;
    document.querySelectorAll(".station-options").forEach((panel) => {
      panel.hidden = true;
      document.querySelector(`#${panel.id.replace("-options", "")}-trigger`)?.setAttribute("aria-expanded", "false");
    });
  });
  const swapButton = document.querySelector("#swap-route");
  if (swapButton) swapButton.addEventListener("click", () => {
    const from = document.querySelector("#from"),
      to = document.querySelector("#to");
    const fromTrigger = document.querySelector("#from-trigger"),
      toTrigger = document.querySelector("#to-trigger");
    const swap = document.querySelector("#swap-route");
    if (!from.value || !to.value || swap.dataset.swapping) return;
    swap.dataset.swapping = "true";
    [from.value, to.value] = [to.value, from.value];
    [fromTrigger.textContent, toTrigger.textContent] = [
      toTrigger.textContent,
      fromTrigger.textContent,
    ];
    [fromTrigger, toTrigger, swap].forEach((element) =>
      element.classList.add("is-swapping"),
    );
    window.setTimeout(() => {
      [fromTrigger, toTrigger, swap].forEach((element) =>
        element.classList.remove("is-swapping"),
      );
      delete swap.dataset.swapping;
    }, 280);
    scheduleDestinationOptions(from.value);
  });
}

function initTimePicker() {
  const input = document.querySelector("#search-time");
  const wrap = document.querySelector("#custom-time-wrap");
  if (!input || !wrap) return;
  const buttons = document.querySelectorAll("[data-time-mode]");
  const setMode = (mode) => {
    const custom = mode === "custom";
    input.disabled = !custom;
    wrap.hidden = !custom;
    if (custom && !input.value) input.value = jakartaTime(false).format();
    buttons.forEach((button) => {
      const active = button.dataset.timeMode === mode;
      button.classList.toggle("is-active", active);
      button.setAttribute("aria-pressed", String(active));
    });
    if (custom) input.focus();
  };
  buttons.forEach((button) =>
    button.addEventListener("click", () => setMode(button.dataset.timeMode)),
  );
}

function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function routeKey(route) {
  return `${String(route.from)}:${String(route.to)}`;
}

function validSavedRoutes() {
  return read(SAVED_KEY)
    .filter(
      (route) =>
        route &&
        /^\d+$/.test(String(route.from)) &&
        /^\d+$/.test(String(route.to)) &&
        String(route.from) !== String(route.to),
    )
    .slice(0, 10);
}

function dayName(offset) {
  if (offset === 0) return "Hari ini";
  if (offset === 1) return "Besok";
  if (offset === 2) return "Lusa";
  return `${offset} hari lagi`;
}

function savedMessage(container, message, actionLabel) {
  container.replaceChildren();
  const panel = element(
    "div",
    "saved-state rounded-2xl bg-white p-5 text-center text-slate-500 shadow-sm ring-1 ring-slate-200",
  );
  panel.appendChild(element("p", "", message));
  if (actionLabel) {
    const retry = element(
      "button",
      "mt-3 min-h-11 rounded-full border border-slate-200 px-4 text-sm font-bold text-blue-600",
      actionLabel,
    );
    retry.type = "button";
    retry.addEventListener("click", () => refreshSavedSchedules(true));
    panel.appendChild(retry);
  }
  container.appendChild(panel);
}

function renderSavedLoading(container, routes) {
  container.replaceChildren();
  routes.forEach((route) => {
    const card = element(
      "div",
      "saved-skeleton rounded-2xl bg-white p-4 shadow-sm ring-1 ring-slate-200",
    );
    card.setAttribute("aria-label", `Memuat ${route.fromName || "rute tersimpan"}`);
    card.appendChild(element("span", "block h-4 w-2/3 rounded bg-slate-200"));
    card.appendChild(element("span", "mt-3 block h-8 w-1/3 rounded bg-slate-200"));
    container.appendChild(card);
  });
}

function savedCard(route, fallback) {
  const card = element(
    "article",
    "saved-route-card rounded-2xl bg-white p-4 shadow-sm ring-1 ring-slate-200",
  );
  card.dataset.routeKey = routeKey(route);
  const header = element("div", "flex items-start justify-between gap-3");
  const title = element("div", "min-w-0");
  title.appendChild(
    element(
      "h2",
      "truncate font-black text-slate-900",
      route.label || `${route.from_name || fallback.fromName || route.from} → ${route.to_name || fallback.toName || route.to}`,
    ),
  );
  const remove = element(
    "button",
    "remove-saved min-h-11 min-w-11 rounded-full text-lg text-slate-500",
    "×",
  );
  remove.type = "button";
  remove.dataset.removeSaved = routeKey(route);
  remove.setAttribute("aria-label", "Hapus rute tersimpan");
  header.append(title, remove);
  card.appendChild(header);
  if (route.label) card.appendChild(element("p", "mt-1 text-xs text-slate-500", `${route.from_name || fallback.fromName} → ${route.to_name || fallback.toName}`));
  const rename = element("button", "mt-2 text-xs font-bold text-blue-600", route.label ? "Ubah nama" : "Beri nama");
  rename.type = "button";
  rename.dataset.renameSaved = routeKey(route);
  card.appendChild(rename);

  if (route.status === "ok" && route.next) {
    const schedule = element("div", "mt-3 flex items-end justify-between gap-3");
    const departure = element("div", "");
    departure.appendChild(
      element("time", "saved-departure block text-3xl font-black", route.next.departure),
    );
    departure.lastChild.dataset.clockTime = "";
    departure.appendChild(
      element(
        "p",
        "mt-1 text-xs font-bold uppercase tracking-wider text-slate-500",
        `${dayName(Number(route.next.day_offset || 0))} · KRL ${route.next.number}`,
      ),
    );
    const countdown = element("div", "text-right");
    countdown.appendChild(element("p", "text-xs text-slate-500", "Berangkat dalam"));
    const countdownValue = element("strong", "countdown-time block text-xl text-blue-600", "—");
    countdownValue.dataset.countdown = route.next.departure;
    countdownValue.dataset.dayOffset = String(route.next.day_offset || 0);
    countdown.appendChild(countdownValue);
    schedule.append(departure, countdown);
    card.appendChild(schedule);
    card.appendChild(
      element(
        "p",
        "mt-3 truncate text-xs text-slate-500",
        `${route.next.route} · tiba ${route.next.arrival} · ${route.next.duration_minutes} menit`,
      ),
    );
  } else {
    const messages = {
      invalid_route: "Rute tersimpan sudah tidak valid.",
      no_service: "Belum ada layanan terjadwal untuk rute ini.",
      error: "Jadwal rute ini belum dapat dimuat.",
    };
    card.appendChild(
      element("p", "mt-3 text-sm text-slate-500", messages[route.status] || messages.error),
    );
  }

  const open = element(
    "a",
    "mt-4 flex min-h-11 items-center justify-center rounded-xl bg-blue-600 px-4 text-sm font-bold text-white",
    "Buka jadwal",
  );
  open.href = `/search?from=${encodeURIComponent(route.from)}&to=${encodeURIComponent(route.to)}`;
  card.appendChild(open);
  return card;
}

function renderSavedRoutes(container, response, saved) {
  const fallbackByKey = new Map(saved.map((route) => [routeKey(route), route]));
  container.replaceChildren();
  (response.routes || []).forEach((route) => {
    container.appendChild(savedCard(route, fallbackByKey.get(routeKey(route)) || {}));
  });
  if (!container.children.length)
    savedMessage(container, "Belum ada rute tersimpan.");
  safelyInitialize(tick);
}

async function refreshSavedSchedules(force) {
  const container = document.querySelector("#saved-routes");
  if (!container) return;
  const saved = validSavedRoutes();
  if (!saved.length) {
    savedMessage(container, "Belum ada rute tersimpan. Simpan dari hasil pencarian.");
    return;
  }
  if (savedRefreshPromise && !force) return savedRefreshPromise;
  const requestID = ++savedRequest;
  renderSavedLoading(container, saved);
  const refreshButton = document.querySelector("#refresh-saved");
  if (refreshButton) {
    refreshButton.disabled = true;
    refreshButton.textContent = "Memuat…";
  }
  const controller =
    typeof AbortController === "undefined" ? null : new AbortController();
  const timeoutID = controller
    ? window.setTimeout(() => controller.abort(), 8000)
    : null;
  let currentPromise;
  const request = async () => {
    try {
      const response = await fetch("/api/saved-routes/schedules", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        signal: controller ? controller.signal : undefined,
        body: JSON.stringify({
          routes: saved.map((route) => ({
            from: Number(route.from),
            to: Number(route.to),
          })),
        }),
      });
      if (!response.ok) throw new Error(`request failed with ${response.status}`);
      const payload = await response.json();
      if (requestID === savedRequest) renderSavedRoutes(container, payload, saved);
    } catch (error) {
      if (requestID === savedRequest)
        savedMessage(container, "Jadwal tersimpan belum dapat dimuat.", "Coba lagi");
    } finally {
      if (timeoutID) window.clearTimeout(timeoutID);
      if (requestID === savedRequest && refreshButton) {
        refreshButton.disabled = false;
        refreshButton.textContent = "↻ Muat ulang";
      }
      if (savedRefreshPromise === currentPromise) savedRefreshPromise = null;
    }
  };
  currentPromise = request();
  savedRefreshPromise = currentPromise;
  return savedRefreshPromise;
}

function initSaved() {
  document.addEventListener("click", (event) => {
    const remove = event.target.closest("[data-remove-saved]");
    if (remove) {
      const saved = validSavedRoutes().filter(
        (route) => routeKey(route) !== remove.dataset.removeSaved,
      );
      write(SAVED_KEY, saved);
      refreshSavedSchedules(true);
      return;
    }
    const rename = event.target.closest("[data-rename-saved]");
    if (rename) {
      const saved = validSavedRoutes();
      const route = saved.find((item) => routeKey(item) === rename.dataset.renameSaved);
      if (!route) return;
      const label = window.prompt("Nama rute (opsional)", route.label || "");
      if (label === null) return;
      route.label = label.trim().slice(0, 32);
      write(SAVED_KEY, saved);
      refreshSavedSchedules(true);
      return;
    }
    const button = event.target.closest(".save-route");
    if (!button) return;
    const route = {
      from: button.dataset.from,
      to: button.dataset.to,
      fromName: stationName(button.dataset.from),
      toName: stationName(button.dataset.to),
    };
    let saved = read(SAVED_KEY);
    const exists = saved.some(
      (item) => item.from === route.from && item.to === route.to,
    );
    saved = exists
      ? saved.filter(
          (item) => !(item.from === route.from && item.to === route.to),
        )
      : [route, ...saved].slice(0, 10);
    write(SAVED_KEY, saved);
    button.textContent = exists ? "☆ Simpan rute" : "★ Tersimpan";
    button.setAttribute("aria-pressed", String(!exists));
  });
  const container = document.querySelector("#saved-routes");
  if (container) {
    refreshSavedSchedules(true);
    const refresh = document.querySelector("#refresh-saved");
    if (refresh) refresh.addEventListener("click", () => refreshSavedSchedules(true));
    document.addEventListener("visibilitychange", () => {
      if (!document.hidden) refreshSavedSchedules();
    });
  }
}
function updateOffline() {
  const banner = document.querySelector("#offline-banner");
  if (banner) banner.classList.toggle("hidden", navigator.onLine);
}
function setSearchLoading(form, loading) {
  const button = form.querySelector("#search-submit");
  if (!button) return;
  button.disabled = loading;
  button.textContent = loading ? "Mencari jadwal…" : button.dataset.label;
  form.setAttribute("aria-busy", String(loading));
}
function setSearchFeedback(form, message) {
  const feedback = form.querySelector("#search-feedback");
  if (!feedback) return;
  feedback.textContent = message || "";
  feedback.classList.toggle("hidden", !message);
}
function rememberRecentRoute(form) {
  const from = form.querySelector("#from")?.value;
  const to = form.querySelector("#to")?.value;
  if (!from || !to || from === to) return;
  const route = { from, to, fromName: stationName(from), toName: stationName(to) };
  const routes = read(RECENT_KEY).filter((item) => !(item.from === from && item.to === to));
  write(RECENT_KEY, [route, ...routes].slice(0, 5));
  renderRecentRoutes();
}
function renderRecentRoutes() {
  const section = document.querySelector("#recent-routes");
  if (!section) return;
  const routes = read(RECENT_KEY).filter((route) => route.from && route.to);
  section.hidden = routes.length === 0;
  const container = section.querySelector("div");
  if (!container) return;
  container.replaceChildren();
  routes.forEach((route) => {
    const link = element("a", "rounded-full border border-slate-200 bg-white px-3 py-2 text-sm font-semibold text-slate-700 shadow-sm dark:bg-slate-900 dark:text-slate-100", `${route.fromName || route.from} → ${route.toName || route.to}`);
    link.href = `/search?from=${encodeURIComponent(route.from)}&to=${encodeURIComponent(route.to)}`;
    container.appendChild(link);
  });
}
function resetSearchLoading() {
  document.querySelectorAll("form").forEach((form) => {
    if (form.querySelector("#search-submit")) setSearchLoading(form, false);
  });
}
document.body.addEventListener("htmx:beforeRequest", (event) => {
  const element = event.detail && event.detail.elt;
  const form = element && element.closest ? element.closest("form") : null;
  if (form && form.querySelector("#search-submit")) {
    setSearchFeedback(form, "");
    rememberRecentRoute(form);
    setSearchLoading(form, true);
  }
});
document.body.addEventListener("htmx:afterRequest", (event) => {
  const element = event.detail && event.detail.elt;
  const form = element && element.closest ? element.closest("form") : null;
  if (form && form.querySelector("#search-submit")) setSearchLoading(form, false);
});
document.body.addEventListener("htmx:sendError", resetSearchLoading);
document.body.addEventListener("htmx:sendError", (event) => {
  resetSearchLoading();
  const form = event.detail?.elt?.closest?.("form");
  if (form) setSearchFeedback(form, "Jadwal belum dapat dimuat. Periksa koneksi lalu coba lagi.");
});
document.body.addEventListener("htmx:responseError", (event) => {
  resetSearchLoading();
  const form = event.detail?.elt?.closest?.("form");
  if (form) setSearchFeedback(form, "Jadwal belum dapat dimuat. Periksa pilihan rute lalu coba lagi.");
});
document.addEventListener("click", (event) => {
  const detail = event.target.closest(".train-detail-link");
  if (detail) {
    sessionStorage.setItem("railnow.detail-return", "true");
    return;
  }
  const back = event.target.closest("[data-back-results]");
  if (!back || sessionStorage.getItem("railnow.detail-return") !== "true") return;
  const referrer = new URL(document.referrer || location.href);
  if (referrer.origin !== location.origin || referrer.pathname !== "/search") return;
  event.preventDefault();
  sessionStorage.removeItem("railnow.detail-return");
  history.back();
});
function safelyInitialize(callback) {
  try {
    callback();
  } catch (error) {
    // Keep independent controls usable if a browser blocks one optional API.
    console.warn("RuteKRL enhancement unavailable", error);
  }
}
safelyInitialize(tick);
setInterval(() => safelyInitialize(tick), 1000);
safelyInitialize(initStations);
safelyInitialize(initTimePicker);
safelyInitialize(initSaved);
safelyInitialize(renderRecentRoutes);
safelyInitialize(updateOffline);
window.addEventListener("online", updateOffline);
window.addEventListener("offline", updateOffline);
window.addEventListener("pageshow", resetSearchLoading);
// Timetables should always be fetched from the server. Remove the legacy
// worker so an earlier deployment cannot keep serving stale scripts or pages.
if ("serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker
      .getRegistrations()
      .then((registrations) => Promise.all(registrations.map((registration) => registration.unregister())))
      .then(() => ("caches" in window ? caches.keys() : []))
      .then((keys) => Promise.all(keys.map((key) => caches.delete(key))))
      .catch((error) => console.warn("RuteKRL cache cleanup unavailable", error));
  });
}
