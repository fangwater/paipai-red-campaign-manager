import { createHash } from "node:crypto";
import { expect, test } from "@playwright/test";
import { makeMaituoWorkbook } from "./maituo-workbook";

test("marks a previously saved workbook and does not upload it again", async ({ page }) => {
  const buffer = await makeMaituoWorkbook("saved");
  const hash = createHash("sha256").update(buffer).digest("hex");
  let postCount = 0;
  await page.route("**/paipai/api/imports/maituo-customer-daily", async (route) => {
    if (route.request().method() === "POST") {
      postCount += 1;
      await route.fulfill({ status: 500, contentType: "application/json", body: JSON.stringify({ success: false, error: "unexpected" }) });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: [{ run_id: 7, file_name: "2026-07-23-MaiTuo.xlsx", file_sha256: hash, report_date: "2026-07-23", fetched: 275, present_sheets: ["总览KPI", "笔记明细", "分SPU总览", "分子账户", "淘搜趋势"], missing_sheets: [], completed_at: "2026-07-24T15:00:00Z" }] })
    });
  });

  await page.goto("/paipai/maituo-daily-report");
  await page.locator('input[type="file"]').setInputFiles({
    name: "2026-07-23-MaiTuo.xlsx",
    mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    buffer
  });

  const queueRow = page.locator(".queue-row");
  await expect(queueRow).toContainText("已保存");
  await expect(page.getByRole("button", { name: "保存 0 个文件" })).toBeDisabled();
  expect(postCount).toBe(0);
});
