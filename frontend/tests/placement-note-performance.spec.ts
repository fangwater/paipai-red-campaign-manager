import { expect, test, type Page } from "@playwright/test";

type Dimension = "audience" | "scenario";

function campaign(
  name: string,
  spend: number,
  cost: number | null,
  latestSpend = spend,
  extras: { advertiser_id?: number | null; campaign_id?: number | null; filter_state?: number | null; enable?: number | null } = {}
) {
  const campaignID = extras.campaign_id ?? null;
  return {
    name,
    spend,
    cost,
    latest_spend: latestSpend,
    advertiser_id: extras.advertiser_id ?? (campaignID ? 9001 : null),
    campaign_id: campaignID,
    filter_state: extras.filter_state ?? null,
    enable: extras.enable ?? null
  };
}

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
      search_campaigns: [
        campaign("辅酶搜索计划", 360, 22, 10, { campaign_id: 101, filter_state: 1, enable: 1 }),
        campaign("回搜计划", 120, 28, 2, { campaign_id: 102, filter_state: 2, enable: 0 }),
        campaign("未匹配搜索计划", 8, null, 0)
      ],
      feed_campaigns: [campaign("辅酶信息流计划", 80, 40, 5, { campaign_id: 201, filter_state: 1, enable: 1 })]
    }),
    makeNote("note-feed", {
      title: "一周辅酶记录", feed_spend: 500, feed_cost: 88, feed_qualified: false, feed_stopped: true,
      feed_campaigns: [
        campaign("辅酶信息流计划", 320, 90, 0, { campaign_id: 201, filter_state: 2, enable: 0 }),
        campaign("信息流放大计划", 180, 70, 0, { campaign_id: 202, filter_state: 8, enable: 0 })
      ],
      search_campaigns: [campaign("辅酶搜索计划", 40, 20, 1, { campaign_id: 101, filter_state: 1, enable: 1 })]
    }),
    makeNote("note-both", {
      title: "双场域笔记", search_spend: 80, search_cost: 18, latest_search_cost: 16, search_cost_change: -2,
      search_qualified: true, feed_spend: 90, feed_cost: 50, feed_qualified: true,
      search_campaigns: [campaign("双场域搜索计划", 80, 18, 8, { campaign_id: 301, filter_state: 1, enable: 1 })],
      feed_campaigns: [campaign("双场域信息流计划", 90, 50, 12, { campaign_id: 302, filter_state: 4, enable: 1 })]
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

async function mockCommon(page: Page, requested: string[], statusPosts: unknown[] = []) {
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, body: "ok" }));
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({
    status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] })
  }));
  await page.route("**/paipai/api/delivery/campaigns/status", async (route) => {
    const body = route.request().postDataJSON();
    statusPosts.push(body);
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          advertiser_id: body.advertiser_id,
          action_type: body.action_type,
          requested_campaign_ids: body.campaign_ids,
          campaign_ids: body.campaign_ids,
          gateway: { operation: "campaign.status", data: {}, request_hash: "test", latency_ms: 1 }
        }
      })
    });
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

