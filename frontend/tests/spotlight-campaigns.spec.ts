import { expect, test, type Page } from "@playwright/test";

const campaign = {
  advertiser_id: 9001,
  advertiser_name: "辅酶聚光账户",
  campaign_id: 81001,
  campaign_name: "通勤搜索放量计划",
  campaign_filter_state: 1,
  campaign_enable: 1,
  marketing_target: 4,
  placement: 2,
  bidding_strategy: 2,
  campaign_day_budget: 20000,
  start_date: "2026-08-18",
  expire_date: "2919-01-01",
  updated_at: "2026-08-22T03:30:00+08:00",
  synced_at: "2026-08-22T04:00:00+08:00",
  unit_count: 1,
  creativity_count: 1
};

async function mockShell(page: Page) {
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, body: "ok" }));
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({
    status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] })
  }));
}

test("campaign manager searches by name or ID and opens all Spotlight dimensions", async ({ page }) => {
  await mockShell(page);
  const searches: string[] = [];
  await page.route("**/paipai/api/analytics/spotlight/campaigns?*", async (route) => {
    searches.push(new URL(route.request().url()).searchParams.get("q") ?? "");
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({
      success: true, data: { total: 1, page: 1, page_size: 25, items: [campaign] }
    }) });
  });
  await page.route("**/paipai/api/analytics/spotlight/campaign-detail?*", async (route) => {
    const params = new URL(route.request().url()).searchParams;
    expect(params.get("advertiser_id")).toBe("9001");
    expect(params.get("campaign_id")).toBe("81001");
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: {
      campaign,
      raw_payload: { campaign_id: 81001, campaign_name: "通勤搜索放量计划", time_period: "111111", explore_config: { expire_hour: 24 } },
      units: [{ id: 71001, name: "通勤搜索人群单元", campaign_id: 81001, enable: 1, filter_state: 10, created_at: "2026-08-18T12:00:00+08:00", updated_at: "2026-08-22T03:00:00+08:00", synced_at: "2026-08-22T04:00:00+08:00", raw_payload: { id: 71001, target_type: 3, event_bid: 800, target_config: { age: "23-27", city: "all" }, keyword_with_bids: [{ keyword: "辅酶q10", bid: 800 }] } }],
      creativities: [{ id: 61001, name: "通勤笔记创意", campaign_id: 81001, unit_id: 71001, enable: 1, filter_state: 8, created_at: "2026-08-18T12:30:00+08:00", updated_at: "2026-08-22T03:10:00+08:00", synced_at: "2026-08-22T04:00:00+08:00", raw_payload: { creativity_id: 61001, note_id: "note-search", primary_title: "精力管理实测", audit_status: 1 } }]
    } }) });
  });

  await page.goto("/paipai/delivery/campaigns");
  await expect(page.getByRole("heading", { name: "计划详情" })).toBeVisible();
  await expect(page.locator(".nav-item.active")).toHaveText("计划详情");
  await expect(page.getByRole("table", { name: "聚光计划搜索结果" })).toContainText("通勤搜索放量计划");

  await page.getByLabel("按计划名称或计划 ID 搜索").fill("81001");
  await page.getByRole("search").getByRole("button", { name: "搜索", exact: true }).click();
  await expect.poll(() => searches.at(-1)).toBe("81001");
  await page.getByRole("link", { name: /通勤搜索放量计划/ }).click();

  await expect(page).toHaveURL(/advertiser_id=9001&campaign_id=81001/);
  await expect(page.getByRole("heading", { name: "通勤搜索放量计划" })).toBeVisible();
  await expect(page.getByLabel("计划全部字段")).toContainText("投放时段");
  await expect(page.getByRole("heading", { name: "广告单元" })).toBeVisible();
  await expect(page.getByText("通勤搜索人群单元")).toBeVisible();
  await expect(page.getByLabel("单元 71001 全部字段")).toContainText("定向配置");
  await expect(page.getByLabel("单元 71001 全部字段")).toContainText("搜索竞价关键词");
  await expect(page.getByRole("heading", { name: "投放创意" })).toBeVisible();
  await expect(page.getByText("通勤笔记创意")).toBeVisible();
  await page.getByText("通勤笔记创意").click();
  await expect(page.getByLabel("创意 61001 全部字段")).toContainText("精力管理实测");
});

test("campaign detail stays usable on mobile", async ({ page }) => {
  await mockShell(page);
  await page.route("**/paipai/api/analytics/spotlight/campaign-detail?*", (route) => route.fulfill({
    status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: {
      campaign, raw_payload: { campaign_id: 81001, campaign_name: "通勤搜索放量计划", campaign_day_budget: 20000 }, units: [], creativities: []
    } })
  }));
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/paipai/delivery/campaigns?advertiser_id=9001&campaign_id=81001");
  await expect(page.getByRole("heading", { name: "通勤搜索放量计划" })).toBeVisible();
  expect(await page.evaluate(() => document.body.scrollWidth <= window.innerWidth)).toBe(true);
  await expect(page.getByLabel("计划全部字段")).toBeVisible();
});
