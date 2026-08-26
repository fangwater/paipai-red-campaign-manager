import { expect, test, type Page, type Route } from "@playwright/test";

const now = "2026-08-26T08:00:00Z";
const advertiserID = 710001;
const noteID = "64a1b2c3d4e5f60718293a4b";
const draftID = "drf_0123456789abcdef0123456789abcdef";

function envelope(data: unknown) {
  return { success: true, data };
}

async function fulfill(route: Route, data: unknown, status = 200) {
  await route.fulfill({ status, contentType: "application/json", body: JSON.stringify(envelope(data)) });
}

function template(placement: "feed" | "search") {
  const feed = placement === "feed";
  return {
    placement,
    placement_code: feed ? 1 : 2,
    available: true,
    sample_count: feed ? 8 : 5,
    mode_sample_count: feed ? 6 : 4,
    confidence: feed ? 0.75 : 0.8,
    latest_synced_at: now,
    summary: {
      marketing_target: feed ? 4 : 13,
      promotion_target: 1,
      bidding_strategy: feed ? 7 : 2,
      optimize_target: 1,
      day_budget_fen: feed ? 30000 : 50000,
      event_bid_fen: feed ? 1800 : 0,
      pacing_mode: feed ? 1 : 2,
      time_period_type: 0,
      conversion_type: 1
    },
    audiences: [{
      id: `${placement}-audience`,
      name: feed ? "职场轻熟人群" : "默认定向",
      description: feed ? "25-34岁 · 女性 · 全国" : "年龄不限 · 性别不限 · 全国",
      sample_count: feed ? 5 : 4,
      target_type: feed ? 3 : 0
    }],
    keyword_defaults: {
      bid_fen: feed ? 1800 : 2300,
      feed_bid_fen: 0,
      keyword_source: 0,
      phrase_match_type: feed ? 1 : 3,
      sample_count: feed ? 0 : 9
    }
  };
}

async function mockQuickPlanAPI(page: Page) {
  let submitted: Record<string, unknown> | undefined;
  await page.route("**/paipai/api/delivery/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const path = url.pathname.replace("/paipai/api/delivery", "");
    if (path === "/session") return fulfill(route, {
      actor: { id: "delivery-console", role: "operator" },
      advertisers: [{ advertiser_id: advertiserID, advertiser_name: "快速计划广告主" }],
      all_advertisers: true
    });
    if (path === "/quick-plan/templates") return fulfill(route, {
      feed: template("feed"),
      search: template("search"),
      generated_at: now
    });
    if (path === "/assets") return fulfill(route, {
      advertiser_id: advertiserID,
      count: 1,
      generated_at: now,
      notes: [{
        note_id: noteID,
        title: "辅酶 Q10 职场精力笔记",
        content: "稿件正文",
        audience: ["职场人"],
        scenarios: ["通勤"],
        note_types: ["科普"],
        historical_spend: 128,
        historical_search_users: 42,
        historical_search_cost: 3.1,
        published: true,
        creativity_count: 2
      }]
    });
    if (path === "/quick-plan/drafts") {
      submitted = request.postDataJSON() as Record<string, unknown>;
      return fulfill(route, {
        id: draftID,
        advertiser_id: advertiserID,
        status: "draft",
        current_version: 1,
        spec: {},
        spec_hash: "a".repeat(64),
        idempotency_key: String(submitted.idempotency_key),
        created_by: "delivery-console",
        updated_by: "delivery-console",
        created_at: now,
        updated_at: now
      }, 201);
    }
    if (path === `/drafts/${draftID}/validate`) return fulfill(route, {
      id: "val_0123456789abcdef0123456789abcdef",
      draft_id: draftID,
      draft_version: 1,
      spec_hash: "a".repeat(64),
      rules_version: "delivery-rules/2026-08-13",
      contract_version: "xhs-jg/2026-05-candidate",
      valid: true,
      errors: [],
      warnings: [],
      capability_snapshot: {},
      valid_until: "2026-08-26T08:15:00Z",
      created_by: "delivery-console",
      created_at: now
    });
    await route.fulfill({ status: 404, contentType: "application/json", body: JSON.stringify({ success: false, error: `unmocked ${path}` }) });
  });
  return { submitted: () => submitted };
}

test("creates a search draft with editable template defaults", async ({ page }) => {
  const state = await mockQuickPlanAPI(page);
  await page.goto("/paipai/quick-plan");

  await expect(page.getByRole("heading", { name: "快速新建计划" })).toBeVisible();
  await expect(page.getByRole("button", { name: "快速新建计划" })).toHaveClass(/active/);
  await expect(page.getByText("8", { exact: true }).first()).toBeVisible();
  await page.getByRole("button", { name: /搜索\s*5/ }).click();
  await expect(page.getByLabel("搜索默认模板").getByText("种草直达", { exact: true })).toBeVisible();
  await expect(page.getByText("升级匹配 · ¥23.00", { exact: true })).toBeVisible();

  await page.getByLabel("营销目标").selectOption("4");
  await page.getByLabel("出价策略").selectOption("7");
  await page.getByLabel("日预算").fill("720");
  await page.getByLabel("关键词出价").fill("12.50");
  await page.getByLabel("消耗节奏").selectOption("0");
  await page.getByLabel("投放时段").selectOption("1");
  await page.getByLabel("关键词匹配").selectOption("0");

  await page.getByRole("button", { name: /辅酶 Q10 职场精力笔记/ }).click();
  await page.getByLabel("定向配置").selectOption("search-audience");
  await page.getByPlaceholder("每行一个搜索词").fill("辅酶Q10\n抗氧化");
  await page.getByRole("button", { name: "生成并校验草稿" }).click();

  await expect(page.getByRole("heading", { name: "草稿已创建并通过校验" })).toBeVisible();
  const submitted = state.submitted();
  expect(Object.keys(submitted || {}).sort()).toEqual([
    "advertiser_id", "audience_id", "idempotency_key", "keywords", "note_id", "note_title", "overrides", "placement"
  ]);
  expect(submitted).toMatchObject({
    advertiser_id: advertiserID,
    placement: "search",
    note_id: noteID,
    audience_id: "search-audience",
    keywords: ["辅酶Q10", "抗氧化"],
    overrides: {
      marketing_target: 4,
      bidding_strategy: 7,
      day_budget_fen: 72000,
      event_bid_fen: 0,
      pacing_mode: 0,
      time_period_type: 1,
      keyword_bid_fen: 1250,
      phrase_match_type: 0
    }
  });
  await expect(page.getByRole("button", { name: "继续编辑" })).toBeVisible();
});

test("opens the global quick-plan template from the delivery navigation", async ({ page }) => {
  await mockQuickPlanAPI(page);
  await page.goto("/paipai/");
  await page.getByRole("button", { name: "快速新建计划", exact: true }).click();

  await expect(page).toHaveURL(/\/paipai\/quick-plan$/);
  await expect(page.getByRole("heading", { name: "快速新建计划" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "全局统计基准" })).toBeVisible();
});

test("keeps the quick-plan controls within a mobile viewport", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await mockQuickPlanAPI(page);
  await page.goto("/paipai/quick-plan");
  await expect(page.getByRole("heading", { name: "快速新建计划" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "本次计划参数" })).toBeVisible();
  await expect(page.getByText("职场轻熟人群", { exact: true }).first()).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
});