test("search placement page lists only search notes with narrowed columns", async ({ page }) => {
  const requested: string[] = [];
  await mockCommon(page, requested);
  await page.goto("/paipai/delivery/search");

  await expect(page.getByRole("heading", { name: "搜索", exact: true })).toBeVisible();
  await expect(page.locator(".breadcrumb")).toContainText("投放管理");
  await expect(page.locator(".breadcrumb")).toContainText("搜索");
  await expect(page.locator(".nav-item.active")).toHaveText("搜索");
  await expect.poll(() => requested).toContain("辅酶:全部:audience::");
  await expect(page.getByText("默认按搜索累计消耗降序；回搜成本变化 = 当日回搜成本 − 累计回搜成本")).toBeVisible();
  await expect(page.getByRole("button", { name: "搜索累计消耗" })).toHaveClass(/active/);

  const table = page.getByRole("table", { name: "按搜索累计消耗排序的笔记" });
  await expect(table.getByRole("columnheader")).toHaveText(["", "笔记", "机构与标签", "计划", "站外成本 15 天", "搜索累计消耗 · 成本", "回搜成本变化"]);
  await expect(table).not.toContainText("薯量 ROI");
  await expect(table).not.toContainText("信息流累计消耗");
  await expect(table.locator("tbody tr")).toHaveCount(20);
  await expect(table.locator("tbody tr").first()).toContainText("通勤精力管理实测");
  await expect(table.locator("tbody tr").first()).toContainText("辅酶搜索计划");
  await expect(table.locator("tbody tr").first()).toContainText("回搜计划");
  await expect(table.locator("tbody tr").first()).toContainText("未匹配搜索计划");
  await expect(table.locator("tbody tr").first().locator(".placement-campaign-state.healthy").first()).toHaveText("有效");
  await expect(table.locator("tbody tr").first().locator(".placement-campaign-state.paused").first()).toHaveText("暂停");
  await expect(table.locator("tbody tr").first()).toContainText("消耗 ¥360.00");
  await expect(table.locator("tbody tr").first()).toContainText("成本 ¥22.00");
  await expect(table.locator("tbody tr").first()).not.toContainText("辅酶信息流计划");
  await expect(table.getByRole("checkbox", { name: "全选本页笔记计划" })).toBeVisible();
  await expect(table.getByRole("checkbox", { name: "选择笔记 note-search" })).toBeVisible();
  await expect(table.getByRole("checkbox", { name: "选择计划 辅酶搜索计划" })).toBeVisible();
  const firstSearchRow = table.locator("tbody tr").first();
  await expect.poll(async () => firstSearchRow.evaluate((row) => {
    const checkbox = row.querySelector("td:first-child");
    const labels = row.querySelector(".placement-note-labels");
    const campaigns = row.querySelector(".placement-note-campaigns");
    const title = row.querySelector(".content-note-title");
    const state = row.querySelector(".placement-campaign-state");
    if (!(checkbox instanceof HTMLElement) || !(labels instanceof HTMLElement) || !(campaigns instanceof HTMLElement) || !(title instanceof HTMLElement) || !(state instanceof HTMLElement)) return false;
    const visibleBox = (element: HTMLElement) => {
      const rect = element.getBoundingClientRect();
      return rect.width > 8 && rect.height > 8 && getComputedStyle(element).opacity !== "0" && getComputedStyle(element).visibility !== "hidden";
    };
    return checkbox.getBoundingClientRect().width <= 24
      && visibleBox(labels)
      && visibleBox(campaigns)
      && visibleBox(title)
      && visibleBox(state)
      && row.getBoundingClientRect().height > 70
      && campaigns.querySelectorAll(".placement-campaign-card").length >= 2;
  })).toBe(true);
  await table.getByRole("checkbox", { name: "选择笔记 note-search" }).check();
  await expect(table.getByRole("checkbox", { name: "选择计划 辅酶搜索计划" })).toBeChecked();
  await expect(table.getByRole("checkbox", { name: "选择计划 回搜计划" })).toBeChecked();
  await expect(table.getByRole("checkbox", { name: "选择计划 未匹配搜索计划" })).toBeChecked();
  await expect(page.getByText("已选 3 个计划")).toBeVisible();
  await table.getByRole("checkbox", { name: "选择计划 回搜计划" }).uncheck();
  await expect(page.getByText("已选 2 个计划")).toBeVisible();
  await table.getByRole("checkbox", { name: "全选本页笔记计划" }).check();
  await expect(page.getByText(/已选 \d+ 个计划/)).toBeVisible();
  await expect(table.getByRole("checkbox", { name: "选择计划 辅酶搜索计划" })).toBeChecked();
  await expect(table.getByRole("link", { name: "查看笔记计划分析 note-search" })).toHaveAttribute("href", "/paipai/note-campaign-analysis?q=note-search");
  await expect(table).not.toContainText("一周辅酶记录");
  await expect(page.getByRole("button", { name: "搜索成本不达标" })).toContainText("1");
  await expect(page.getByRole("button", { name: "搜索已停投" })).toContainText("1");
  await expect(page.getByRole("button", { name: "信息流成本不达标" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "信息流已停投" })).toHaveCount(0);

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
  await page.getByRole("button", { name: "搜索已停投" }).click();
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" }).locator("tbody tr")).toHaveCount(1);
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" })).toContainText("search-02");

  const noteIDSearch = page.getByLabel("按笔记 ID 搜索");
  const filterCards = page.getByLabel("笔记表现筛选");
  await expect.poll(async () => {
    const searchBox = await noteIDSearch.boundingBox();
    const cards = await filterCards.boundingBox();
    return searchBox && cards ? searchBox.x >= cards.x + cards.width - 8 : false;
  }).toBe(true);
  await page.getByRole("button", { name: "搜索已停投" }).click();
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

test("feed placement page lists only feed notes without ROI or search columns", async ({ page }) => {
  const requested: string[] = [];
  await mockCommon(page, requested);
  await page.goto("/paipai/delivery/feed");

  await expect(page.getByRole("heading", { name: "信息流" })).toBeVisible();
  await expect(page.locator(".nav-item.active")).toHaveText("信息流");
  await expect.poll(() => requested).toContain("辅酶:全部:audience::");
  await expect(page.getByText("默认按信息流累计消耗降序；回搜成本变化 = 当日回搜成本 − 累计回搜成本")).toBeVisible();
  await expect(page.getByRole("button", { name: "信息流累计消耗" })).toHaveClass(/active/);

  const table = page.getByRole("table", { name: "按信息流累计消耗排序的笔记" });
  await expect(table.getByRole("columnheader")).toHaveText(["", "笔记", "机构与标签", "计划", "站外成本 15 天", "信息流累计消耗 · 成本"]);
  await expect(table).not.toContainText("薯量 ROI");
  await expect(table).not.toContainText("搜索累计消耗");
  await expect(table).not.toContainText("回搜成本变化");
  await expect(table.locator("tbody tr")).toHaveCount(2);
  await expect(table.locator("tbody tr").first()).toContainText("一周辅酶记录");
  await expect(table.locator("tbody tr").first()).toContainText("辅酶信息流计划");
  await expect(table.locator("tbody tr").first()).toContainText("信息流放大计划");
  await expect(table.locator("tbody tr").first().locator(".placement-campaign-state.paused").first()).toHaveText("暂停");
  await expect(table.locator("tbody tr").first()).toContainText("暂停阶段");
  await expect(table.locator("tbody tr").first()).toContainText("消耗 ¥320.00");
  await expect(table.locator("tbody tr").first()).not.toContainText("辅酶搜索计划");
  await expect(table.locator("tbody tr").first().locator(".content-note-stopped")).toHaveText("已停投");
  await expect(table.locator("tbody tr").first().locator(".placement-campaign-card.stopped").first()).toContainText("近一天 0");
  await expect(table.getByRole("checkbox", { name: "选择笔记 note-feed" })).toBeVisible();
  await expect(table.getByRole("checkbox", { name: "选择计划 辅酶信息流计划" })).toBeVisible();
  await expect(table).toContainText("双场域笔记");
  await expect(table).toContainText("双场域信息流计划");
  await expect(table).not.toContainText("通勤精力管理实测");
  await table.getByRole("checkbox", { name: "选择笔记 note-feed" }).check();
  await expect(table.getByRole("checkbox", { name: "选择计划 辅酶信息流计划" })).toBeChecked();
  await expect(table.getByRole("checkbox", { name: "选择计划 信息流放大计划" })).toBeChecked();
  await expect(page.getByText("已选 2 个计划")).toBeVisible();
  await expect(page.getByRole("button", { name: "信息流成本不达标" })).toContainText("1");
  await expect(page.getByRole("button", { name: "信息流已停投" })).toContainText("1");
  await expect(page.getByRole("button", { name: "搜索成本不达标" })).toHaveCount(0);

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

test("search placement can pause selected campaigns and edit one by double click", async ({ page }) => {
  const requested: string[] = [];
  const statusPosts: unknown[] = [];
  await mockCommon(page, requested, statusPosts);
  await page.goto("/paipai/delivery/search");

  const table = page.getByRole("table", { name: "按搜索累计消耗排序的笔记" });
  await expect(page.getByRole("button", { name: "一键暂停", exact: true })).toBeDisabled();
  await table.getByRole("checkbox", { name: "选择计划 辅酶搜索计划" }).check();
  await table.getByRole("checkbox", { name: "选择计划 未匹配搜索计划" }).check();
  await expect(page.getByRole("button", { name: "一键暂停 1", exact: true })).toBeEnabled();
  await page.getByRole("button", { name: "一键暂停 1", exact: true }).click();
  const pauseDialog = page.getByRole("dialog", { name: "一键暂停已选计划" });
  await expect(pauseDialog).toContainText("已忽略 1 个未匹配计划");
  await pauseDialog.getByRole("button", { name: "暂停 1 个计划" }).click();
  await expect.poll(() => statusPosts).toEqual([{ advertiser_id: 9001, campaign_ids: [101], action_type: 2 }]);
  await expect(page.getByText("已暂停 1 个计划，并刷新状态")).toBeVisible();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "一键暂停", exact: true })).toBeDisabled();

  await table.getByText("回搜计划", { exact: true }).dblclick();
  const editDialog = page.getByRole("dialog", { name: "修改计划状态" });
  await expect(editDialog).toContainText("当前状态：暂停");
  await editDialog.getByRole("radio", { name: "有效" }).check();
  await editDialog.getByRole("button", { name: "设为有效" }).click();
  await expect.poll(() => statusPosts.at(-1)).toEqual({ advertiser_id: 9001, campaign_ids: [102], action_type: 1 });
  await expect(page.getByText("计划「回搜计划」已设为有效，并刷新状态")).toBeVisible();
});
