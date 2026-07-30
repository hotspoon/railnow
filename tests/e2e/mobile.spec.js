import { expect, test } from "@playwright/test";

const savedRoute = {
  from: "5",
  to: "7",
  fromName: "Bogor",
  toName: "Juanda",
};

test("custom time, update status, and mobile layout stay precise", async ({
  page,
}) => {
  await page.goto("/search?from=5&to=7&time=07%3A30");

  await expect(page.locator("#search-time")).toHaveValue("07:30");
  await expect(page.locator('[data-time-mode="custom"]')).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(page.locator(".schedule-status summary")).toContainText(
    "Data diperbarui",
  );
  await expect(page.locator(".next-train-card .departure-time")).toHaveText(
    "07:32",
  );
  await expect(page.locator(".next-train-card")).toHaveAttribute(
    "href",
    "/train/1?from=5&to=7&time=07%3A30",
  );

  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - window.innerWidth,
  );
  expect(overflow).toBeLessThanOrEqual(1);
});

test("saved route shows its next schedule and can be removed", async ({
  page,
}) => {
  await page.goto("/search?from=5&to=7&time=07%3A30");
  await page.locator(".save-route").click();
  await expect(page.locator(".save-route")).toHaveText("★ Saved");

  await page.goto("/saved");
  const card = page.locator(".saved-route-card");
  await expect(card).toContainText("Bogor → Juanda");
  await expect(card.locator(".saved-departure")).toHaveText(/^\d{2}:\d{2}$/);
  await expect(card).toContainText(/Train BGR-/);

  await page.reload();
  await expect(page.locator(".saved-route-card")).toContainText(
    "Bogor → Juanda",
  );
  await page.locator("[data-remove-saved]").click();
  await expect(page.locator("#saved-routes")).toContainText(
    "No saved routes yet",
  );
});

test("saved routes provide a retry state when the schedule request fails", async ({
  page,
}) => {
  await page.addInitScript((route) => {
    localStorage.setItem("railnow.savedRoutes", JSON.stringify([route]));
  }, savedRoute);
  await page.route("**/api/saved-routes/schedules", (route) => route.abort());
  await page.goto("/saved");

  await expect(page.locator("#saved-routes")).toContainText(
    "Jadwal tersimpan belum dapat dimuat",
  );
  await expect(page.getByRole("button", { name: "Coba lagi" })).toBeVisible();
});

test("invalid custom time returns a clear client error", async ({ request }) => {
  const response = await request.get("/search?from=5&to=7&time=25%3A90");
  expect(response.status()).toBe(400);
  expect(await response.text()).toContain("HH:MM");
});
