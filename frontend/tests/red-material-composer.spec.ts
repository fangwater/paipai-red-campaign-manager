import { expect, test } from "@playwright/test";

const transparentPNG = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", "base64");
const savedMaterialID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";

test("adds a structured red material from the dedicated entry", async ({ page }) => {
  const created: Array<{ noteID: string; noteURL: string; title: string; body: string; comments: string; imageCount: number }> = [];
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true }) }));
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] }) }));
  await page.route("**/paipai/api/analytics/maituo/manual-materials*", async (route) => {
    const request = route.request();
    if (request.method() === "POST") {
      const payload = request.postDataBuffer();
      const body = payload ? payload.toString("utf8") : "";
      created.push({
        noteID: /name="note_id"[\s\S]*?\r\n\r\n([^\r]+)/.exec(body)?.[1] ?? "",
        noteURL: /name="note_url"[\s\S]*?\r\n\r\n([^\r]+)/.exec(body)?.[1] ?? "",
        title: /name="title"[\s\S]*?\r\n\r\n([^\r]+)/.exec(body)?.[1] ?? "",
        body: /name="body"[\s\S]*?\r\n\r\n([^\r]+)/.exec(body)?.[1] ?? "",
        comments: /name="comments"[\s\S]*?\r\n\r\n([^\r]+)/.exec(body)?.[1] ?? "",
        imageCount: (body.match(/name="images"/g) ?? []).length
      });
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify({
          success: true,
          data: {
            material_id: savedMaterialID,
            note_id: "aaaaaaaaaaaaaaaaaaaaaaaa",
            note_url: "https://www.xiaohongshu.com/explore/aaaaaaaaaaaaaaaaaaaaaaaa",
            title: "辅酶Q10早起实测",
            body: "连续两周记录睡眠和精力变化。",
            comments: ["这条笔记帮到我了"],
            tagged: false,
            images: [{ asset_id: "b".repeat(64), width: 1, height: 1 }],
            image_count: 1,
            comment_count: 1
          }
        })
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          total: created.length,
          page: 1,
          page_size: 8,
          items: created.length === 0 ? [] : [{
            material_id: savedMaterialID,
            note_id: "aaaaaaaaaaaaaaaaaaaaaaaa",
            note_url: "https://www.xiaohongshu.com/explore/aaaaaaaaaaaaaaaaaaaaaaaa",
            title: "辅酶Q10早起实测",
            body: "连续两周记录睡眠和精力变化。",
            comments: ["这条笔记帮到我了"],
            tagged: false,
            images: [{ asset_id: "b".repeat(64), width: 1, height: 1 }],
            image_count: 1,
            comment_count: 1
          }]
        }
      })
    });
  });

  await page.goto("/paipai/");
  await page.getByRole("button", { name: /进入添加素材/ }).click();
  await expect(page).toHaveURL(/\/paipai\/red-materials\/new$/);
  await expect(page.getByRole("heading", { name: "添加素材", exact: true })).toBeVisible();
  await expect(page.locator(".nav-item.active")).toContainText("添加素材");
  await expect(page.getByText("还没有手动添加的素材")).toBeVisible();

  await page.getByRole("textbox", { name: "笔记 ID" }).fill("AAAAAAAAAAAAAAAAAAAAAAAA");
  await expect(page.getByRole("textbox", { name: "笔记链接" })).toHaveValue(
    "https://www.xiaohongshu.com/explore/aaaaaaaaaaaaaaaaaaaaaaaa"
  );
  await page.getByRole("textbox", { name: "素材标题" }).fill("辅酶Q10早起实测");
  await page.getByRole("textbox", { name: "素材正文" }).fill("连续两周记录睡眠和精力变化。");
  await page.getByRole("textbox", { name: "素材评论 1" }).fill("这条笔记帮到我了");
  await page.locator('input[type="file"]').setInputFiles({
    name: "cover.png",
    mimeType: "image/png",
    buffer: transparentPNG
  });
  await expect(page.getByRole("img", { name: "素材图片 1" })).toBeVisible();
  await page.getByRole("button", { name: "保存素材" }).click();
  await expect.poll(() => created).toEqual([{
    noteID: "aaaaaaaaaaaaaaaaaaaaaaaa",
    noteURL: "https://www.xiaohongshu.com/explore/aaaaaaaaaaaaaaaaaaaaaaaa",
    title: "辅酶Q10早起实测",
    body: "连续两周记录睡眠和精力变化。",
    comments: '["这条笔记帮到我了"]',
    imageCount: 1
  }]);
  await expect(page.getByText("素材已保存，可继续添加或前往检索查看")).toBeVisible();
  await expect(page.getByRole("button", { name: "编辑素材 辅酶Q10早起实测" })).toBeVisible();
  await expect(page.getByText("aaaaaaaaaaaaaaaaaaaaaaaa · 1 张图 · 1 条评论 · 待标注")).toBeVisible();
});
