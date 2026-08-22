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
    latest_search_cost: id === "note-1" ? 28 : 55,
    search_cost_change: id === "note-1" ? 4 : 15,
    search_qualified: id === "note-1",
    feed_spend: id === "note-2" ? 500 : 0,
    feed_cost: id === "note-2" ? 88 : null,
    feed_qualified: false,
    latest_search_spend: id === "note-2" ? 8 : 12,
    latest_feed_spend: id === "note-2" ? 0 : 6,
    search_stopped: false,
    feed_stopped: id === "note-2",
    flow_evaluated: id === "note-1",
    flow_qualified: id === "note-1",
    roi: id === "note-1" ? 1.8 : 0.4,
    roi_qualified: id === "note-1",
    all_qualified: id === "note-1",
    ...overrides
  };
}

function makeResult(spu: string, agency: string, dimension: Dimension, publishedStartDate: string, publishedEndDate: string) {
  const primaryDimension = dimension === "audience" ? "职场人" : "精力疲惫";
  const secondDimension = dimension === "audience" ? "健身人" : "运动恢复";
  const notes = [makeNote("note-1"), makeNote("note-2")];
  const paginationNotes = Array.from({ length: 21 }, (_, index) => makeNote("page-" + String(index + 1).padStart(2, "0"), {
    title: "分页笔记 " + (index + 1),
    content_type: "经验分享",
    search_spend: 39 - index,
    search_cost: 20,
    latest_search_cost: 20,
    search_cost_change: 0,
    feed_spend: 0,
    feed_cost: null
  }));
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
      total_notes: 25,
      content_type_tagged: 24,
      audience_tagged: 24,
      scenario_tagged: 24,
      dandelion_cost_notes: 24,
      flow_evaluated_notes: 1,
      roi_evaluated_notes: 23,
      all_metrics_notes: 1
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
        notes: [makeNote("note-3", { content_type: "经验分享", audience: "健身人", scenario: "运动恢复", search_spend: 80, search_cost: null, latest_search_cost: null, search_cost_change: null, latest_search_spend: 0, search_stopped: true, feed_spend: 90, feed_cost: null, roi: null, roi_qualified: false, all_qualified: false })]
      },
      {
        content_type: "经验分享",
        dimension: primaryDimension,
        total_notes: paginationNotes.length,
        dandelion_eligible: paginationNotes.length,
        boom_count: 0,
        boom_rate: 0,
        flow_evaluated: 0,
        flow_qualified: 0,
        roi_evaluated: paginationNotes.length,
        roi_qualified: 0,
        all_qualified: 0,
        notes: paginationNotes
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

test("renders content heatmap, filters and note drawer", async ({ page }) => {
  const requested: string[] = [];
  await mockCommon(page, requested);
  await page.goto("/paipai/content-analysis");

  await expect(page.getByRole("heading", { name: "内容分析" })).toBeVisible();
  await expect.poll(() => requested).toContain("辅酶:全部:audience::");
  await expect(page.getByText("2026-07-29", { exact: true })).toBeVisible();
  await expect(page.getByText("内容类型覆盖")).toBeVisible();
  await expect(page.getByRole("button", { name: "科普 职场人 爆文率50%" })).toBeVisible();
  await expect(page.getByText("未标注", { exact: true })).toHaveCount(0);

  const publishedStartDate = page.getByLabel("发布时间开始");
  const publishedEndDate = page.getByLabel("发布时间结束");
  await publishedStartDate.fill("2026-07-01");
  await publishedEndDate.fill("2026-07-31");
  await expect.poll(() => requested.at(-1)).toBe("辅酶:全部:audience:2026-07-01:2026-07-31");
  await page.getByRole("button", { name: "清除发布时间范围" }).click();
  await expect(publishedStartDate).toHaveValue("");
  await expect(publishedEndDate).toHaveValue("");
  await expect.poll(() => requested.at(-1)).toBe("辅酶:全部:audience::");

  await expect(page.getByText("默认按搜索累计消耗降序；回搜成本变化 = 当日回搜成本 − 累计回搜成本")).toBeVisible();
  const sortedNotes = page.getByRole("table", { name: "按搜索累计消耗排序的笔记" });
  await expect(sortedNotes.getByRole("columnheader")).toHaveText(["笔记", "机构与标签", "站外成本 15 天", "搜索累计消耗 · 成本", "信息流累计消耗 · 成本", "回搜成本变化", "薯量 ROI"]);
  await expect(sortedNotes.locator("tbody tr")).toHaveCount(20);
  await expect(sortedNotes.locator("tbody tr").first()).toContainText("通勤精力管理实测");
  await expect(sortedNotes.getByRole("link", { name: "查看笔记场域分析 note-1" })).toHaveAttribute("href", "/paipai/note-campaign-analysis?q=note-1");
  await expect(sortedNotes.locator("tbody tr").first().locator(".content-note-stopped")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "搜索累计消耗" })).toHaveClass(/active/);
  await page.getByRole("button", { name: "信息流累计消耗" }).click();
  const feedSortedNotes = page.getByRole("table", { name: "按信息流累计消耗排序的笔记" });
  await expect(feedSortedNotes.locator("tbody tr").first()).toContainText("一周辅酶记录");
  await page.getByRole("button", { name: "回搜成本变化" }).click();
  const changeSortedNotes = page.getByRole("table", { name: "按回搜成本变化排序的笔记" });
  await expect(changeSortedNotes.locator("tbody tr").first()).toContainText("一周辅酶记录");
  await expect(changeSortedNotes.locator("tbody tr").first()).toContainText("+¥15.00");
  await page.getByRole("button", { name: "搜索累计消耗" }).click();
  await expect(page.getByRole("button", { name: "搜索成本不达标" })).toContainText("2");
  await expect(page.getByRole("button", { name: "信息流成本不达标" })).toContainText("2");
  await expect(page.getByRole("button", { name: "搜索已停投" })).toContainText("1");
  await expect(page.getByRole("button", { name: "信息流已停投" })).toContainText("1");
  await page.getByRole("button", { name: "搜索成本不达标" }).click();
  const searchUnqualifiedNotes = page.getByRole("table", { name: "按搜索累计消耗排序的笔记" });
  await expect(searchUnqualifiedNotes.locator("tbody tr")).toHaveCount(2);
  await expect(searchUnqualifiedNotes).toContainText("一周辅酶记录");
  await expect(searchUnqualifiedNotes).toContainText("暂无成本");
  await page.getByLabel("搜索成本不达标阈值").fill("50");
  await expect(searchUnqualifiedNotes.locator("tbody tr")).toHaveCount(1);
  await expect(searchUnqualifiedNotes).toContainText("暂无成本");
  await page.getByRole("button", { name: "搜索成本不达标" }).click();
  await page.getByRole("button", { name: "信息流成本不达标" }).click();
  const feedUnqualifiedNotes = page.getByRole("table", { name: "按搜索累计消耗排序的笔记" });
  await expect(feedUnqualifiedNotes.locator("tbody tr")).toHaveCount(2);
  await expect(feedUnqualifiedNotes).toContainText("一周辅酶记录");
  await expect(feedUnqualifiedNotes).toContainText("暂无成本");
  await page.getByRole("button", { name: "信息流成本不达标" }).click();
  await page.getByRole("button", { name: "信息流已停投" }).click();
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" }).locator("tbody tr")).toHaveCount(1);
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" })).toContainText("一周辅酶记录");
  const feedStoppedRow = page.getByRole("table", { name: "按搜索累计消耗排序的笔记" }).locator("tbody tr").first();
  await expect(feedStoppedRow.locator("td").nth(3).locator(".content-note-stopped")).toHaveCount(0);
  await expect(feedStoppedRow.locator("td").nth(4).locator(".content-note-stopped")).toHaveText("已停投");
  await page.getByRole("button", { name: "信息流已停投" }).click();
  await page.getByRole("button", { name: "搜索已停投" }).click();
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" }).locator("tbody tr")).toHaveCount(1);
  const searchStoppedRow = page.getByRole("table", { name: "按搜索累计消耗排序的笔记" }).locator("tbody tr").first();
  await expect(searchStoppedRow).toContainText("note-3");
  await expect(searchStoppedRow.locator("td").nth(3).locator(".content-note-stopped")).toHaveText("已停投");
  await expect(searchStoppedRow.locator("td").nth(4).locator(".content-note-stopped")).toHaveCount(0);
  await page.getByRole("button", { name: "搜索已停投" }).click();
  const noteIDSearch = page.getByLabel("按笔记 ID 搜索");
  const filterCards = page.getByLabel("笔记表现筛选");
  await expect(noteIDSearch).toBeVisible();
  await expect.poll(async () => {
    const searchBox = await noteIDSearch.boundingBox();
    const cards = await filterCards.boundingBox();
    return searchBox && cards ? searchBox.x >= cards.x + cards.width - 8 : false;
  }).toBe(true);
  await noteIDSearch.fill("note-2");
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" }).locator("tbody tr")).toHaveCount(1);
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" })).toContainText("一周辅酶记录");
  await page.getByRole("button", { name: "信息流已停投" }).click();
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" }).locator("tbody tr")).toHaveCount(1);
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" })).toContainText("一周辅酶记录");
  await page.getByRole("button", { name: "信息流已停投" }).click();
  await page.getByRole("button", { name: "清除笔记 ID 搜索" }).click();
  await expect(noteIDSearch).toHaveValue("");
  await expect(page.getByText("共 24 篇 · 每页 20 篇")).toBeVisible();
  const pageSelect = page.getByLabel("选择笔记页码");
  await expect(pageSelect).toHaveValue("1");
  await expect(pageSelect.locator("option")).toHaveCount(2);
  const nextPage = page.getByRole("button", { name: "下一页" });
  await nextPage.click();
  await expect(pageSelect).toHaveValue("2");
  await expect(sortedNotes.locator("tbody tr")).toHaveCount(4);
  await expect(nextPage).toBeDisabled();
  await expect(page.getByRole("button", { name: "上一页" })).toBeEnabled();

  await page.getByRole("button", { name: "科普 职场人 爆文率50%" }).click();
  await expect(page.getByRole("heading", { name: "科普 × 职场人" })).toBeVisible();
  const drawerNotes = page.getByRole("table", { name: "热力图笔记明细" });
  await expect(drawerNotes.locator("tbody tr")).toHaveCount(2);
  await expect(drawerNotes.getByRole("link", { name: /通勤精力管理实测/ })).toHaveAttribute("href", "https://www.xiaohongshu.com/explore/note-1");
  await expect(drawerNotes.getByRole("link", { name: "查看笔记场域分析 note-1" })).toHaveAttribute("href", "/paipai/note-campaign-analysis?q=note-1");
  await page.getByRole("button", { name: "三项达标 1" }).click();
  await expect(drawerNotes.locator("tbody tr")).toHaveCount(1);
  await page.getByRole("button", { name: "关闭", exact: true }).click();

  await page.getByRole("button", { name: "用户场景", exact: true }).click();
  await expect.poll(() => requested).toContain("辅酶:全部:scenario::");
  await expect(page.getByRole("button", { name: "科普 精力疲惫 爆文率50%" })).toBeVisible();
  await expect(pageSelect).toHaveValue("1");
  await expect(sortedNotes.locator("tbody tr")).toHaveCount(20);

  await page.getByRole("button", { name: "曼杰", exact: true }).click();
  await expect.poll(() => requested).toContain("辅酶:曼杰:scenario::");

  await page.getByLabel("包含未标注").check();
  await expect(page.getByText("未标注", { exact: true }).first()).toBeVisible();

  await page.getByRole("button", { name: "磷虾油", exact: true }).click();
  await expect.poll(() => requested).toContain("磷虾油:曼杰:scenario::");
});

test("content heatmap remains usable on mobile", async ({ page }) => {
  const requested: string[] = [];
  await mockCommon(page, requested);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/paipai/content-analysis");

  await expect(page.getByRole("heading", { name: "内容分析" })).toBeVisible();
  await expect(page.getByRole("button", { name: "科普 职场人 爆文率50%" })).toBeVisible();
  await expect(page.locator(".content-heatmap-scroll")).toBeVisible();
  await expect(page.getByRole("table", { name: "按搜索累计消耗排序的笔记" })).toBeVisible();
  const noteTableScroll = page.locator(".content-note-section .content-note-table-wrap");
  expect(await noteTableScroll.evaluate((element) => element.scrollWidth > element.clientWidth)).toBe(true);
  await expect(page.locator("body")).toHaveCSS("overflow-x", "visible");
  expect(await page.evaluate(() => document.body.scrollWidth <= window.innerWidth)).toBe(true);
});
