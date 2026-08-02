import { expect, test, type Page } from "@playwright/test";

type Dimension = "audience" | "scenario";

function makeNote(id: string, overrides: Record<string, unknown> = {}) {
  return {
    note_id: id,
    title: id === "note-1" ? "通勤精力管理实测" : "一周辅酶记录",
    url: "https://www.xiaohongshu.com/explore/" + id,
    author: id === "note-1" ? "一颗栗子" : "小K",
    published_date: "2026-07-20",
    agency: "智元",
    content_type: "科普",
    audience: "职场人",
    scenario: "精力疲惫",
    dandelion_cost: id === "note-1" ? 12.5 : 31,
    boom: id === "note-1",
    search_spend: id === "note-1" ? 480 : 60,
    search_cost: id === "note-1" ? 24 : 40,
    search_qualified: id === "note-1",
    feed_spend: 0,
    feed_cost: null,
    feed_qualified: false,
    flow_evaluated: id === "note-1",
    flow_qualified: id === "note-1",
    roi: id === "note-1" ? 1.8 : 0.4,
    roi_qualified: id === "note-1",
    all_qualified: id === "note-1",
    ...overrides
  };
}

function makeResult(spu: string, agency: string, dimension: Dimension) {
  const primaryDimension = dimension === "audience" ? "职场人" : "精力疲惫";
  const secondDimension = dimension === "audience" ? "健身人" : "运动恢复";
  const notes = [makeNote("note-1"), makeNote("note-2")];
  return {
    spu,
    agency,
    dimension,
    sources: {
      dandelion_data_date: "2026-07-29",
      dandelion_synced_at: "2026-08-02T09:00:00+08:00",
      maituo_report_date: "2026-07-30",
      guorai_snapshot_date: "2026-08-01",
      guorai_window_start: "2026-07-19",
      guorai_window_end: "2026-08-01",
      manuscript_synced_at: "2026-08-02T08:00:00+08:00"
    },
    coverage: {
      total_notes: 4,
      content_type_tagged: 3,
      audience_tagged: 3,
      scenario_tagged: 3,
      dandelion_cost_notes: 3,
      flow_evaluated_notes: 2,
      roi_evaluated_notes: 2,
      all_metrics_notes: 2
    },
    types: ["科普", "经验分享", "未标注"],
    dimensions: [primaryDimension, secondDimension, "未标注"],
    cells: [
      {
        content_type: "科普",
        dimension: primaryDimension,
        total_notes: 2,
        dandelion_eligible: 2,
        boom_count: 1,
        boom_rate: 0.5,
        flow_evaluated: 1,
        flow_qualified: 1,
        roi_evaluated: 2,
        roi_qualified: 1,
        all_qualified: 1,
        notes
      },
      {
        content_type: "经验分享",
        dimension: secondDimension,
        total_notes: 1,
        dandelion_eligible: 1,
        boom_count: 1,
        boom_rate: 1,
        flow_evaluated: 1,
        flow_qualified: 0,
        roi_evaluated: 0,
        roi_qualified: 0,
        all_qualified: 0,
        notes: [makeNote("note-3", { content_type: "经验分享", audience: "健身人", scenario: "运动恢复", roi: null, roi_qualified: false, all_qualified: false })]
      },
      {
        content_type: "未标注",
        dimension: "未标注",
        total_notes: 1,
        dandelion_eligible: 0,
        boom_count: 0,
        boom_rate: null,
        flow_evaluated: 0,
        flow_qualified: 0,
        roi_evaluated: 0,
        roi_qualified: 0,
        all_qualified: 0,
        notes: [makeNote("note-4", { content_type: "未标注", audience: "未标注", scenario: "未标注", dandelion_cost: null, boom: false, roi: null })]
      }
    ]
  };
}

async function mockCommon(page: Page, requested: string[]) {
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, body: "ok" }));
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({
    status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] })
  }));
  await page.route("**/paipai/api/analytics/content-analysis?*", async (route) => {
    const params = new URL(route.request().url()).searchParams;
    const spu = params.get("spu") ?? "辅酶";
    const agency = params.get("agency") ?? "全部";
    const dimension = (params.get("dimension") ?? "audience") as Dimension;
    requested.push(spu + ":" + agency + ":" + dimension);
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: makeResult(spu, agency, dimension) })
    });
  });
}

test("renders content heatmap, filters and note drawer", async ({ page }) => {
  const requested: string[] = [];
  await mockCommon(page, requested);
  await page.goto("/paipai/content-analysis");

  await expect(page.getByRole("heading", { name: "内容分析" })).toBeVisible();
  await expect.poll(() => requested).toContain("辅酶:全部:audience");
  await expect(page.getByText("2026-07-29", { exact: true })).toBeVisible();
  await expect(page.getByText("内容类型覆盖")).toBeVisible();
  await expect(page.getByRole("button", { name: "科普 职场人 爆文率50%" })).toBeVisible();
  await expect(page.getByText("未标注", { exact: true })).toHaveCount(0);

  await page.getByRole("button", { name: "科普 职场人 爆文率50%" }).click();
  await expect(page.getByRole("heading", { name: "科普 × 职场人" })).toBeVisible();
  await expect(page.locator(".content-note-table tbody tr")).toHaveCount(2);
  await expect(page.getByRole("link", { name: /通勤精力管理实测/ })).toHaveAttribute("href", "https://www.xiaohongshu.com/explore/note-1");
  await page.getByRole("button", { name: "三项达标 1" }).click();
  await expect(page.locator(".content-note-table tbody tr")).toHaveCount(1);
  await page.getByRole("button", { name: "关闭", exact: true }).click();

  await page.getByRole("button", { name: "用户场景", exact: true }).click();
  await expect.poll(() => requested).toContain("辅酶:全部:scenario");
  await expect(page.getByRole("button", { name: "科普 精力疲惫 爆文率50%" })).toBeVisible();

  await page.getByRole("button", { name: "曼杰", exact: true }).click();
  await expect.poll(() => requested).toContain("辅酶:曼杰:scenario");

  await page.getByLabel("包含未标注").check();
  await expect(page.getByText("未标注", { exact: true }).first()).toBeVisible();

  await page.getByRole("button", { name: "磷虾油", exact: true }).click();
  await expect.poll(() => requested).toContain("磷虾油:曼杰:scenario");
});

test("content heatmap remains usable on mobile", async ({ page }) => {
  const requested: string[] = [];
  await mockCommon(page, requested);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/paipai/content-analysis");

  await expect(page.getByRole("heading", { name: "内容分析" })).toBeVisible();
  await expect(page.getByRole("button", { name: "科普 职场人 爆文率50%" })).toBeVisible();
  await expect(page.locator(".content-heatmap-scroll")).toBeVisible();
  await expect(page.locator("body")).toHaveCSS("overflow-x", "visible");
});
