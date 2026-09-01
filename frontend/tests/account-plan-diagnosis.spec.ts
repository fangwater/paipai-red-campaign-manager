import { expect, test } from "@playwright/test";

const overviewPoints = Array.from({ length: 30 }, (_, index) => {
  const date = new Date(Date.UTC(2026, 5, 28 + index)).toISOString().slice(0, 10);
  const searchSpend = 4200 + index * 55;
  const feedSpend = 1100 + index * 18;
  return {
    report_date: date,
    total_spend: searchSpend + feedSpend,
    search_spend: searchSpend,
    search_cost: 31 + index * 0.28,
    search_cpc: 2.18 - index * 0.011,
    search_ctr_pct: 3.42 + index * 0.031,
    search_rate_pct: 9.6 + index * 0.09,
    feed_spend: feedSpend,
    feed_cost: 68 + index * 0.35,
    feed_cpc: 2.74 - index * 0.014,
    feed_ctr_pct: 2.81 + index * 0.026,
    feed_search_rate_pct: 6.4 + index * 0.07,
  };
});

test("renders independent subaccount overview and placement diagnostics", async ({ page }) => {
  await page.route("**/paipai/api/analytics/maituo/account-plan-diagnosis?*", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          report_date: "2026-07-27",
          spu: "辅酶",
          account_kpi: 70,
          account_overviews: [
            { account: "Megared脉拓-飓风03", current_total_spend: 7400, points: overviewPoints },
            {
              account: "Megared脉拓-智元01",
              current_total_spend: 6200,
              points: overviewPoints.map((point) => ({
                ...point,
                total_spend: point.total_spend * 0.84,
              }))
            }
          ],
          accounts: [{
            account: "Megared脉拓-飓风03",
            placement: "搜索",
            spend: 5804.76,
            search_users: 148,
            original_cost: 39.22,
            correction_coefficient: null,
            cost: 39.22,
            search_rate_pct: 12.45,
            cpc: 1.86,
            ctr_pct: 4.32,
            note_count: 17,
            cost_metric: "回搜成本",
            previous_cost: 34.86,
            change_pct: 0.125,
            kpi: 70,
            status: "good",
            points: [
              { report_date: "2026-07-21", spend: 5010.12, search_users: 109, original_cost: 45.74, correction_coefficient: null, cost: 45.74, search_rate_pct: 10.21, cpc: 2.12, ctr_pct: 3.71, note_count: 14 },
              { report_date: "2026-07-22", spend: 5232.44, search_users: 154, original_cost: 33.98, correction_coefficient: null, cost: 33.98, search_rate_pct: 11.08, cpc: 2.03, ctr_pct: 3.92, note_count: 15 },
              { report_date: "2026-07-23", spend: 5483.91, search_users: 148, original_cost: 37.05, correction_coefficient: null, cost: 37.05, search_rate_pct: 11.63, cpc: 1.98, ctr_pct: 4.01, note_count: 15 },
              { report_date: "2026-07-24", spend: null, search_users: null, original_cost: null, correction_coefficient: null, cost: null, search_rate_pct: null, cpc: null, ctr_pct: null, note_count: null },
              { report_date: "2026-07-25", spend: null, search_users: null, original_cost: null, correction_coefficient: null, cost: null, search_rate_pct: null, cpc: null, ctr_pct: null, note_count: null },
              { report_date: "2026-07-26", spend: 5612.77, search_users: 161, original_cost: 34.86, correction_coefficient: null, cost: 34.86, search_rate_pct: 12.02, cpc: 1.92, ctr_pct: 4.13, note_count: 16 },
              { report_date: "2026-07-27", spend: 5804.76, search_users: 148, original_cost: 39.22, correction_coefficient: null, cost: 39.22, search_rate_pct: 12.45, cpc: 1.86, ctr_pct: 4.32, note_count: 17 }
            ]
          }]
        }
      })
    });
  });

  await page.goto("/paipai/account-plan-diagnosis");
  await expect(page.getByRole("heading", { name: "子账户诊断" })).toBeVisible();
  await expect(page.locator(".diagnosis-page-heading")).toContainText("分子账户独立汇总");
  await expect(page.getByRole("heading", { name: "子账户数据总览" })).toBeVisible();
  const accountSelect = page.getByRole("combobox", { name: "选择子账户" });
  await expect(accountSelect).toHaveValue("Megared脉拓-飓风03");
  await expect(accountSelect.locator("option")).toHaveCount(2);
  await expect(page.locator(".diagnosis-trend-card")).toHaveCount(6);
  await expect(page.getByRole("heading", { name: "综合加权回搜重合系数" })).toHaveCount(0);
  await expect(page.getByRole("img", { name: "总消耗趋势图" })).toBeVisible();
  await expect(page.getByRole("img", { name: "信息流消耗与回搜成本趋势图" })).toBeVisible();
  await expect(page.getByRole("img", { name: "搜索 / 信息流回搜率趋势图" })).toBeVisible();
  await expect.poll(async () => page.locator(".diagnosis-trend-canvas canvas").count()).toBe(6);
  const paintedPixels = await page.locator(".diagnosis-trend-canvas canvas").evaluateAll((canvases) => canvases.map((canvas) => {
    const context = (canvas as HTMLCanvasElement).getContext("2d");
    if (!context) return 0;
    const pixels = context.getImageData(0, 0, canvas.width, canvas.height).data;
    let painted = 0;
    for (let index = 3; index < pixels.length; index += 4) {
      if (pixels[index] > 0) painted++;
    }
    return painted;
  }));
  expect(paintedPixels.every((count) => count > 100)).toBe(true);
  await page.getByRole("button", { name: "14日" }).click();
  await expect(page.getByRole("button", { name: "14日" })).toHaveClass(/active/);
  await accountSelect.selectOption("Megared脉拓-智元01");
  await expect(page.locator(".diagnosis-overview-heading p")).toContainText("Megared脉拓-智元01");
  await page.getByRole("button", { name: "全部" }).click();
  await expect(page.getByRole("button", { name: "全部" })).toHaveClass(/active/);
  await expect(page.locator(".diagnosis-overview-heading p")).toContainText("2026-06-28 - 2026-07-27");
  await page.getByRole("button", { name: "7日" }).click();
  await expect(page.locator(".diagnosis-account-table tbody tr")).toHaveCount(1);
  await expect(page.getByText("+12.5%", { exact: true })).toBeVisible();
  await expect(page.getByText("日报成本", { exact: true })).toBeVisible();
  await expect(page.locator(".diagnosis-account-table tbody tr td").nth(3)).toHaveText("¥39.22");
  await expect(page.getByText("修正后", { exact: false })).toHaveCount(0);
  const sparklinePath = await page.locator(".diagnosis-sparkline path").getAttribute("d");
  expect(sparklinePath?.match(/M/g)).toHaveLength(1);
  await expect(page.locator(".diagnosis-account-table thead")).not.toContainText("计划");
  await expect(page.locator(".diagnosis-account-table tbody tr td")).toHaveCount(8);
  await expect(page.getByRole("button", { name: "Megared脉拓-飓风03" })).toHaveCount(0);
  await expect(page.locator(".diagnosis-drawer")).toHaveCount(0);
});
