import { expect, test } from "@playwright/test";

const match = {
  advertiser_id: 336376,
  advertiser_name: "品牌主账户",
  campaign_id: 6243312,
  campaign_name: "磷虾油搜索计划",
  campaign_filter_state: 1,
  campaign_enable: 1,
  marketing_target: 4,
  placement: 2,
  optimize_target: 0,
  optimize_objective: 0,
  deep_optimize_objective: -1,
  promotion_target: 1,
  bidding_strategy: 2,
  campaign_day_budget: 10000,
  campaign_created_at: "2026-07-20T10:00:00+08:00",
  campaign_updated_at: "2026-07-23T18:00:00+08:00",
  start_date: "2026-07-20",
  expire_date: "2919-01-01",
  synced_at: "2026-07-24T11:10:38+08:00",
  units: [{
    unit_id: 7001,
    unit_name: "搜索核心人群",
    unit_enable: 1,
    unit_filter_state: 10,
    event_bid: 350,
    target_type: 2,
    not_available_status: 0,
    creation_type: 0,
    updated_at: "2026-07-23T17:00:00+08:00",
    synced_at: "2026-07-24T11:13:03+08:00",
    creativities: [{
      creativity_id: 8001,
      creativity_name: "笔记投放创意",
      creativity_enable: 1,
      creativity_filter_state: 8,
      material_type: 1,
      conversion_type: 0,
      note_id: "note-a",
      item_id: "item-a",
      audit_status: 1,
      creativity_audit_state: 3,
      creation_type: 0,
      updated_at: "2026-07-23T16:00:00+08:00",
      synced_at: "2026-07-24T11:15:35+08:00"
    }]
  }]
};

test("queries and expands all linked Spotlight levels", async ({ page }) => {
  const requests: URL[] = [];
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: { status: "ok" } }) }));
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] }) }));
  await page.route("**/paipai/api/analytics/maituo/xhs-links?*", async (route) => {
    const url = new URL(route.request().url());
    requests.push(url);
    const secondMatch = { ...match, campaign_id: 6243313, campaign_name: "辅酶信息流计划", placement: 1, units: [] };
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          report_date: "2026-07-23",
          total: 26,
          page: Number(url.searchParams.get("page") || 1),
          page_size: 25,
          items: [
            { note_id: "note-a", campaign_name: "磷虾油搜索计划", placement: "搜索", subaccounts: ["磷虾油"], spend: 1280.5, search_users: 80, search_cost: 16.01, matches: [match] },
            { note_id: "note-b", campaign_name: "辅酶信息流计划", placement: "信息流", subaccounts: ["辅酶"], spend: 620, search_users: 32, search_cost: 19.38, matches: [secondMatch] }
          ]
        }
      })
    });
  });

  await page.goto("/paipai/xhs-link-query");
  await expect(page.getByRole("heading", { name: "聚光关联查询" })).toBeVisible();
  await expect(page.locator(".xhs-link-table tbody tr")).toHaveCount(2);
  await expect(page.locator(".linked-campaign")).toContainText("品牌主账户");
  await expect(page.locator(".unit-table tbody tr")).toHaveCount(1);
  await expect(page.locator(".creativity-table tbody tr")).toHaveCount(1);
  await expect(page.locator(".campaign-field-grid")).toContainText("产品种草");
  await expect(page.locator(".campaign-field-grid")).toContainText("笔记");
  await expect(page.locator(".campaign-field-grid")).toContainText("手动出价");
  await expect(page.locator(".campaign-field-grid")).toContainText("点击量");
  await expect(page.locator(".campaign-field-grid")).toContainText("未启用");
  await expect(page.locator(".campaign-field-grid")).not.toContainText("官方未定义（0）");
  await page.getByRole("button", { name: "查看优化目标配置说明" }).click();
  const objectiveHelp = page.getByRole("dialog", { name: "优化目标可配置项" });
  await expect(objectiveHelp).toBeVisible();
  await expect(objectiveHelp).toContainText("产品种草");
  await expect(objectiveHelp).toContainText("点击份额（SOC）");
  await expect(objectiveHelp).toContainText("客资收集");
  await expect(objectiveHelp).toContainText("APP下载按钮点击");
  await expect(objectiveHelp).toContainText("微信小游戏订单支付数");
  await expect(objectiveHelp.getByText("当前计划")).toBeVisible();
  await page.getByRole("button", { name: "关闭优化目标配置说明" }).click();
  await expect(objectiveHelp).toBeHidden();
  await expect(page.locator(".unit-table tbody tr")).toContainText("有效");
  await expect(page.locator(".unit-table tbody tr")).toContainText("智能定向");
  await expect(page.locator(".unit-table tbody tr")).toContainText("创意不为空");
  await expect(page.locator(".unit-table tbody tr")).toContainText("标准投");
  await expect(page.locator(".creativity-table tbody tr")).toContainText("有效");
  await expect(page.locator(".creativity-table tbody tr")).toContainText("笔记");
  await expect(page.locator(".creativity-table tbody tr")).toContainText("无组件");
  await expect(page.locator(".creativity-table tbody tr")).toContainText("审核通过");
  await expect(page.getByText("http", { exact: false })).toHaveCount(0);

  await page.locator(".xhs-link-table tbody tr").nth(1).click();
  await expect(page.locator(".link-detail-heading")).toContainText("辅酶信息流计划");

  await page.getByPlaceholder("搜索笔记、计划、广告主、单元或创意").fill("核心人群");
  await expect.poll(() => requests.some((url) => url.searchParams.get("q") === "核心人群")).toBeTruthy();

  await page.getByRole("button", { name: "下一页" }).click();
  await expect.poll(() => requests.some((url) => url.searchParams.get("page") === "2")).toBeTruthy();

  await page.getByRole("button", { name: "聚光关联查询" }).click();
  await expect(page).toHaveURL(/\/paipai\/xhs-link-query$/);
});
