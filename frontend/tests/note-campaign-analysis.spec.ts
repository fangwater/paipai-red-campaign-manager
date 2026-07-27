import { expect, test } from "@playwright/test";

const points = [
  { report_date: "2026-07-21", spend: 10, search_users: 2, search_cost: 5, cumulative_spend: 10, cumulative_search_users: 2 },
  { report_date: "2026-07-22", spend: 0, search_users: 0, search_cost: 0, cumulative_spend: 10, cumulative_search_users: 2 },
  { report_date: "2026-07-23", spend: 20, search_users: 4, search_cost: 5, cumulative_spend: 30, cumulative_search_users: 6 }
];

test("renders cumulative ECharts and switches the note campaign key", async ({ page }) => {
  const requestedWindows: string[] = [];
  const requestedSorts: string[] = [];
  await page.route("**/paipai/api/analytics/maituo/note-campaigns?*", async (route) => {
    const url = new URL(route.request().url());
    requestedWindows.push(url.searchParams.get("window") ?? "");
    requestedSorts.push(url.searchParams.get("sort") || "");
    const windowOption = url.searchParams.get("window") ?? "7d";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          window: windowOption,
          sort: url.searchParams.get("sort") || "cumulative_spend",
          report_dates: points.map((point) => point.report_date),
          total: 2,
          page: 1,
          page_size: 25,
          items: [
            { note_id: "note-a", campaign_name: "磷虾油搜索计划", placement: "搜索", first_report_date: "2026-07-21", last_report_date: "2026-07-23", active_days: 2, latest_spend: 20, total_spend: 30, total_search_users: 6, latest_search_cost: 5, points },
            { note_id: "note-b", campaign_name: "辅酶信息流计划", placement: "信息流", first_report_date: "2026-07-21", last_report_date: "2026-07-23", active_days: 3, latest_spend: 8, total_spend: 18, total_search_users: 3, latest_search_cost: 4, points: points.map((point) => ({ ...point, cumulative_spend: point.cumulative_spend * 0.6, cumulative_search_users: Math.round(point.cumulative_search_users * 0.5) })) }
          ]
        }
      })
    });
  });

  await page.goto("/paipai/note-campaign-analysis");
  await expect(page.getByRole("heading", { name: "笔记计划分析" })).toBeVisible();
  await expect(page.locator(".metric-chart")).toHaveCount(3);
  await expect(page.getByText("累计回搜成本", { exact: true })).toHaveCount(0);
  await expect(page.getByText("回搜成本", { exact: true })).toBeVisible();
  await expect(page.locator(".focus-identity")).toContainText("磷虾油搜索计划");
  await expect(page.locator(".analysis-table tbody tr")).toHaveCount(2);
  await expect(page.getByText("http", { exact: false })).toHaveCount(0);

  await expect.poll(async () => page.locator(".metric-chart canvas").evaluateAll((canvases) => canvases.map((canvas) => {
    const element = canvas as HTMLCanvasElement;
    const context = element.getContext("2d");
    if (!context || element.width === 0 || element.height === 0) return false;
    const pixels = context.getImageData(0, 0, element.width, element.height).data;
    for (let index = 3; index < pixels.length; index += 4) if (pixels[index] > 0) return true;
    return false;
  }))).toEqual([true, true, true]);

  await page.locator(".analysis-table tbody tr").nth(1).click();
  await expect(page.locator(".focus-identity")).toContainText("辅酶信息流计划");
  await page.getByRole("button", { name: "3D" }).click();
  await expect.poll(() => requestedWindows).toContain("3d");
  await page.getByRole("button", { name: "当天消耗" }).click();
  await expect.poll(() => requestedSorts).toContain("daily_spend");
});
