import { expect, test, type Page } from "@playwright/test";

type Dimension = "audience" | "scenario";

function makeNote(id: string, overrides: Record<string, unknown> = {}) {
  return {
    note_id: id,
    title: id === "note-search" ? "通勤精力管理实测" : id === "note-feed" ? "一周辅酶记录" : "双场域笔记",
    url: "https://www.xiaohongshu.com/explore/" + id,
    author: "一颗栗子",
    published_date: "2026-07-20",
    agency: "智元",
    content_type: "科普",
    audience: "职场人",
    scenario: "精力疲惫",
    dandelion_cost: 12.5,
    boom: true,
    search_spend: 0,
    search_cost: null,
    latest_search_cost: null,
    search_cost_change: null,
    search_qualified: false,
    feed_spend: 0,
    feed_cost: null,
    feed_qualified: false,
    latest_search_spend: 0,
    latest_feed_spend: 0,
    search_campaigns: [],
    feed_campaigns: [],
    search_stopped: false,
    feed_stopped: false,
    ...overrides
  };
}

function makeResult(spu: string, agency: string, dimension: Dimension, publishedStartDate: string, publishedEndDate: string) {
  const searchNotes = Array.from({ length: 21 }, (_, index) => makeNote("search-" + String(index + 1).padStart(2, "0"), {
    title: "搜索笔记 " + (index + 1),
    search_spend: 400 - index,
    search_cost: index === 0 ? 40 : 20,
    latest_search_cost: index === 0 ? 55 : 20,
    search_cost_change: index === 0 ? 15 : 0,
    search_qualified: index !== 0,
    search_stopped: index === 1
  }));
  const notes = [
    makeNote("note-search", {
      search_spend: 480, search_cost: 24, latest_search_cost: 28, search_cost_change: 4, search_qualified: true, latest_search_spend: 12,
      search_campaigns: [{ name: "通勤搜索放量计划", advertiser_id: 9001, advertiser_name: "辅酶聚光账户", campaign_id: 81001, filter_state: 1, enable: 1, synced_at: "2026-08-22T03:30:00+08:00" }]
    }),
    makeNote("note-feed", {
      title: "一周辅酶记录", feed_spend: 500, feed_cost: 88, feed_qualified: false, feed_stopped: true,
      feed_campaigns: [{ name: "一周辅酶信息流计划", advertiser_id: 9001, advertiser_name: "辅酶聚光账户", campaign_id: 82001, filter_state: 2, enable: 0, synced_at: "2026-08-22T03:30:00+08:00" }]
    }),
    makeNote("note-both", {
      title: "双场域笔记", search_spend: 80, search_cost: 18, latest_search_cost: 16, search_cost_change: -2,
      search_qualified: true, feed_spend: 90, feed_cost: 50, feed_qualified: true
    }),
    ...searchNotes
  ];
  return {
    spu,
    agency,
    dimension,
    published_start_date: publishedStartDate,
    published_end_date: publishedEndDate,
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
      total_notes: notes.length,
      content_type_tagged: notes.length,
      audience_tagged: notes.length,
      scenario_tagged: notes.length,
      dandelion_cost_notes: notes.length,
      flow_evaluated_notes: notes.length,
      roi_evaluated_notes: notes.length,
      all_metrics_notes: notes.length
    },
    types: ["科普"],
    dimensions: ["职场人"],
    cells: [{
      content_type: "科普",
      dimension: "职场人",
      total_notes: notes.length,
      dandelion_eligible: notes.length,
      boom_count: notes.length,
      boom_rate: 1,
      flow_evaluated: notes.length,
      flow_qualified: notes.length,
      roi_evaluated: notes.length,
      roi_qualified: notes.length,
      all_qualified: notes.length,
      notes
    }]
  };
}

