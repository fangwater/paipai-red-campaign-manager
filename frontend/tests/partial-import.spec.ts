import { expect, test } from "@playwright/test";
import { makeMaituoWorkbook } from "./maituo-workbook";

test("accepts the current note-only Maituo workbook", async ({ page }) => {
  await page.route("**/paipai/api/imports/maituo-customer-daily", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: [] })
    });
  });

  const buffer = await makeMaituoWorkbook("partial", ["笔记明细"]);
  await page.goto("/paipai/maituo-daily-report");
  await page.locator('input[type="file"]').setInputFiles({
    name: "2026-07-13-MaiTuo.xlsx",
    mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    buffer
  });

  const row = page.locator(".queue-row");
  await expect(row).toContainText("已识别笔记明细");
  await expect(row).not.toContainText("缺少：");
  await expect(page.getByRole("button", { name: "保存 1 个文件" })).toBeEnabled();
});
