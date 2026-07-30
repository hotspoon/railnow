import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const route = {
  from: "5",
  to: "7",
  fromName: "<img src=x onerror=alert(1)>",
  toName: "Juanda",
};

function page() {
  document.body.innerHTML = `
    <button id="refresh-saved" type="button">↻ Refresh</button>
    <div id="saved-routes"></div>
    <span id="live-clock"></span>`;
}

function response() {
  return {
    ok: true,
    json: async () => ({
      data_status: "available",
      routes: [
        {
          from: 5,
          to: 7,
          from_name: "Bogor",
          to_name: "Juanda",
          status: "ok",
          next: {
            train_id: 1,
            number: "BGR-1001",
            route: "Bogor — Jakarta Kota",
            departure: "07:32",
            arrival: "08:25",
            duration_minutes: 53,
            day_offset: 1,
          },
        },
      ],
    }),
  };
}

async function loadApp() {
  vi.resetModules();
  await import("../../public/js/app.js");
  await vi.runAllTicks();
}

describe("live saved routes", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    localStorage.clear();
    page();
    localStorage.setItem("railnow.savedRoutes", JSON.stringify([route]));
    global.fetch = vi.fn(() => Promise.resolve(response()));
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("loads a live card with a day-aware countdown", async () => {
    await loadApp();
    await vi.runAllTicks();

    expect(global.fetch).toHaveBeenCalledWith(
      "/api/saved-routes/schedules",
      expect.objectContaining({ method: "POST" }),
    );
    expect(document.querySelector(".saved-route-card").textContent).toContain(
      "Bogor → Juanda",
    );
    expect(document.querySelector("[data-countdown]").dataset.dayOffset).toBe("1");
    expect(document.querySelector(".saved-route-card").textContent).toContain(
      "Train BGR-1001",
    );
  });

  it("renders stored names as text and removes a route", async () => {
    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        routes: [
          {
            from: 5,
            to: 7,
            status: "no_service",
          },
        ],
      }),
    });
    await loadApp();
    await vi.runAllTicks();

    expect(document.querySelector("#saved-routes img")).toBeNull();
    expect(document.querySelector(".saved-route-card").textContent).toContain(
      "<img src=x onerror=alert(1)>",
    );
    document.querySelector("[data-remove-saved]").click();
    await vi.runAllTicks();
    expect(JSON.parse(localStorage.getItem("railnow.savedRoutes"))).toEqual([]);
    expect(document.querySelector("#saved-routes").textContent).toContain(
      "No saved routes yet",
    );
  });

  it("shows a retry action after an API failure", async () => {
    global.fetch.mockRejectedValueOnce(new Error("offline"));
    await loadApp();
    await vi.runAllTicks();

    expect(document.querySelector("#saved-routes").textContent).toContain(
      "Jadwal tersimpan belum dapat dimuat",
    );
    expect(document.querySelector("#saved-routes button").textContent).toBe(
      "Coba lagi",
    );
  });
});