async function mockCommon(page: Page, requested: string[], statusPosts: Array<Record<string, unknown>> = []) {
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, body: "ok" }));
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({
    status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] })
  }));
  await page.route("**/paipai/api/delivery/campaigns/status", async (route) => {
    const body = route.request().postDataJSON() as Record<string, unknown>;
    statusPosts.push(body);
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({
      success: true,
      data: { advertiser_id: body.advertiser_id, action_type: body.action_type, campaign_ids: body.campaign_ids }
    }) });
  });
  await page.route("**/paipai/api/analytics/content-analysis?*", async (route) => {
    const params = new URL(route.request().url()).searchParams;
    const spu = params.get("spu") ?? "辅酶";
    const agency = params.get("agency") ?? "全部";
    const dimension = (params.get("dimension") ?? "audience") as Dimension;
    const publishedStartDate = params.get("published_start_date") ?? "";
    const publishedEndDate = params.get("published_end_date") ?? "";
    requested.push([spu, agency, dimension, publishedStartDate, publishedEndDate].join(":"));
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: makeResult(spu, agency, dimension, publishedStartDate, publishedEndDate) })
    });
  });
}

test("search placement page lists note-level search aggregates with narrowed columns", async ({ page }) => {
  const requested: string[] = [];
  const statusPosts: Array<Record<string, unknown>> = [];
  await mockCommon(page, requested, statusPosts);
  await page.goto("/paipai/delivery/search");

  await expect(page.getByRole("heading", { name: "搜索", exact: true })).toBeVisible();
  await expect(page.locator(".breadcrumb")).toContainText("投放管理");
  await expect(page.locator(".breadcrumb")).toContainText("搜索");
  await expect(page.locator(".nav-item.active")).toHaveText("搜索");
  await expect.poll(() => requested).toContain("辅酶:全部:audience::");
  await expect(page.getByText("默认按搜索累计消耗降序；计划来自聚光笔记关联，消耗与成本保留为日报笔记场域口径。双击计划可修改状态。")).toBeVisible();
  await expect(page.getByRole("button", { name: "搜索累计消耗" })).toHaveClass(/active/);

  const table = page.getByRole("table", { name: "按搜索累计消耗排序的笔记" });
  await expect(table.getByRole("columnheader")).toHaveText(["", "笔记", "机构与标签", "聚光计划", "站外成本 15 天", "搜索累计消耗 · 成本", "回搜成本变化"]);
  await expect(table).not.toContainText("薯量 ROI");
  await expect(table).not.toContainText("信息流累计消耗");
  await expect(table.getByRole("columnheader", { name: "聚光计划", exact: true })).toBeVisible();
  await expect(table.getByRole("checkbox", { name: "全选本页笔记计划" })).toBeVisible();
  await expect(table.locator("tbody tr")).toHaveCount(20);
  const firstSearchRow = table.locator("tbody tr").first();
  await expect(firstSearchRow).toContainText("通勤精力管理实测");
  await expect(firstSearchRow).toContainText("通勤搜索放量计划");
  await expect(firstSearchRow).toContainText("辅酶聚光账户 · 81001");
  await expect(firstSearchRow).toContainText("有效");
  await expect(firstSearchRow.getByRole("link", { name: "查看计划 通勤搜索放量计划 详情" })).toHaveAttribute("href", "/paipai/delivery/campaigns?advertiser_id=9001&campaign_id=81001");
  await expect(firstSearchRow).toContainText("¥480.00 · ¥24.00");
  await expect.poll(async () => firstSearchRow.evaluate((row) => {
    const identity = row.querySelector(".content-note-identity");
    const labels = row.querySelector(".placement-note-labels");
    const metric = row.querySelector(".content-note-metric");
    if (!(identity instanceof HTMLElement) || !(labels instanceof HTMLElement) || !(metric instanceof HTMLElement)) return false;
    const visibleBox = (element: HTMLElement) => {
      const rect = element.getBoundingClientRect();
      return rect.width > 8 && rect.height > 8 && getComputedStyle(element).opacity !== "0" && getComputedStyle(element).visibility !== "hidden";
    };
    return visibleBox(identity) && visibleBox(labels) && visibleBox(metric) && row.querySelectorAll("td").length === 7;
  })).toBe(true);
  await table.getByRole("checkbox", { name: "选择计划 通勤搜索放量计划" }).check();
  await page.getByRole("button", { name: "一键暂停 1" }).click();
  await expect(page.getByRole("dialog", { name: "一键暂停已选计划" })).toBeVisible();
  await page.getByRole("button", { name: "暂停 1 个计划" }).click();
  await expect.poll(() => statusPosts).toEqual([{ advertiser_id: 9001, campaign_ids: [81001], action_type: 2 }]);
  await expect(page.getByText("已暂停 1 个计划，并刷新状态")).toBeVisible();
  await expect(table.getByRole("link", { name: "查看笔记场域分析 note-search" })).toHaveAttribute("href", "/paipai/note-campaign-analysis?q=note-search");
  await expect(table).not.toContainText("一周辅酶记录");
  await expect(page.getByRole("button", { name: "搜索成本不达标" })).toContainText("1");
  await expect(page.getByRole("button", { name: "搜索未停投" })).toContainText("22");
  await expect(page.getByRole("button", { name: "信息流成本不达标" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "信息流未停投" })).toHaveCount(0);

  await page.getByRole("button", { name: "回搜成本变化" }).click();
  const changeTable = page.getByRole("table", { name: "按回搜成本变化排序的笔记" });
  await expect(changeTable.locator("tbody tr").first()).toContainText("搜索笔记 1");
  await expect(changeTable.locator("tbody tr").first()).toContainText("+¥15.00");

  await page.getByRole("button", { name: "信息流累计消耗" }).click();
  const feedSorted = page.getByRole("table", { name: "按信息流累计消耗排序的笔记" });
  await expect(feedSorted.locator("tbody tr").first()).toContainText("双场域笔记");
  await expect(feedSorted).not.toContainText("一周辅酶记录");

  await page.getByRole("button", { name: "搜索累计消耗" }).click();
  await page.getByRole("button", { name: "搜索成本不达标" }).click();
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" }).locator("tbody tr")).toHaveCount(1);
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" })).toContainText("搜索笔记 1");
  await page.getByLabel("搜索成本不达标阈值").fill("50");
  await expect(page.getByText("当前筛选条件下暂无搜索笔记")).toBeVisible();
  await page.getByRole("button", { name: "搜索成本不达标" }).click();
  await page.getByRole("button", { name: "搜索未停投" }).click();
  await expect(page.getByText("共 22 篇 · 每页 20 篇")).toBeVisible();
  await expect(page.getByRole("link", { name: "查看笔记场域分析 search-02" })).toHaveCount(0);

  const noteIDSearch = page.getByLabel("按笔记 ID 搜索");
  const filterCards = page.getByLabel("笔记表现筛选");
  await expect.poll(async () => {
    const searchBox = await noteIDSearch.boundingBox();
    const cards = await filterCards.boundingBox();
    return searchBox && cards ? searchBox.x >= cards.x + cards.width - 8 : false;
  }).toBe(true);
  await page.getByRole("button", { name: "搜索未停投" }).click();
  await noteIDSearch.fill("note-search");
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" }).locator("tbody tr")).toHaveCount(1);
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" })).toContainText("通勤精力管理实测");
  await page.getByRole("button", { name: "清除笔记 ID 搜索" }).click();
  await expect(page.getByText("共 23 篇 · 每页 20 篇")).toBeVisible();

  await page.getByRole("button", { name: "曼杰", exact: true }).click();
  await expect.poll(() => requested.at(-1)).toBe("辅酶:曼杰:audience::");
  await page.getByRole("button", { name: "磷虾油", exact: true }).click();
  await expect.poll(() => requested.at(-1)).toBe("磷虾油:曼杰:audience::");
});

