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
      raw_payload: { campaign_id: 81001, campaign_name: "通勤搜索放量计划", marketing_target: 4, bidding_strategy: 2, campaign_day_budget: 20000, optimize_objective: 18, campaign_filter_state: 1, not_available_status: 0, constraint_type: 101, time_period: "111111", explore_config: { expire_hour: 24 } },
      units: [{ id: 71001, name: "通勤搜索人群单元", campaign_id: 81001, enable: 1, filter_state: 10, created_at: "2026-08-18T12:00:00+08:00", updated_at: "2026-08-22T03:00:00+08:00", synced_at: "2026-08-22T04:00:00+08:00", raw_payload: { id: 71001, target_type: 3, event_bid: 800, not_available_status: 0, target_config: { target_age: "23-27", target_city: "all", intelligent_expansion: 1 }, keyword_with_bids: [{ keyword: "辅酶q10", bid: 800 }] } }],
      creativities: [{ id: 61001, name: "通勤笔记创意", campaign_id: 81001, unit_id: 71001, enable: 1, filter_state: 8, created_at: "2026-08-18T12:30:00+08:00", updated_at: "2026-08-22T03:10:00+08:00", synced_at: "2026-08-22T04:00:00+08:00", raw_payload: { creativity_id: 61001, note_id: "note-search", primary_title: "精力管理实测", audit_status: 1, conversion_type: 3 } }]
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
  const campaignFields = page.getByRole("region", { name: "计划全部字段", exact: true });
  const campaignBasics = page.getByLabel("计划全部字段基础信息");
  await expect(campaignBasics).toContainText("通勤搜索放量计划");
  await expect(campaignBasics).toContainText("81001");
  await expect(campaignFields).toContainText("投放时段");
  await expect(campaignFields.locator(".spotlight-field-grid > div").filter({ hasText: "出价策略" })).toContainText("手动出价");
  await expect(campaignFields.locator(".spotlight-field-grid > div").filter({ hasText: "计划日预算" })).toContainText("¥200.00");
  await expect(campaignFields.locator(".spotlight-field-grid > div").filter({ hasText: "计划日预算" })).toContainText("原始值 20000 分");
  await expect(campaignFields.locator('.spotlight-field-grid > div:has(dt code:text-is("not_available_status"))')).toContainText("正常可用，无不可用原因");
  const unknownConstraint = campaignFields.locator('.spotlight-field-grid > div:has(dt code:text-is("constraint_type"))');
  await expect(unknownConstraint).toContainText("尚未收录该码值含义");
  await expect(unknownConstraint).toContainText("原始值 101");
  const objectiveField = campaignFields.locator('.spotlight-field-grid > div:has(dt code:text-is("optimize_objective"))');
  await expect(objectiveField).toContainText("站外转化量");
  await expect(objectiveField.getByText("点击份额（SOC）")).toBeVisible();
  await expect(objectiveField.locator("details")).toHaveCount(0);
  await expect(campaignFields.locator(".spotlight-field-grid > div").filter({ hasText: "计划状态" })).toContainText("状态码说明");
  await expect(page.getByRole("heading", { name: "广告单元" })).toBeVisible();
  await expect(page.getByText("通勤搜索人群单元")).toBeVisible();
  const unitFields = page.getByRole("region", { name: "单元 71001 全部字段", exact: true });
  await expect(unitFields).toContainText("定向配置");
  await expect(unitFields).toContainText("搜索竞价关键词");
  await expect(unitFields.locator(".spotlight-field-grid > div").filter({ hasText: "定向类型" })).toContainText("高级定向");
  await expect(unitFields.locator(".spotlight-field-grid > div").filter({ hasText: "单元出价" })).toContainText("¥8.00");
  await expect(unitFields.locator('.spotlight-field-grid > div:has(dt code:text-is("not_available_status"))')).toContainText("创意不为空，可正常参与投放");
  const targetConfig = unitFields.locator(".spotlight-field-grid > div").filter({ hasText: "定向配置" });
  await expect(targetConfig.getByText("3 个字段")).toBeVisible();
  const expansionField = targetConfig.locator('.spotlight-object-value > dl > div:has(dt code:text-is("intelligent_expansion"))');
  await expect(expansionField.getByText("智能扩量", { exact: true })).toBeVisible();
  await expect(expansionField.locator(".spotlight-interpreted-value strong")).toHaveText("开启");
  await expect(page.getByRole("heading", { name: "投放创意" })).toBeVisible();
  await expect(page.getByText("通勤笔记创意")).toBeVisible();
  await page.getByText("通勤笔记创意").click();
  const creativityFields = page.getByRole("region", { name: "创意 61001 全部字段", exact: true });
  await expect(creativityFields).toContainText("精力管理实测");
  await expect(creativityFields.locator(".spotlight-field-grid > div").filter({ hasText: "审核状态" })).toContainText("审核通过");
  await expect(creativityFields.locator(".spotlight-field-grid > div").filter({ hasText: "转化类型" })).toContainText("私信组件");
  await campaignFields.getByLabel("查看出价策略说明").click();
  await expect(page).toHaveURL(/delivery\/helper\?field=bidding_strategy&level=campaign/);
  await expect(page.getByRole("heading", { name: "聚光配置助手" })).toBeVisible();
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
  await expect(page.getByRole("region", { name: "计划全部字段", exact: true })).toBeVisible();
});
