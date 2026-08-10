import { expect, test } from "@playwright/test";

const points = [
  { report_date: "2026-07-21", spend: 10, search_users: 2, search_cost: 5, cumulative_spend: 10, cumulative_search_users: 2 },
  { report_date: "2026-07-22", spend: 0, search_users: 0, search_cost: 0, cumulative_spend: 10, cumulative_search_users: 2 },
  { report_date: "2026-07-23", spend: 20, search_users: 4, search_cost: 5, cumulative_spend: 30, cumulative_search_users: 6 }
];

const manuscriptAssetID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
const transparentPNG = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", "base64");

test("renders cumulative ECharts and switches the note campaign key", async ({ page }) => {
  const requestedWindows: string[] = [];
  const requestedSorts: string[] = [];
  const requestedPlanIDs: string[] = [];
  await page.route("**/paipai/api/analytics/maituo/note-campaigns?*", async (route) => {
    const url = new URL(route.request().url());
    requestedWindows.push(url.searchParams.get("window") ?? "");
    requestedSorts.push(url.searchParams.get("sort") || "");
    requestedPlanIDs.push(url.searchParams.get("plan_id") || "");
    const windowOption = url.searchParams.get("window") ?? "7d";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          window: windowOption,
          sort: url.searchParams.get("sort") || "cumulative_spend",
          report_dates: points.map((point) => point.report_date),
          total: 2,
          page: 1,
          page_size: 25,
          items: [
            { note_id: "note-a", campaign_name: "磷虾油搜索计划", placement: "搜索", first_report_date: "2026-07-21", last_report_date: "2026-07-23", active_days: 2, latest_spend: 20, total_spend: 30, total_search_users: 6, latest_search_cost: 5, points },
            { note_id: "note-b", campaign_name: "辅酶信息流计划", placement: "信息流", first_report_date: "2026-07-21", last_report_date: "2026-07-23", active_days: 3, latest_spend: 8, total_spend: 18, total_search_users: 3, latest_search_cost: 4, points: points.map((point) => ({ ...point, cumulative_spend: point.cumulative_spend * 0.6, cumulative_search_users: Math.round(point.cumulative_search_users * 0.5) })) }
          ]
        }
      })
    });
  });
  await page.route("**/paipai/api/manuscript-assets/*", (route) => {
    void route.fulfill({ status: 200, contentType: "image/png", body: transparentPNG });
  });
  await page.route("**/paipai/api/analytics/maituo/note-content?*", async (route) => {
    const url = new URL(route.request().url());
    const noteID = url.searchParams.get("note_id") || "";
    const tagsComplete = noteID === "note-a";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          note_id: noteID,
          note_url: `https://www.xiaohongshu.com/explore/${noteID}`,
          found: true,
          note_content: "这是从稿件库查询到的真实笔记内容。",
          blocks: [
            { type: "paragraph", text: "这是从稿件库查询到的真实笔记内容。" },
            { type: "image", asset_id: manuscriptAssetID, width: 1, height: 1, caption: "定稿配图" },
            { type: "paragraph", text: "图片后的正文。" }
          ],
          providers: ["智元"],
          tags: {
            note_type: ["科普", "经验分享"],
            cover_type: ["大字报"],
            commercial_intensity: ["软广"],
            audience: ["职场人"],
            user_scenario: tagsComplete ? ["精力疲惫"] : [],
            progress: ["已发布"],
            complete: tagsComplete,
            missing_fields: tagsComplete ? [] : ["user_scenario"]
          }
        }
      })
    });
  });

  await page.goto("/paipai/note-campaign-analysis?plan_id=211253819&plan_name=%E5%85%B3%E8%81%94%E8%AE%A1%E5%88%92&window=all");
  await expect(page.getByRole("heading", { name: "笔记计划分析" })).toBeVisible();
  await expect.poll(() => requestedPlanIDs).toContain("211253819");
  await expect(page.getByText("关联计划", { exact: true })).toBeVisible();
  await expect(page.locator(".metric-chart")).toHaveCount(3);
  await expect(page.getByText("累计回搜成本", { exact: true })).toHaveCount(0);
  await expect(page.getByText("回搜成本", { exact: true })).toBeVisible();
  await expect(page.locator(".focus-identity")).toContainText("磷虾油搜索计划");
  await expect(page.locator(".analysis-table tbody tr")).toHaveCount(2);
  await expect(page.getByText("http", { exact: false })).toHaveCount(0);

  await page.getByRole("button", { name: "查询内容" }).click();
  const contentDialog = page.getByRole("dialog", { name: "笔记内容" });
  await expect(contentDialog).toContainText("这是从稿件库查询到的真实笔记内容。");
  await expect(contentDialog).toContainText("智元");
  const tags = contentDialog.getByRole("region", { name: "稿件标签" });
  await expect(tags).toContainText("标签完整");
  await expect(tags).toContainText("科普");
  await expect(tags).toContainText("精力疲惫");
  await expect(contentDialog.getByRole("img", { name: "定稿配图" })).toBeVisible();
  await expect(contentDialog.locator(".manuscript-blocks")).toContainText("图片后的正文。");
  await expect(contentDialog.locator(".manuscript-blocks > :nth-child(2) img")).toHaveAttribute(
    "src",
    `/paipai/api/manuscript-assets/${manuscriptAssetID}`
  );
  await contentDialog.getByRole("button", { name: "查看大图：定稿配图" }).click();
  const lightbox = page.getByRole("dialog", { name: "稿件大图" });
  await expect(lightbox.getByRole("img", { name: "定稿配图" })).toBeVisible();
  await lightbox.getByRole("button", { name: "关闭稿件大图" }).click();
  await expect(contentDialog.getByRole("link", { name: "打开小红书笔记" })).toHaveAttribute("href", "https://www.xiaohongshu.com/explore/note-a");
  await contentDialog.getByRole("button", { name: "关闭笔记内容" }).click();
  await expect(contentDialog).toHaveCount(0);

  await expect.poll(async () => page.locator(".metric-chart canvas").evaluateAll((canvases) => canvases.map((canvas) => {
    const element = canvas as HTMLCanvasElement;
    const context = element.getContext("2d");
    if (!context || element.width === 0 || element.height === 0) return false;
    const pixels = context.getImageData(0, 0, element.width, element.height).data;
    for (let index = 3; index < pixels.length; index += 4) if (pixels[index] > 0) return true;
    return false;
  }))).toEqual([true, true, true]);

  await page.locator(".analysis-table tbody tr").nth(1).click();
  await expect(page.locator(".focus-identity")).toContainText("辅酶信息流计划");
  await page.getByRole("button", { name: "查询内容" }).click();
  const incompleteDialog = page.getByRole("dialog", { name: "笔记内容" });
  const incompleteTags = incompleteDialog.getByRole("region", { name: "稿件标签" });
  await expect(incompleteTags).toContainText("标签待补充");
  await expect(incompleteTags.locator("dl > div.missing")).toContainText("用户场景");
  await expect(incompleteTags.locator("dl > div.missing")).toContainText("待补充");
  await incompleteDialog.getByRole("button", { name: "关闭笔记内容" }).click();
  await page.getByRole("button", { name: "3D" }).click();
  await expect.poll(() => requestedWindows).toContain("3d");
  await page.getByRole("button", { name: "当天消耗" }).click();
  await expect.poll(() => requestedSorts).toContain("daily_spend");
});
