import { expect, test } from "@playwright/test";

const materialID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
const noteID = "bbbbbbbbbbbbbbbbbbbbbbbb";
const pendingItem = {
  material_id: materialID,
  note_id: noteID,
  note_url: `https://www.xiaohongshu.com/explore/${noteID}`,
  title: "待标注标题",
  body: "待标注正文",
  comments: ["评论"],
  tags: {
    note_type: "",
    cover_type: "",
    commercial_intensity: "",
    audience: "",
    user_scenario: ""
  },
  tagged: false,
  images: [{ asset_id: "c".repeat(64), width: 1, height: 1 }],
  image_count: 1,
  comment_count: 1
};

test("pending materials page can annotate tags", async ({ page }) => {
  let remaining = [pendingItem];
  const savedTags: Array<Record<string, string>> = [];
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true }) }));
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] }) }));
  await page.route("**/paipai/api/manuscript-assets/*", (route) => route.fulfill({
    status: 200,
    contentType: "image/png",
    body: Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=", "base64")
  }));
  await page.route("**/paipai/api/analytics/maituo/manual-materials*", async (route) => {
    const url = new URL(route.request().url());
    expect(url.searchParams.get("untagged")).toBe("true");
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          search: url.searchParams.get("q") || "",
          untagged: true,
          total: remaining.length,
          page: 1,
          page_size: 10,
          tag_options: {
            note_type: ["科普"],
            cover_type: ["大字报"],
            commercial_intensity: ["软广"],
            audience: ["职场人"],
            user_scenario: ["精力疲惫"]
          },
          items: remaining
        }
      })
    });
  });
  await page.route("**/paipai/api/analytics/maituo/manual-material-tags*", async (route) => {
    expect(route.request().method()).toBe("PUT");
    const url = new URL(route.request().url());
    expect(url.searchParams.get("material_id")).toBe(materialID);
    const payload = route.request().postDataJSON() as Record<string, string>;
    savedTags.push(payload);
    remaining = [];
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          ...pendingItem,
          tags: payload,
          tagged: true
        }
      })
    });
  });

  await page.goto("/paipai/");
  await page.getByRole("button", { name: /进入待标注素材/ }).click();
  await expect(page).toHaveURL(/\/paipai\/red-materials\/pending$/);
  await expect(page.getByRole("heading", { name: "待标注素材", exact: true })).toBeVisible();
  await expect(page.locator(".nav-item.active")).toContainText("待标注素材");
  await expect(page.getByRole("heading", { name: "待标注标题" })).toBeVisible();
  await expect(page.getByText(noteID)).toBeVisible();
  await page.getByRole("button", { name: "标注", exact: true }).click();
  await expect(page.getByRole("dialog", { name: "标注素材" })).toBeVisible();
  await page.getByRole("combobox", { name: "内容类型" }).fill("科普");
  await page.getByRole("combobox", { name: "封面类型" }).fill("大字报");
  await page.getByRole("combobox", { name: "商业强度" }).fill("软广");
  await page.getByRole("combobox", { name: "对话人群" }).fill("职场人");
  await page.getByRole("combobox", { name: "用户场景" }).fill("精力疲惫");
  await page.getByRole("button", { name: "保存标签" }).click();
  await expect.poll(() => savedTags).toEqual([{
    note_type: "科普",
    cover_type: "大字报",
    commercial_intensity: "软广",
    audience: "职场人",
    user_scenario: "精力疲惫"
  }]);
  await expect(page.getByRole("dialog", { name: "标注素材" })).toHaveCount(0);
  await expect(page.getByText("没有待标注的素材")).toBeVisible();
});
