import { expect, test, type Page } from "@playwright/test";

function makeOverview(days: number, spu: "辅酶" | "磷虾油") {
  const dates = Array.from({ length: days }, (_, index) => {
    const date = new Date(Date.UTC(2026, 6, 1 + index));
    return date.toISOString().slice(0, 10);
  });
  const metric = (key: string, label: string, unit: string, start: number, change: number) => ({
    key,
    label,
    unit,
    current_value: start * days,
    previous_value: start * days * 0.8,
    change_pct: change,
    points: dates.map((date, index) => ({ date, value: start + index * 2 }))
  });
  return {
    days,
    spu,
    trend: {
      start_date: dates[0],
      end_date: dates[dates.length - 1],
      previous_start_date: "2026-07-15",
      previous_end_date: "2026-07-21",
      available_days: days,
      metrics: [
        metric("spend", "每日消耗", "currency", 12000, 0.25),
        metric("search_cost", "回搜成本", "currency", 24, -0.1),
        metric("search_uv", "淘搜 UV", "count", 500, 0.18),
        metric("order_uv", "成交 UV", "count", 120, 0.12)
      ]
    },
    new_notes: {
      start_date: dates[0],
      end_date: dates[dates.length - 1],
      total: 3,
      source_synced_at: "2026-07-30T09:00:00Z",
      daily: dates.map((date, index) => ({ date, count: index < 3 ? index + 1 : 0 })),
      agencies: [
        {
          agency: "智元",
          count: 2,
          audience_tags: ["职场人", "健身人"],
          notes: [
            { note_id: "note-1", title: "通勤党的辅酶体验", url: "https://example.com/note-1", author: "一颗栗子", published_date: dates[1], agency: "智元", audience: "职场人", note_type: "图文", content_tag: "通勤" },
            { note_id: "note-2", title: "运动恢复记录", url: "", author: "小K", published_date: dates[0], agency: "智元", audience: "健身人", note_type: "视频", content_tag: "健身" }
          ]
        },
        {
          agency: "曼杰",
          count: 1,
          audience_tags: ["中老年"],
          notes: [{ note_id: "note-3", title: "给爸妈选辅酶", url: "https://example.com/note-3", author: "阿橙", published_date: dates[2], agency: "曼杰", audience: "中老年", note_type: "图文", content_tag: "家庭" }]
        },
        { agency: "引响", count: 0, audience_tags: [], notes: [] },
        { agency: "飓风", count: 0, audience_tags: [spu + "选购"], notes: [] },
        { agency: "有一有二", count: 0, audience_tags: [], notes: [] }
      ]
    }
  };
}

async function mockCommon(page: Page, requestedQueries: string[]) {
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, body: "ok" }));
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] }) }));
  await page.route("**/paipai/api/analytics/overview?*", async (route) => {
    const params = new URL(route.request().url()).searchParams;
    const days = params.get("days") ?? "7";
    const spu = (params.get("spu") ?? "辅酶") as "辅酶" | "磷虾油";
    requestedQueries.push(spu + ":" + days);
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: makeOverview(Number(days), spu) }) });
  });
}

test("renders overview trends and agency note details", async ({ page }) => {
  const requestedQueries: string[] = [];
  await mockCommon(page, requestedQueries);

  await page.goto("/paipai/overview");
  await expect(page.getByRole("heading", { name: "数据总览" })).toBeVisible();
  await expect(page.locator(".overview-metric-card")).toHaveCount(4);
  await expect.poll(() => requestedQueries).toContain("辅酶:7");
  await expect(page.getByText("较前周期 +25.0%")).toBeVisible();
  await expect(page.getByText("辅酶选购", { exact: true })).toBeVisible();
  await expect(page.locator(".agency-table tbody tr")).toHaveCount(2);

  await expect.poll(async () => page.locator(".overview-metric-card canvas, .new-notes-chart canvas").evaluateAll((canvases) => canvases.map((canvas) => {
    const element = canvas as HTMLCanvasElement;
    const context = element.getContext("2d");
    if (!context || element.width === 0 || element.height === 0) return false;
    const pixels = context.getImageData(0, 0, element.width, element.height).data;
    for (let index = 3; index < pixels.length; index += 4) if (pixels[index] > 0) return true;
    return false;
  }))).toEqual([true, true, true, true, true]);

  await page.getByRole("button", { name: /曼杰/ }).click();
  await expect(page.getByRole("heading", { name: "曼杰 · 笔记详情" })).toBeVisible();
  await expect(page.getByText("给爸妈选辅酶")).toBeVisible();
  await expect(page.getByRole("link", { name: "打开笔记 note-3" })).toHaveAttribute("href", "https://example.com/note-3");

  await page.getByRole("button", { name: "磷虾油", exact: true }).click();
  await expect.poll(() => requestedQueries).toContain("磷虾油:7");
  await expect(page.getByText("磷虾油选购", { exact: true })).toBeVisible();

  await page.getByRole("button", { name: "14日" }).click();
  await expect.poll(() => requestedQueries).toContain("磷虾油:14");
});

test("overview remains usable on mobile", async ({ page }) => {
  const requestedQueries: string[] = [];
  await mockCommon(page, requestedQueries);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/paipai/overview");

  await expect(page.getByRole("heading", { name: "数据总览" })).toBeVisible();
  await expect(page.locator(".overview-metric-card").first()).toBeVisible();
  await expect(page.getByRole("button", { name: /智元/ })).toBeVisible();
  await expect(page.locator("body")).toHaveCSS("overflow-x", "visible");
});
