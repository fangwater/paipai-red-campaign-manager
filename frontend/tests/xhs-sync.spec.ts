import { expect, test } from "@playwright/test";

const recent = [
  {
    run_id: 18, mode: "full", target: "creativities", trigger_type: "api", status: "succeeded",
    advertisers_count: 1, campaigns_count: 0, units_count: 0, creativities_count: 443,
    deactivated_count: 0, started_at: "2026-07-24T11:15:11+08:00", finished_at: "2026-07-24T11:15:35+08:00"
  },
  {
    run_id: 17, mode: "incremental", target: "units", trigger_type: "api", status: "succeeded",
    advertisers_count: 1, campaigns_count: 0, units_count: 12, creativities_count: 0,
    deactivated_count: 0, started_at: "2026-07-24T11:13:02+08:00", finished_at: "2026-07-24T11:13:03+08:00"
  },
  {
    run_id: 16, mode: "incremental", target: "campaigns", trigger_type: "api", status: "succeeded",
    advertisers_count: 1, campaigns_count: 8, units_count: 0, creativities_count: 0,
    deactivated_count: 0, started_at: "2026-07-24T11:11:31+08:00", finished_at: "2026-07-24T11:11:32+08:00"
  }
];

test("opens real Spotlight sync routes and triggers the selected mode", async ({ page }) => {
  let triggerBody: unknown;
  await page.route("**/paipai/api/xhs-jg/sync/status", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: { running: false, recent } }) });
  });
  await page.route("**/paipai/api/xhs-jg/sync/campaigns", async (route) => {
    triggerBody = route.request().postDataJSON();
    await route.fulfill({
      status: 202,
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: { ...recent[2], run_id: 19, mode: "full", status: "running", started_at: "2026-07-26T12:00:00+08:00", finished_at: undefined } })
    });
  });

  await page.goto("/paipai/");
  await page.getByRole("button", { name: "推广计划", exact: true }).click();
  await expect(page).toHaveURL(/\/paipai\/xhs-jg-sync\/campaigns$/);
  await expect(page.getByRole("heading", { name: "聚光数据同步" })).toBeVisible();
  await expect(page.locator(".sync-target")).toHaveCount(3);
  await expect(page.locator(".sync-history tbody tr")).toHaveCount(3);
  await expect(page.getByText("http", { exact: false })).toHaveCount(0);

  await page.getByRole("button", { name: "广告单元", exact: true }).click();
  await expect(page).toHaveURL(/\/paipai\/xhs-jg-sync\/units$/);
  await expect(page.locator(".sync-target.active")).toContainText("广告单元");
  await page.getByRole("button", { name: "推广计划", exact: true }).click();

  const campaign = page.locator(".sync-target").filter({ has: page.getByRole("heading", { name: "推广计划", exact: true }) });
  await campaign.getByRole("button", { name: "全量", exact: true }).click();
  await campaign.getByRole("button", { name: "同步计划", exact: true }).click();
  await expect.poll(() => triggerBody).toEqual({ mode: "full" });
  await expect(page.locator(".sync-running-banner")).toContainText("推广计划正在同步");
});
