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

const savedReportHistory = (count: number) => Array.from({ length: count }, (_, index) => {
  const date = new Date(Date.UTC(2026, 7, 20 - index)).toISOString().slice(0, 10);
  return savedReport(date, count - index);
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

test("opens a saved report as a note and placement merged table", async ({ page }) => {
  let requestedDate = "";
  await page.route("**/paipai/api/imports/maituo-customer-daily*", async (route) => {
    const reportDate = new URL(route.request().url()).searchParams.get("report_date");
    if (!reportDate) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          success: true,
          data: savedReportHistory(45).map((report, index) => index === 0 ? { ...report, merged_rows: 87 } : report)
        })
      });
      return;
    }
    requestedDate = reportDate;
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          report_date: reportDate,
          total: 30,
          items: Array.from({ length: 30 }, (_, index) => ({
            note_id: index === 0 ? "note-merged" : `note-merged-${index}`,
            note_url: "https://www.xiaohongshu.com/explore/note-merged",
            category: "辅酶",
            placement: "搜索",
            keyword_category_note: "品牌词",
            spend: 120.5,
            search_users: 12,
            search_cost: 10.04,
            estimated_postback_cost: 6.33,
            search_rate_pct: 4.2,
            cpc: 0.75,
            ctr_pct: 1.25,
            subaccount: "不应展示的子账户",
            campaign_name: "不应展示的计划"
          }))
        }
      })
    });
  });

  await page.goto("/paipai/maituo-daily-report");
  await expect(page.getByRole("columnheader", { name: "合并行数" })).toBeVisible();
  await expect(page.locator(".saved-report-row td").nth(3)).toHaveText("87");
  await page.locator(".saved-report-row").first().click();
  await expect.poll(() => requestedDate).toBe("2026-08-20");

  const detail = page.locator("#saved-report-detail");
  await expect(detail).toBeFocused();
  const table = page.getByRole("table", { name: "2026-08-20 合并笔记明细" });
  await expect(table).toBeVisible();
  await expect.poll(() => detail.evaluate((element) => {
    const bounds = element.getBoundingClientRect();
    const topbarBottom = document.querySelector(".topbar")?.getBoundingClientRect().bottom ?? 0;
    return bounds.top >= topbarBottom && bounds.top <= topbarBottom + 52;
  })).toBe(true);

  await expect(table.locator("tbody tr")).toHaveCount(30);
  await expect(table).toContainText("note-merged");
  await expect(table).toContainText("搜索");
  await expect(table).toContainText("¥120.50");
  await expect(table.getByRole("columnheader", { name: "子账户", exact: true })).toHaveCount(0);
  await expect(table.getByRole("columnheader", { name: "计划", exact: true })).toHaveCount(0);
  await expect(table).not.toContainText("不应展示的子账户");
  await expect(table).not.toContainText("不应展示的计划");
  await expect(page.getByRole("heading", { name: "子账户文件目录" })).toHaveCount(0);
});

test("shows an error when a saved report detail cannot be loaded", async ({ page }) => {
  await page.route("**/paipai/api/imports/maituo-customer-daily*", async (route) => {
    const reportDate = new URL(route.request().url()).searchParams.get("report_date");
    if (!reportDate) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ success: true, data: [savedReport("2026-08-20", 59)] })
      });
      return;
    }
    await route.fulfill({
      status: 502,
      contentType: "application/json",
      body: JSON.stringify({ success: false, error: "历史明细暂不可用" })
    });
  });

  await page.goto("/paipai/maituo-daily-report");
  const reportRow = page.locator(".saved-report-row");
  await reportRow.focus();
  await reportRow.press("Enter");
  await expect(page.getByRole("alert")).toContainText("历史明细暂不可用");
  await expect(page.locator("#saved-report-detail")).toBeFocused();
  await expect(page.getByRole("table", { name: /合并笔记明细/ })).toHaveCount(0);
});
