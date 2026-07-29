import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "../../public/css/station-select.css";

const station = (id, name, code) =>
  `<button type="button" data-station-id="${id}" data-station-name="${name}">${name} <span>${code}</span></button>`;

function page() {
  document.body.innerHTML = `
    <form>
      <button type="button" id="from-trigger" class="station-trigger" data-target="from">Manggarai</button>
      <input id="from" value="1" />
      <div id="from-options" class="station-options" hidden>
        <input id="from-query" class="station-query" data-target="from" />
        ${station(1, "Manggarai", "MRI")}
        ${station(2, "Buaran", "BUA")}
        ${station(3, "Bogor", "BOO")}
      </div>
      <button type="button" id="swap-route">Swap</button>
      <button type="button" id="to-trigger" class="station-trigger" data-target="to">Bogor</button>
      <input id="to" value="3" />
      <div id="to-options" class="station-options" hidden>
        <input id="to-query" class="station-query" data-target="to" />
        ${station(2, "Buaran", "BUA")}
        ${station(3, "Bogor", "BOO")}
        ${station(4, "Cikarang", "CKR")}
      </div>
      <button id="search-submit" data-label="Find next train">Find next train</button>
    </form>
    <div id="offline-banner" class="hidden"></div>
    <span id="live-clock"></span>
    <time data-clock-time>05:13:30</time>
    <strong data-countdown="05:13:30" data-next-day="false">--:--</strong>`;
}

function response(stations) {
  return { ok: true, json: async () => stations };
}

async function loadPicker() {
  vi.resetModules();
  await import("../../public/js/app.js");
}

describe("station picker", () => {
  beforeEach(async () => {
    vi.useFakeTimers();
    page();
    global.fetch = vi.fn(() => Promise.resolve(response([{ ID: 3, Name: "Bogor" }])));
    await loadPicker();
    await vi.advanceTimersByTimeAsync(180);
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("filters Buaran immediately while typing", () => {
    const query = document.querySelector("#from-query");
    query.value = "buara";
    query.dispatchEvent(new Event("input", { bubbles: true }));

    expect(document.querySelector('#from-options [data-station-id="2"]').hidden).toBe(false);
    expect(document.querySelector('#from-options [data-station-id="1"]').hidden).toBe(true);
    expect(document.querySelector('#from-options [data-station-id="3"]').hidden).toBe(true);
    expect(
      getComputedStyle(
        document.querySelector('#from-options [data-station-id="1"]'),
      ).display,
    ).toBe("none");
  });

  it("normalizes timetable seconds for compact mobile cards", () => {
    expect(document.querySelector("[data-clock-time]").textContent).toBe("05:13");
    expect(document.querySelector("[data-countdown]").textContent).not.toContain(":");
  });

  it("only exposes direct destinations after choosing an origin", async () => {
    const buaran = document.querySelector('#from-options [data-station-id="2"]');
    buaran.click();
    await vi.advanceTimersByTimeAsync(180);

    expect(global.fetch).toHaveBeenLastCalledWith(
      "/stations/destination-options?from=2",
      expect.anything(),
    );
    expect(document.querySelector('#to-options [data-station-id="3"]').hidden).toBe(false);
    expect(document.querySelector('#to-options [data-station-id="2"]').hidden).toBe(true);
    expect(document.querySelector('#to-options [data-station-id="4"]').hidden).toBe(true);
    expect(
      getComputedStyle(
        document.querySelector('#to-options [data-station-id="4"]'),
      ).display,
    ).toBe("none");
    expect(document.querySelector("#to-trigger").disabled).toBe(false);
  });

  it("re-enables the destination controls when its request fails", async () => {
    global.fetch.mockClear();
    global.fetch.mockRejectedValueOnce(new Error("network unavailable"));

    document.querySelector('#from-options [data-station-id="2"]').click();
    await vi.advanceTimersByTimeAsync(180);

    expect(document.querySelector("#to-trigger").disabled).toBe(false);
    expect(document.querySelector("#swap-route").disabled).toBe(false);
    expect(document.querySelector("#to-trigger").textContent).toBe("Bogor");
  });
});
