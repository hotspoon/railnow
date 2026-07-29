const SAVED_KEY = "railnow.savedRoutes";
const RECENT_KEY = "railnow.recentStations";
let allowedDestinations = null;
let destinationRequest = 0;
let destinationTimer = null;

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

function jakartaTime() {
  try {
    return new Intl.DateTimeFormat("en-GB", {
      timeZone: "Asia/Jakarta",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hourCycle: "h23",
    });
  } catch (error) {
    return new Intl.DateTimeFormat("en-GB", {
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  }
}
function jakartaSeconds() {
  const parts = jakartaTime().formatToParts();
  const value = (type) => {
    const part = parts.find((candidate) => candidate.type === type);
    return Number(part ? part.value : 0);
  };
  return value("hour") * 3600 + value("minute") * 60 + value("second");
}
function tick() {
  const clock = document.querySelector("#live-clock");
  if (clock) clock.textContent = `${jakartaTime().format()} WIB`;
  document.querySelectorAll("[data-countdown]").forEach((el) => {
    const [h, m] = el.dataset.countdown.split(":").map(Number);
    let diff = h * 3600 + m * 60 - jakartaSeconds();
    if (el.dataset.nextDay === "true" || diff < 0) diff += 24 * 3600;
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
  const recents = read(RECENT_KEY).filter((station) => station.id !== id);
  write(RECENT_KEY, [{ id, name }, ...recents].slice(0, 5));
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
    trigger.textContent = "Loading destinations…";
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
      if (trigger.textContent === "Loading destinations…")
        trigger.textContent = originalLabel || "Select station";
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
    if (selected) trigger.textContent = stationName(selected);
    trigger.addEventListener("click", () => {
      document.querySelectorAll(".station-options").forEach((panel) => {
        if (panel.id !== `${target}-options`) panel.hidden = true;
      });
      const panel = document.querySelector(`#${target}-options`);
      panel.hidden = !panel.hidden;
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
  });
  document.addEventListener("click", (event) => {
    const option = event.target.closest("[data-station-id]");
    if (!option) return;
    const target = option
      .closest(".station-options")
      .id.replace("-options", "");
    setStation(target, option.dataset.stationId, option.dataset.stationName);
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
function initSaved() {
  document.addEventListener("click", (event) => {
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
    button.textContent = exists ? "☆ Save route" : "★ Saved";
  });
  const container = document.querySelector("#saved-routes");
  if (container) {
    const saved = read(SAVED_KEY);
    container.innerHTML = saved.length
      ? saved
          .map(
            (route) =>
              `<a class="block rounded-2xl bg-white p-4 shadow-sm ring-1 ring-slate-200" href="/search?from=${encodeURIComponent(route.from)}&to=${encodeURIComponent(route.to)}">★ ${route.fromName} → ${route.toName}</a>`,
          )
          .join("")
      : '<p class="text-slate-500">No saved routes yet.</p>';
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
  button.textContent = loading ? "Finding trains…" : button.dataset.label;
  form.setAttribute("aria-busy", String(loading));
}
function resetSearchLoading() {
  document.querySelectorAll("form").forEach((form) => {
    if (form.querySelector("#search-submit")) setSearchLoading(form, false);
  });
}
document.body.addEventListener("htmx:beforeRequest", (event) => {
  const element = event.detail && event.detail.elt;
  const form = element && element.closest ? element.closest("form") : null;
  if (form && form.querySelector("#search-submit")) setSearchLoading(form, true);
});
document.body.addEventListener("htmx:afterRequest", (event) => {
  const element = event.detail && event.detail.elt;
  const form = element && element.closest ? element.closest("form") : null;
  if (form && form.querySelector("#search-submit")) setSearchLoading(form, false);
});
document.body.addEventListener("htmx:sendError", resetSearchLoading);
document.body.addEventListener("htmx:responseError", resetSearchLoading);
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
    console.warn("RailNow enhancement unavailable", error);
  }
}
safelyInitialize(tick);
setInterval(() => safelyInitialize(tick), 1000);
safelyInitialize(initStations);
safelyInitialize(initSaved);
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
      .catch((error) => console.warn("RailNow cache cleanup unavailable", error));
  });
}
