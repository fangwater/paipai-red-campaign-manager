import { expect, test } from "@playwright/test";

const emptyMetrics = {
  total_pay_amount: null,
  part_pay_amount: null,
  ad_cost: null,
  click_count: null,
  interaction_count: null,
  total_roi: null
};

function makeResult(type: "note" | "plan") {
  const isNote = type === "note";
  return {
    entity_type: type,
    spu: "辅酶",
    sort: "roi",
    snapshot: {
      fetch_id: isNote ? 40 : 41,
      entity_type: type,
      snapshot_date: "2026-08-01",
      window_start: isNote ? "2026-07-19" : "2026-07-26",
      window_end: "2026-08-01",
      window_days: isNote ? 14 : 7,
      source_cutoff_date: "2026-08-01",
      brand_name: "MegaRed",
      attribution_type: "",
      attribution_model: "",
      attribution_window_days: 0,
      row_count: isNote ? 1906 : 82,
      finished_at: "2026-08-02T01:00:00Z"
    },
    summary: {
      item_count: isNote ? 1906 : 82,
      account_count: isNote ? 749 : 12,
      linked_count: isNote ? 988 : 66,
      new_count: isNote ? 12 : 4,
      metric_item_count: 0,
      metrics: emptyMetrics
    },
    total: isNote ? 1906 : 82,
    page: 1,
    page_size: 25,
    items: isNote ? [{
      id: "note-001",
      url: "https://www.xiaohongshu.com/explore/note-001",
      name: "出差也能坚持的辅酶补充记录",
      author_name: "会飞的桃子",
      account_name: "",
      publish_time: "2026-07-31 10:30:00",
      picture_url: "https://example.com/cover.jpg",
      spu_id: "spu-1",
      spu_name: "MegaRed 辅酶",
      tag: "",
      plan_type: "",
      note_type: 1,
      linked_note_count: 0,
      is_new: true,
      metrics: emptyMetrics
    }] : [{
      id: "plan-001",
      url: "",
      name: "磷虾油搜索计划",
      author_name: "",
      account_name: "MegaRed旗舰店",
      publish_time: "2026-07-30 09:00:00",
      picture_url: "",
      spu_id: "",
      spu_name: "",
      tag: "搜索",
      plan_type: "搜索推广",
      note_type: 0,
      linked_note_count: 8,
      is_new: false,
      metrics: emptyMetrics
    }]
  };
}

test("shows the latest Guorai snapshot and switches entity type", async ({ page }) => {
  const requestedTypes: string[] = [];
  const requestedSearches: string[] = [];
  const requestedSorts: string[] = [];
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, body: "ok" }));
  const requestedFilters: string[] = [];
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] }) }));
  await page.route("**/cover.jpg", (route) => route.fulfill({ status: 200, contentType: "image/png", body: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", "base64") }));
  await page.route("**/paipai/api/analytics/guorai/latest?*", async (route) => {
    const url = new URL(route.request().url());
    const type = (url.searchParams.get("type") ?? "note") as "note" | "plan";
    requestedTypes.push(type);
    requestedSearches.push(url.searchParams.get("q") ?? "");
    requestedSorts.push(url.searchParams.get("sort") ?? "");
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: makeResult(type) }) });
    requestedFilters.push(type + ":" + (url.searchParams.get("spu") ?? ""));
  });

  await page.goto("/paipai/guorai-data");
  await expect(page.getByRole("heading", { name: "薯量数据" })).toBeVisible();
  await expect(page.getByText("1,906", { exact: true })).toBeVisible();
  await expect(page.getByText("出差也能坚持的辅酶补充记录")).toBeVisible();
  const noteLink = page.getByRole("link", { name: "打开笔记：出差也能坚持的辅酶补充记录" });
  await expect(noteLink).toHaveAttribute("href", "https://www.xiaohongshu.com/explore/note-001");
  await expect(noteLink).toHaveAttribute("target", "_blank");
  await expect(page.getByText("当前快照未返回投放指标")).toBeVisible();
  await expect.poll(() => requestedSorts).toContain("roi");
  await expect(page.getByText("曝光量", { exact: true })).toHaveCount(0);
  await expect.poll(() => requestedFilters).toContain("note:辅酶");
  await expect(page.getByRole("combobox", { name: "SPU" })).toHaveValue("辅酶");
  await page.getByRole("combobox", { name: "SPU" }).selectOption("磷虾油");
  await expect.poll(() => requestedFilters).toContain("note:磷虾油");
  await expect(page.getByRole("heading", { name: "磷虾油笔记明细" })).toBeVisible();
  await expect(page.getByRole("columnheader", { name: "曝光", exact: true })).toHaveCount(0);
  await expect(page.locator(".guorai-cover img")).toHaveCount(1);

  await page.getByPlaceholder("搜索笔记、达人或 SPU").fill("辅酶");
  await expect.poll(() => requestedSearches).toContain("辅酶");

  await page.getByRole("button", { name: "计划", exact: true }).click();
  await expect.poll(() => requestedTypes).toContain("plan");
  await expect.poll(() => requestedFilters).toContain("plan:磷虾油");
  await expect(page.getByRole("heading", { name: "磷虾油计划明细" })).toBeVisible();
  await expect(page.getByText("磷虾油搜索计划")).toBeVisible();
  await expect(page.getByText("MegaRed旗舰店")).toBeVisible();
  await expect(page.getByText("8", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "查看磷虾油搜索计划的8条关联笔记分析" }).click();
  await expect(page).toHaveURL(/\/paipai\/note-campaign-analysis\?.*plan_id=plan-001/);
  await expect(page).toHaveURL(/window=all/);
});
