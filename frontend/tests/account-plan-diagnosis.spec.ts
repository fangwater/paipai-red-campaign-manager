import { expect, test } from "@playwright/test";

test("renders account diagnosis and drills into plan actions", async ({ page }) => {
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
          plan_kpis: { 搜索: 30, 信息流: 70 },
          dandelion_synced_at: "2026-08-02T09:15:00+08:00",
          dandelion_matched: 1,
          dandelion_missing: 1,
          accounts: [{
            account: "Megared脉拓-飓风03",
            placement: "搜索",
            spend: 5804.76,
            search_users: 148,
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
            over_plans: 1,
            enlarge_plans: 1,
            stop_plans: 1,
            points: [
              { report_date: "2026-07-21", spend: 5010.12, search_users: 109, cost: 45.74, search_rate_pct: 10.21, cpc: 2.12, ctr_pct: 3.71, note_count: 14 },
              { report_date: "2026-07-22", spend: 5232.44, search_users: 154, cost: 33.98, search_rate_pct: 11.08, cpc: 2.03, ctr_pct: 3.92, note_count: 15 },
              { report_date: "2026-07-23", spend: 5483.91, search_users: 148, cost: 37.05, search_rate_pct: 11.63, cpc: 1.98, ctr_pct: 4.01, note_count: 15 },
              { report_date: "2026-07-24", spend: null, search_users: null, cost: null, search_rate_pct: null, cpc: null, ctr_pct: null, note_count: null },
              { report_date: "2026-07-25", spend: null, search_users: null, cost: null, search_rate_pct: null, cpc: null, ctr_pct: null, note_count: null },
              { report_date: "2026-07-26", spend: 5612.77, search_users: 161, cost: 34.86, search_rate_pct: 12.02, cpc: 1.92, ctr_pct: 4.13, note_count: 16 },
              { report_date: "2026-07-27", spend: 5804.76, search_users: 148, cost: 39.22, search_rate_pct: 12.45, cpc: 1.86, ctr_pct: 4.32, note_count: 17 }
            ],
            plans: [
              { note_id: "note-stop", note_url: "https://www.xiaohongshu.com/explore/note-stop", campaign_name: "连续超标计划", spend: 200.26, cost: 33.38, cost_metric: "回搜成本", kpi: 30, over_kpi: true, action: "stop", consecutive_over_kpi: 3, dandelion: { title: "辅酶Q10真实体验", author: "测试达人", note_type: "图文", content_tag: "单品", published_date: "2026-07-20", data_updated_date: "2026-08-01", dandelion_amount: 220, impressions: 12000, reads: 1800, interactions: 75, read_cost: 0.12, interaction_cost: 2.93 } },
              { note_id: "note-grow", note_url: "https://www.xiaohongshu.com/explore/note-grow", campaign_name: "低成本放大计划", spend: 100.34, cost: 12.54, cost_metric: "回搜成本", kpi: 30, over_kpi: false, action: "enlarge", consecutive_over_kpi: 0 }
            ]
          }]
        }
      })
    });
  });

  await page.goto("/paipai/account-plan-diagnosis");
  await expect(page.getByRole("heading", { name: "子账户与计划诊断" })).toBeVisible();
  await expect(page.locator(".diagnosis-account-table tbody tr")).toHaveCount(1);
  await expect(page.getByText("+12.5%", { exact: true })).toBeVisible();
  await expect(page.locator(".diagnosis-sparkline path")).toHaveAttribute("d", /M/);
  await expect(page.getByText("蒲公英 1/2 · 更新 8月2日", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "Megared脉拓-飓风03" }).click();
  const drawer = page.getByRole("complementary", { name: /Megared脉拓-飓风03计划诊断/ });
  await expect(drawer).toBeVisible();
  await expect(drawer.getByRole("heading", { name: "子账户数据总览" })).toBeVisible();
  await expect(drawer.locator(".diagnosis-metric-card")).toHaveCount(7);
  await expect(drawer.getByText("回搜人数", { exact: true })).toBeVisible();
  await expect(drawer.getByText("148", { exact: true })).toBeVisible();
  await expect(drawer.getByText("12.45%", { exact: true })).toBeVisible();
  await expect(drawer.locator(".diagnosis-metric-sparkline path")).toHaveCount(7);
  await expect(page.getByText("连续超标计划", { exact: true })).toBeVisible();
  await expect(page.getByText("辅酶Q10真实体验", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: /建议放大 1/ }).click();
  await expect(page.getByText("低成本放大计划", { exact: true })).toBeVisible();
  await expect(page.getByText("未匹配", { exact: true })).toBeVisible();
});
