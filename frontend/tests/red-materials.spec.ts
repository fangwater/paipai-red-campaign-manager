import { expect, test } from "@playwright/test";

const materialID = "6208dd8e000000002103e259";
const filledMaterialID = "6208dd8e000000002103e260";
const sourceNoteIDs = ["6a5975fd000000001003f991", "6a33d5aa000000001c024f8a"];
const sourceManuscripts = [
  { note_id: sourceNoteIDs[0], title: "通勤精力管理实测", url: "https://example.feishu.cn/wiki/manuscript-1" },
  { note_id: sourceNoteIDs[1], title: "熬夜恢复方法记录", url: "https://example.feishu.cn/wiki/manuscript-2" }
];
const manuscriptAssetID = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
const transparentPNG = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", "base64");

test("opens the first-level red materials directory from the homepage", async ({ page }) => {
  const searches: string[] = [];
  const pages: string[] = [];
  const requestedManuscriptIDs: string[] = [];
  const savedReferenceContents: string[] = [];
  const requestedFilters: Array<{ provider: string; noteType: string; audience: string }> = [];
  const referenceContents = new Map<string, string>([[filledMaterialID, "这是已经填充的参考素材正文。"]]);
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true }) }));
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] }) }));
  await page.route("**/paipai/api/manuscript-assets/*", (route) => {
    void route.fulfill({ status: 200, contentType: "image/png", body: transparentPNG });
  });
  await page.route("**/paipai/api/analytics/maituo/note-content?*", async (route) => {
    const url = new URL(route.request().url());
    const noteID = url.searchParams.get("note_id") || "";
    requestedManuscriptIDs.push(noteID);
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          note_id: noteID,
          note_url: `https://www.xiaohongshu.com/explore/${noteID}`,
          found: true,
          note_content: "这是中台保存的稿件正文。",
          blocks: [
            { type: "paragraph", text: "这是中台保存的稿件正文。" },
            { type: "image", asset_id: manuscriptAssetID, width: 1, height: 1, caption: "存档配图" }
          ],
          providers: ["智元"],
          tags: {
            note_type: ["科普"],
            cover_type: ["大字报"],
            commercial_intensity: ["软广"],
            audience: ["职场人"],
            user_scenario: ["精力疲惫"],
            progress: ["已发布"],
            complete: true,
            missing_fields: []
          }
        }
      })
    });
  });
  await page.route("**/paipai/api/analytics/maituo/reference-material-content*", async (route) => {
    const request = route.request();
    if (request.method() === "PUT") {
      const payload = request.postDataJSON() as { reference_note_id: string; note_content: string };
      referenceContents.set(payload.reference_note_id, payload.note_content);
      savedReferenceContents.push(payload.note_content);
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          success: true,
          data: { note_id: payload.reference_note_id, found: true, note_content: payload.note_content, source: "manual" }
        })
      });
      return;
    }
    const url = new URL(request.url());
    const noteID = url.searchParams.get("note_id") || "";
    const noteContent = referenceContents.get(noteID) || "";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          note_id: noteID,
          found: noteContent !== "",
          note_content: noteContent,
          source: noteID === filledMaterialID ? "manuscript" : noteContent ? "manual" : ""
        }
      })
    });
  });
  await page.route("**/paipai/api/analytics/maituo/reference-materials?*", async (route) => {
    const url = new URL(route.request().url());
    const requestedPage = url.searchParams.get("page") || "1";
    searches.push(url.searchParams.get("q") || "");
    pages.push(requestedPage);
    requestedFilters.push({
      provider: url.searchParams.get("provider") || "",
      noteType: url.searchParams.get("note_type") || "",
      audience: url.searchParams.get("audience") || ""
    });
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          search: url.searchParams.get("q") || "",
          filters: {
            provider: url.searchParams.get("provider") || "",
            note_type: url.searchParams.get("note_type") || "",
            cover_type: url.searchParams.get("cover_type") || "",
            commercial_intensity: url.searchParams.get("commercial_intensity") || "",
            audience: url.searchParams.get("audience") || "",
            user_scenario: url.searchParams.get("user_scenario") || ""
          },
          filter_options: {
            providers: ["智元", "曼杰"],
            note_type: ["科普", "经验分享"],
            cover_type: ["大字报", "产品图"],
            commercial_intensity: ["软广", "硬广"],
            audience: ["职场人", "中老年"],
            user_scenario: ["精力疲惫", "父母心脏养护"]
          },
          stats: { material_count: 30, source_note_count: 27, reference_count: 34, provider_count: 3 },
          total: 30,
          page: Number(requestedPage),
          page_size: 25,
          items: [{
            reference_note_id: materialID,
            note_url: `https://www.xiaohongshu.com/explore/${materialID}`,
            source_note_ids: sourceNoteIDs,
            source_manuscripts: sourceManuscripts,
            providers: ["智元", "曼杰"],
            tags: {
              note_type: ["科普"],
              cover_type: ["大字报"],
              commercial_intensity: ["软广"],
              audience: ["职场人"],
              user_scenario: ["精力疲惫"]
            },
            usage_count: 2,
            has_content: false,
            content_source: ""
          }, {
            reference_note_id: filledMaterialID,
            note_url: `https://www.xiaohongshu.com/explore/${filledMaterialID}`,
            source_note_ids: [sourceNoteIDs[1]],
            source_manuscripts: [sourceManuscripts[1]],
            providers: ["智元"],
            tags: {
              note_type: ["经验分享"],
              cover_type: ["产品图"],
              commercial_intensity: ["硬广"],
              audience: ["中老年"],
              user_scenario: ["父母心脏养护"]
            },
            usage_count: 1,
            has_content: true,
            content_source: "manuscript"
          }]
        }
      })
    });
  });

  await page.goto("/paipai/");
  await expect(page.getByText("14 个可用功能", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: /进入检索素材/ }).click();
  await expect(page).toHaveURL(/\/paipai\/red-materials$/);
  await expect(page.getByRole("heading", { name: "检索素材" })).toBeVisible();
  await expect(page.locator(".nav-item.active")).toContainText("检索素材");
  await expect(page.locator(".red-material-stats")).toContainText("30");
  await expect(page.locator(".red-material-stats")).toContainText("34");
  await expect(page.getByRole("button", { name: "按机构筛选 智元" }).first()).toBeVisible();
  await expect(page.getByRole("button", { name: "按机构筛选 曼杰" })).toBeVisible();
  await page.getByRole("combobox", { name: "内容类型" }).selectOption("科普");
  await expect.poll(() => requestedFilters.some((filters) => filters.noteType === "科普")).toBe(true);
  await page.getByRole("button", { name: "按对话人群筛选 职场人" }).click();
  await expect.poll(() => requestedFilters.some((filters) =>
    filters.noteType === "科普" && filters.audience === "职场人"
  )).toBe(true);
  const resetFilters = page.getByRole("button", { name: "清除全部筛选" });
  await expect(resetFilters).toBeEnabled();
  await resetFilters.click();
  await expect(page.getByRole("combobox", { name: "内容类型" })).toHaveValue("");
  await expect(page.getByRole("combobox", { name: "对话人群" })).toHaveValue("");
  await page.getByRole("button", { name: "按机构筛选 智元" }).first().click();
  await expect.poll(() => requestedFilters.some((filters) => filters.provider === "智元")).toBe(true);
  await page.getByRole("button", { name: "取消机构筛选 智元" }).first().click();
  await expect(page.getByRole("combobox", { name: "来源机构" })).toHaveValue("");
  const materialLink = page.getByRole("link", { name: `打开红薯素材 ${materialID}` });
  await expect(materialLink).toHaveAttribute("href", `https://www.xiaohongshu.com/explore/${materialID}`);
  await expect(page.getByText("未填充", { exact: true })).toHaveCount(1);
  await expect(page.getByText("已填充", { exact: true })).toHaveCount(1);
  await expect(page.locator(".red-material-sources").first()).not.toContainText(sourceNoteIDs[0]);
  await expect(page.getByRole("button", { name: `查看已存稿件 ${sourceManuscripts[0].title}` })).toBeVisible();
  await expect(page.getByRole("link", { name: `打开飞书稿件 ${sourceManuscripts[0].title}` }))
    .toHaveAttribute("href", sourceManuscripts[0].url);

  await page.getByRole("button", { name: `查看参考内容 ${filledMaterialID}` }).click();
  const referenceDialog = page.getByRole("dialog", { name: "参考内容" });
  await expect(referenceDialog).toContainText("这是已经填充的参考素材正文。");
  await expect(referenceDialog).toContainText("稿件库");
  await expect(referenceDialog.getByRole("button", { name: "编辑参考内容" })).toBeVisible();
  await referenceDialog.getByRole("button", { name: "关闭参考内容" }).click();

  await page.getByRole("button", { name: `录入参考内容 ${materialID}` }).click();
  const editor = page.getByRole("dialog", { name: "录入参考内容" });
  await editor.getByRole("textbox", { name: "素材内容" }).fill("这是人工录入的参考素材正文。");
  await editor.getByRole("button", { name: "保存", exact: true }).click();
  await expect.poll(() => savedReferenceContents).toContain("这是人工录入的参考素材正文。");
  await expect(referenceDialog).toContainText("这是人工录入的参考素材正文。");
  await expect(referenceDialog).toContainText("人工录入");
  await referenceDialog.getByRole("button", { name: "关闭参考内容" }).click();
  await expect(page.getByText("已填充", { exact: true })).toHaveCount(2);
  const sourceButton = page.getByRole("button", { name: `查看已存稿件 ${sourceManuscripts[0].title}` });
  await expect(sourceButton).toBeVisible();
  await expect(sourceButton).not.toHaveAttribute("href", /.+/);
  await sourceButton.click();
  await expect.poll(() => requestedManuscriptIDs).toContain(sourceNoteIDs[0]);
  const manuscriptDialog = page.getByRole("dialog", { name: "对应稿件" });
  await expect(manuscriptDialog).toContainText("这是中台保存的稿件正文。");
  await expect(manuscriptDialog).toContainText(sourceManuscripts[0].title);
  await expect(manuscriptDialog).toContainText("智元");
  await expect(manuscriptDialog.getByRole("region", { name: "稿件标签" })).toContainText("精力疲惫");
  await expect(manuscriptDialog.getByRole("img", { name: "存档配图" })).toBeVisible();
  await expect(manuscriptDialog.getByRole("link")).toHaveCount(0);
  await expect(page).toHaveURL(/\/paipai\/red-materials$/);
  await manuscriptDialog.getByRole("button", { name: "关闭对应稿件" }).click();
  await expect(manuscriptDialog).toHaveCount(0);

  await page.getByPlaceholder("搜索素材 ID、稿件标题、机构或标签").fill("智元");
  await expect.poll(() => searches).toContain("智元");
  await page.getByRole("button", { name: "下一页" }).click();
  await expect.poll(() => pages).toContain("2");
});
