const SAVED_KEY = "railnow.savedRoutes";
const RECENT_KEY = "railnow.recentStations";
let allowedDestinations = null;

function jakartaTime() {
  return new Intl.DateTimeFormat("en-GB", {
    timeZone: "Asia/Jakarta",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  });
}
function jakartaSeconds() {
  const parts = jakartaTime().formatToParts();
  const value = (type) =>
    Number(parts.find((part) => part.type === type)?.value || 0);
  return value("hour") * 3600 + value("minute") * 60 + value("second");
}
function tick() {
  const clock = document.querySelector("#live-clock");
  if (clock) clock.textContent = `${jakartaTime().format()} WIB`;
  document.querySelectorAll("[data-countdown]").forEach((el) => {
    const [h, m] = el.dataset.countdown.split(":").map(Number);
    let diff = h * 3600 + m * 60 - jakartaSeconds();
    if (el.dataset.nextDay === "true" || diff < 0) diff += 24 * 3600;
    el.textContent = `${String(Math.floor(diff / 60)).padStart(2, "0")}:${String(diff % 60).padStart(2, "0")}`;
  });
}
function read(key) {
  try {
    const value = JSON.parse(localStorage.getItem(key));
    return Array.isArray(value) ? value : [];
  } catch {
    return [];
  }
}
function write(key, value) {
  localStorage.setItem(key, JSON.stringify(value));
}
function stationName(id) {
  return (
    document.querySelector(`[data-station-id="${id}"]`)?.dataset.stationName ||
    id
  );
}
function setStation(target, id, name) {
  document.querySelector(`#${target}`).value = id;
  const trigger = document.querySelector(`#${target}-trigger`);
  if (trigger) trigger.textContent = name;
  document.querySelector(`#${target}-query`).value = "";
  document.querySelector(`#${target}-options`).hidden = true;
  const recents = read(RECENT_KEY).filter((station) => station.id !== id);
  write(RECENT_KEY, [{ id, name }, ...recents].slice(0, 5));
  if (target === "from") updateDestinationOptions(id);
}
async function updateDestinationOptions(from) {
  const trigger = document.querySelector("#to-trigger");
  const swap = document.querySelector("#swap-route");
  const originalLabel = trigger?.textContent;
  if (trigger) {
    trigger.disabled = true;
    trigger.textContent = "Loading destinations…";
  }
  if (swap) swap.disabled = true;
  try {
    const response = await fetch(
      `/stations/destination-options?from=${encodeURIComponent(from)}`,
    );
    if (!response.ok) return;
    const destinations = await response.json();
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
  } catch {
    /* Keep the currently rendered options when offline. */
  } finally {
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
    if (target === "from" && selected) updateDestinationOptions(selected);
  });
  document.querySelectorAll(".station-query").forEach((input) => {
    const target = input.dataset.target;
    input.addEventListener("input", () => {
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
    });
  });
  document.addEventListener("click", (event) => {
    const option = event.target.closest("[data-station-id]");
    if (!option) return;
    const target = option
      .closest(".station-options")
      .id.replace("-options", "");
    setStation(target, option.dataset.stationId, option.dataset.stationName);
  });
  document.querySelector("#swap-route")?.addEventListener("click", () => {
    const from = document.querySelector("#from"),
      to = document.querySelector("#to");
    const fromTrigger = document.querySelector("#from-trigger"),
      toTrigger = document.querySelector("#to-trigger");
    [from.value, to.value] = [to.value, from.value];
    [fromTrigger.textContent, toTrigger.textContent] = [
      toTrigger.textContent,
      fromTrigger.textContent,
    ];
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
  document
    .querySelector("#offline-banner")
    ?.classList.toggle("hidden", navigator.onLine);
}
function setSearchLoading(form, loading) {
  const button = form.querySelector("#search-submit");
  if (!button) return;
  button.disabled = loading;
  button.textContent = loading ? "Finding trains…" : button.dataset.label;
  form.setAttribute("aria-busy", String(loading));
}
document.body.addEventListener("htmx:beforeRequest", (event) => {
  const form = event.detail.elt?.closest("form");
  if (form?.querySelector("#search-submit")) setSearchLoading(form, true);
});
document.body.addEventListener("htmx:afterRequest", (event) => {
  const form = event.detail.elt?.closest("form");
  if (form?.querySelector("#search-submit")) setSearchLoading(form, false);
});
tick();
setInterval(tick, 1000);
initStations();
initSaved();
updateOffline();
window.addEventListener("online", updateOffline);
window.addEventListener("offline", updateOffline);
if ("serviceWorker" in navigator)
  window.addEventListener("load", () =>
    navigator.serviceWorker.register("/sw.js"),
  );
