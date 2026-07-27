import { expect, test } from "@playwright/test";

const savedReport = (date: string, runID: number) => ({
  run_id: runID,
  file_name: `${date}-MaiTuo.xlsx`,
  file_sha256: `hash-${date}`,
  report_date: date,
  fetched: 250,
  present_sheets: ["总览KPI", "笔记明细", "分SPU总览", "分子账户", "淘搜趋势"],
  missing_sheets: [],
  completed_at: "2026-07-26T00:42:00+08:00"
});

test("labels Friday and Saturday gaps as business weekends", async ({ page }) => {
  await page.route("**/paipai/api/imports/maituo-customer-daily", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: [savedReport("2026-07-05", 2), savedReport("2026-07-02", 1)] })
    });
  });

  await page.goto("/paipai/maituo-daily-report");
  const rows = page.locator(".saved-table tbody tr");
  await expect(rows).toHaveCount(4);
  await expect(rows.nth(1)).toContainText("2026-07-04");
  await expect(rows.nth(1)).toContainText("周六");
  await expect(rows.nth(1)).toContainText("无需日报");
  await expect(rows.nth(2)).toContainText("2026-07-03");
  await expect(rows.nth(2)).toContainText("周五");
  await expect(rows.nth(2)).toContainText("无需日报");
  await expect(page.getByText("缺少报表")).toHaveCount(0);
});

test("flags a missing non-weekend date", async ({ page }) => {
  await page.route("**/paipai/api/imports/maituo-customer-daily", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: [savedReport("2026-07-08", 2), savedReport("2026-07-06", 1)] })
    });
  });

  await page.goto("/paipai/maituo-daily-report");
  const missing = page.locator(".calendar-row.missing");
  await expect(missing).toContainText("2026-07-07");
  await expect(missing).toContainText("周二");
  await expect(missing).toContainText("缺少报表");
});