test("feed placement page lists note-level feed aggregates without ROI or search columns", async ({ page }) => {
  const requested: string[] = [];
  await mockCommon(page, requested);
  await page.goto("/paipai/delivery/feed");

  await expect(page.getByRole("heading", { name: "信息流" })).toBeVisible();
  await expect(page.locator(".nav-item.active")).toHaveText("信息流");
  await expect.poll(() => requested).toContain("辅酶:全部:audience::");
  await expect(page.getByText("默认按信息流累计消耗降序；计划来自聚光笔记关联，消耗与成本保留为日报笔记场域口径。双击计划可修改状态。")).toBeVisible();
  await expect(page.getByRole("button", { name: "信息流累计消耗" })).toHaveClass(/active/);

  const table = page.getByRole("table", { name: "按信息流累计消耗排序的笔记" });
  await expect(table.getByRole("columnheader")).toHaveText(["", "笔记", "机构与标签", "聚光计划", "站外成本 15 天", "信息流累计消耗 · 成本"]);
  await expect(table).not.toContainText("薯量 ROI");
  await expect(table).not.toContainText("搜索累计消耗");
  await expect(table).not.toContainText("回搜成本变化");
  await expect(table.getByRole("columnheader", { name: "聚光计划", exact: true })).toBeVisible();
  await expect(table).toContainText("一周辅酶信息流计划");
  await expect(table).toContainText("辅酶聚光账户 · 82001");
  await expect(table).toContainText("暂停");
  await expect(table.getByRole("link", { name: "查看计划 一周辅酶信息流计划 详情" })).toHaveAttribute("href", "/paipai/delivery/campaigns?advertiser_id=9001&campaign_id=82001");
  await expect(table.getByRole("checkbox", { name: "选择计划 一周辅酶信息流计划" })).toBeVisible();
  await expect(table.locator("tbody tr")).toHaveCount(2);
  await expect(table.locator("tbody tr").first()).toContainText("一周辅酶记录");
  await expect(table.locator("tbody tr").first()).toContainText("¥500.00 · ¥88.00");
  await expect(table.locator("tbody tr").first().locator(".content-note-stopped")).toHaveText("已停投");
  await expect(table).toContainText("双场域笔记");
  await expect(table).not.toContainText("通勤精力管理实测");
  await expect(page.getByRole("button", { name: "信息流成本不达标" })).toContainText("1");
  await expect(page.getByRole("button", { name: "信息流未停投" })).toContainText("1");
  await expect(page.getByRole("button", { name: "搜索成本不达标" })).toHaveCount(0);

  await page.getByRole("button", { name: "信息流未停投" }).click();
  await expect(table.locator("tbody tr")).toHaveCount(1);
  await expect(table).toContainText("双场域笔记");
  await expect(table).not.toContainText("一周辅酶记录");
  await page.getByRole("button", { name: "信息流未停投" }).click();

  await page.getByRole("button", { name: "搜索累计消耗" }).click();
  const searchSorted = page.getByRole("table", { name: "按搜索累计消耗排序的笔记" });
  await expect(searchSorted.locator("tbody tr").first()).toContainText("双场域笔记");
  await expect(searchSorted).not.toContainText("通勤精力管理实测");
});

test("placement note tables remain usable on mobile", async ({ page }) => {
  const requested: string[] = [];
  await mockCommon(page, requested);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/paipai/delivery/search");

  await expect(page.getByRole("heading", { name: "搜索", exact: true })).toBeVisible();
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" })).toBeVisible();
  const searchTableScroll = page.locator(".content-note-section .content-note-table-wrap");
  expect(await searchTableScroll.evaluate((element) => element.scrollWidth > element.clientWidth)).toBe(true);
  await expect(page.locator("body")).toHaveCSS("overflow-x", "visible");
  expect(await page.evaluate(() => document.body.scrollWidth <= window.innerWidth)).toBe(true);

  await page.goto("/paipai/delivery/feed");
  await expect(page.getByRole("heading", { name: "信息流" })).toBeVisible();
  await expect(page.getByRole("table", { name: "按信息流累计消耗排序的笔记" })).toBeVisible();
  const feedTableScroll = page.locator(".content-note-section .content-note-table-wrap");
  expect(await feedTableScroll.evaluate((element) => element.scrollWidth > element.clientWidth)).toBe(true);
  expect(await page.evaluate(() => document.body.scrollWidth <= window.innerWidth)).toBe(true);
});
