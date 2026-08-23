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
      raw_payload: { campaign_id: 81001, campaign_name: "通勤搜索放量计划", campaign_create_time: "2026-08-18 10:00:00", campaign_update_time: "2026-08-22 03:30:00", creation_type: 0, build_type: 0, creativity_state: 0, platform: 1, marketing_target: 4, bidding_strategy: 3, campaign_day_budget: 20000, optimize_objective: 18, campaign_filter_state: 1, campaign_enable: 1, not_available_status: 0, constraint_type: 101, constraint_value: 0, start_time: "2026-08-18", expire_time: "2919-01-01", time_period_type: 1, time_period: "111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111", explore_config: { expire_hour: 24 } },
      units: [{ id: 71001, name: "通勤搜索人群单元", campaign_id: 81001, enable: 1, filter_state: 10, created_at: "2026-08-18T12:00:00+08:00", updated_at: "2026-08-22T03:00:00+08:00", synced_at: "2026-08-22T04:00:00+08:00", raw_payload: { id: 71001, target_type: 3, event_bid: 800, not_available_status: 0, note_ids: ["note-search"], target_config: { target_age: "23-27#28-32#51-100", target_city_type: 0, target_city: "北京#上海", targetAreaCode: "2205#2307", intelligent_expansion: 1 }, keyword_with_bids: [{ keyword: "辅酶q10", bid: 800 }] } }],
      creativities: [{ id: 61001, name: "通勤笔记创意", campaign_id: 81001, unit_id: 71001, enable: 1, filter_state: 8, note_title: "精力管理实测", note_url: "https://www.xiaohongshu.com/explore/note-search?xsec_token=test", created_at: "2026-08-18T12:30:00+08:00", updated_at: "2026-08-22T03:10:00+08:00", synced_at: "2026-08-22T04:00:00+08:00", raw_payload: { creativity_id: 61001, note_id: "note-search", primary_title: "过期标题不会覆盖同步标题", material_type: 1, creativity_filter_state: 8, creativity_audit_state: 3, audit_status: 1, conversion_type: 3 } }]
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
  const overview = page.getByRole("region", { name: "计划概要", exact: true });
  await expect(overview).toContainText("81001");
  await expect(overview).toContainText("聚光投放平台");
  await expect(overview).toContainText("搜索推广");
  await expect(overview).toContainText("¥200.00");
  await expect(overview).toContainText("2026-08-18 至 2919-01-01");
  await expect(overview.getByRole("link", { name: "通勤搜索人群单元" })).toHaveAttribute("href", "#spotlight-units");
  await expect(overview.getByRole("link", { name: "通勤笔记创意" })).toHaveAttribute("href", "#spotlight-creativities");
  const executionStatus = overview.locator(".spotlight-decision-field").filter({ hasText: "执行状态" });
  await expect(executionStatus).toContainText("有效");
  await expect(executionStatus).toContainText("计划开关开启");
  await expect(executionStatus).toContainText("当前无不可用原因");
  await expect(executionStatus.locator("code")).toHaveCount(8);
  await expect(executionStatus).toContainText("计划状态说明");
  const campaignFields = page.getByRole("region", { name: "计划执行配置（全部字段）", exact: true });
  await expect(campaignFields.getByRole("heading", { name: "创意搭建类型与状态" })).toBeVisible();
  await expect(campaignFields.getByRole("heading", { name: "排期与投放时段" })).toBeVisible();
  await expect(campaignFields).toContainText("投放时段");
  await expect(campaignFields).toContainText("每天全天");
  await expect(campaignFields.locator(".spotlight-field-grid > div").filter({ hasText: "出价策略" })).toContainText("最大转化");
  await expect(campaignFields.locator('dt code:text-is("not_available_status")')).toHaveCount(0);
  await expect(campaignFields.locator('.spotlight-field-grid > div:has(dt code:text-is("creation_type"))')).toContainText("标准投放");
  await expect(campaignFields.locator('.spotlight-field-grid > div:has(dt code:text-is("creativity_state"))')).toContainText("无附加创意状态限制");
  const costControl = campaignFields.locator(".spotlight-decision-field").filter({ hasText: "成本控制" });
  await expect(costControl).toContainText("最大转化：不设目标成本");
  await expect(costControl).toContainText("约束类型 101");
  await expect(costControl).toContainText("不设目标成本（最大转化）");
  const objectiveField = campaignFields.locator('.spotlight-field-grid > div:has(dt code:text-is("optimize_objective"))');
  await expect(objectiveField).toContainText("站外转化量");
  await expect(objectiveField.getByText("点击份额（SOC）")).toBeVisible();
  await expect(objectiveField.locator("details")).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "广告单元" })).toBeVisible();
  await expect(page.locator("#spotlight-units").getByText("通勤搜索人群单元", { exact: true })).toBeVisible();
  const unitFields = page.getByRole("region", { name: "单元 71001 全部字段", exact: true });
  await expect(unitFields).toContainText("定向配置");
  await expect(unitFields).toContainText("搜索竞价关键词");
  await expect(unitFields.locator(".spotlight-field-grid > div").filter({ hasText: "定向类型" })).toContainText("高级定向");
  await expect(unitFields.locator(".spotlight-field-grid > div").filter({ hasText: "单元出价" })).toContainText("¥8.00");
  await expect(unitFields.locator('.spotlight-field-grid > div:has(dt code:text-is("not_available_status"))')).toContainText("创意不为空，可正常参与投放");
  const targetConfig = unitFields.locator(".spotlight-field-grid > div").filter({ hasText: "定向配置" });
  await expect(targetConfig.getByText("4 个字段")).toBeVisible();
  await expect(targetConfig).toContainText("23-27 岁、28-32 岁、51 岁以上");
  await expect(targetConfig).toContainText("全国投放");
  await expect(targetConfig.locator('dt code:text-is("targetAreaCode")')).toHaveCount(0);
  const expansionField = targetConfig.locator('.spotlight-object-value > dl > div:has(dt code:text-is("intelligent_expansion"))');
  await expect(expansionField.getByText("智能扩量", { exact: true })).toBeVisible();
  await expect(expansionField.locator(".spotlight-interpreted-value strong")).toHaveText("开启");
  await expect(page.getByRole("heading", { name: "投放创意" })).toBeVisible();
  await expect(page.locator("#spotlight-creativities").getByText("通勤笔记创意", { exact: true })).toBeVisible();
  const creativityFields = page.getByRole("region", { name: "创意 61001 全部字段", exact: true });
  await expect(creativityFields).toContainText("精力管理实测");
  await expect(creativityFields.getByRole("link", { name: "精力管理实测" })).toHaveAttribute("href", "https://www.xiaohongshu.com/explore/note-search?xsec_token=test");
  await expect(creativityFields.locator(".spotlight-field-grid > div").filter({ hasText: "素材类型" })).toContainText("笔记");
  await expect(creativityFields.locator(".spotlight-field-grid > div").filter({ hasText: "创意状态" })).toContainText("有效");
  await expect(creativityFields.locator('.spotlight-field-grid > div:has(dt code:text-is("audit_status"))')).toContainText("审核通过");
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
  await expect(page.getByRole("region", { name: "计划概要", exact: true })).toBeVisible();
  await expect(page.getByRole("region", { name: "计划执行配置（全部字段）", exact: true })).toBeVisible();
});
