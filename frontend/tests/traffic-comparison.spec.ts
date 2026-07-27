import { expect, test } from "@playwright/test";

const dates = ["2026-07-21", "2026-07-22", "2026-07-23"];
const points = (costs: number[]) => dates.map((reportDate, index) => ({
  report_date: reportDate,
  spend: 100 + index * 20,
  search_users: 5 + index,
  search_cost: costs[index],
  has_search_cost: true
}));

test("prioritizes cost gaps and expands campaigns for the same note placement", async ({ page }) => {
  const requestedWindows: string[] = [];
  await page.route("**/paipai/api/analytics/maituo/traffic-comparisons?*", async (route) => {
    const url = new URL(route.request().url());
    requestedWindows.push(url.searchParams.get("window") ?? "");
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          window: url.searchParams.get("window") || "7d",
          report_dates: dates,
          latest_date: "2026-07-23",
          total: 2,
          page: 1,
          page_size: 25,
          items: [
            {
              note_id: "note-multi", placement: "搜索", campaign_count: 2, comparable_campaign_count: 2,
              latest_search_cost_min: 10, latest_search_cost_max: 25, search_cost_gap: 15,
              latest_spend: 360, latest_search_users: 13,
              campaigns: [
                { campaign_name: "计划高成本", first_report_date: dates[0], last_report_date: dates[2], active_days: 3, latest_spend: 200, latest_search_users: 6, latest_search_cost: 25, has_latest_search_cost: true, total_spend: 500, total_search_users: 16, points: points([18, 22, 25]) },
                { campaign_name: "计划低成本", first_report_date: dates[0], last_report_date: dates[2], active_days: 3, latest_spend: 160, latest_search_users: 7, latest_search_cost: 10, has_latest_search_cost: true, total_spend: 430, total_search_users: 18, points: points([12, 11, 10]) }
              ]
            },
            {
              note_id: "note-single", placement: "信息流", campaign_count: 1, comparable_campaign_count: 1,
              latest_search_cost_min: 30, latest_search_cost_max: 30, search_cost_gap: 0,
              latest_spend: 120, latest_search_users: 4,
              campaigns: [
                { campaign_name: "单计划", first_report_date: dates[0], last_report_date: dates[2], active_days: 3, latest_spend: 120, latest_search_users: 4, latest_search_cost: 30, has_latest_search_cost: true, total_spend: 300, total_search_users: 10, points: points([32, 31, 30]) }
              ]
            }
          ]
        }
      })
    });
  });
  await page.route("**/paipai/api/analytics/maituo/traffic-comparison-delivery?*", async (route) => {
    const url = new URL(route.request().url());
    const noteID = url.searchParams.get("note_id") || "";
    const placement = url.searchParams.get("placement") || "";
    const target = (crowd: string, city: string) => ({
      gender: "", age: "18-24#25-34", city, area_code: "", device: "ios", device_price: "",
      intelligent_expansion: 1, generalization_switch: 0, search_city_intent: "1",
      interest_keywords: [], behavior_keywords: [crowd + "行为"], excluded_crowds: [],
      crowd_packages: [{ name: crowd, value: crowd }], content_interests: ["美妆护肤"], shopping_interests: [],
      premium_crowds: [], dandelion_crowds: [], brand_interest_group: false, brand_recognition_group: false,
      category_interest_group: true, goods_interest_group: false
    });
    const match = (name: string, budget: number, crowd: string, city: string) => ({
      advertiser_name: "品牌广告账户", campaign_day_budget: budget, bidding_strategy: 3,
      marketing_target: 4, optimize_objective: 1,
      units: [{ unit_name: name + "单元", event_bid: 1500, target_type: 3, delivery: {
        search_keywords: [{ keyword: crowd + "关键词", bid: 1200, feed_bid: 0, phrase_match_type: 1 }], target: target(crowd, city)
      }}]
    });
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: {
        report_date: "2026-07-23", note_id: noteID, placement,
        campaigns: noteID === "note-multi" ? [
          { campaign_name: "计划高成本", subaccounts: ["账户A"], matches: [match("高成本", 20000, "都市白领", "北京#上海#北屯")] },
          { campaign_name: "计划低成本", subaccounts: ["账户A"], matches: [match("低成本", 10000, "精致妈妈", "北京#广东")] }
        ] : [{ campaign_name: "单计划", subaccounts: ["账户A"], matches: [match("单计划", 10000, "都市白领", "北京")] }]
      }} )
    });
  });

  await page.goto("/paipai/traffic-comparison");
  await expect(page.getByRole("heading", { name: "投流情况对比" })).toBeVisible();
  await expect(page.locator(".comparison-list-table tbody tr")).toHaveCount(2);
  await expect(page.locator(".comparison-list-table tbody tr").first()).toContainText("¥15.00");
  await expect(page.locator(".comparison-detail")).toContainText("计划高成本");
  await expect(page.locator(".comparison-detail")).toContainText("计划低成本");
  await expect(page.locator(".comparison-gap-summary")).toContainText("¥15.00");
  await expect(page.locator(".comparison-campaign-table tbody tr")).toHaveCount(2);
  await expect(page.locator(".comparison-campaign-table tbody tr").first()).toContainText("+¥15.00");
  await expect(page.locator(".delivery-difference-section")).toContainText("投流配置差异");
  await expect(page.locator(".delivery-difference-section")).not.toContainText("日报子账户");
  await expect(page.locator(".delivery-difference-section")).not.toContainText("聚光广告账户");
  await expect(page.locator(".delivery-difference-section")).not.toContainText("计划日预算");
  await expect(page.locator(".delivery-difference-section")).toContainText("都市白领");
  await expect(page.locator(".delivery-difference-section")).toContainText("精致妈妈");
  await expect(page.locator(".delivery-plan-heading").first()).toContainText("当天成本 ¥25.00");
  await expect(page.locator(".delivery-plan-heading").nth(1).locator("xpath=..")) .toHaveClass(/best-cost-column/);
  const regionRow = page.locator(".delivery-diff-table tbody tr").filter({ hasText: "省级地域" });
  await expect(regionRow).toContainText("上海");
  await expect(regionRow).toContainText("广东");
  await expect(regionRow).toContainText("新疆");
  await expect(regionRow).not.toContainText("北屯");
  await expect(regionRow).not.toContainText("北京");
  await expect(page.locator(".delivery-diff-table tbody tr.same")).toHaveCount(0);

  await page.getByText("显示相同项").click();
  await expect(page.locator(".delivery-diff-table tbody tr.same")).not.toHaveCount(0);
  await expect(page.locator(".delivery-difference-section")).toContainText("出价策略");

  const chart = page.getByRole("img", { name: "不同计划回搜成本对比折线图" });
  await expect(chart).toBeVisible();
  await expect.poll(() => chart.locator("canvas").evaluate((canvas) => {
    const context = canvas.getContext("2d");
    if (!context) return false;
    const pixels = context.getImageData(0, 0, canvas.width, canvas.height).data;
    for (let index = 3; index < pixels.length; index += 4) if (pixels[index] > 0) return true;
    return false;
  })).toBe(true);

  await page.locator(".comparison-list-table tbody tr").nth(1).click();
  await expect(page.locator(".comparison-detail-heading")).toContainText("note-single");
  await expect(page.locator(".comparison-gap-summary")).toContainText("无计划差异");
  await expect(page.locator(".comparison-campaign-table tbody tr")).toHaveCount(1);

  await page.getByRole("button", { name: "3D" }).click();
  await expect.poll(() => requestedWindows).toContain("3d");
});
