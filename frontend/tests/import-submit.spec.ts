import { expect, test } from "@playwright/test";
import { makeMaituoWorkbook } from "./maituo-workbook";

test("sorts multiple workbooks and saves them from oldest to newest", async ({ page }) => {
  const submitted: string[] = [];
  const saved: Array<Record<string, unknown>> = [];
  await page.route("**/paipai/api/imports/maituo-customer-daily", async (route) => {
    if (route.request().method() === "GET") {
      await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: saved }) });
      return;
    }
    const body = route.request().postDataBuffer()?.toString("utf8") ?? "";
    const fileName = body.match(/filename="([^"]+)"/)?.[1] ?? "unknown.xlsx";
    const reportDate = fileName.match(/\d{4}-\d{2}-\d{2}/)?.[0] ?? "";
    submitted.push(fileName);
    const result = {
      run_id: submitted.length,
      file_name: fileName,
      file_sha256: `hash-${reportDate}`,
      report_date: reportDate,
      already_saved: false,
      present_sheets: ["总览KPI", "笔记明细", "分SPU总览", "分子账户", "淘搜趋势"],
      missing_sheets: [],
      table_count: 5,
      fetched: 5,
      inserted: 5,
      updated: 0,
      unchanged: 0,
      deleted: 0,
      tables: []
    };
    saved.unshift({ run_id: result.run_id, file_name: fileName, file_sha256: result.file_sha256, report_date: reportDate, fetched: 5, present_sheets: result.present_sheets, missing_sheets: [], completed_at: "2026-07-25T10:00:00Z" });
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: result }) });
  });

  const newer = await makeMaituoWorkbook("newer");
  const older = await makeMaituoWorkbook("older");
  await page.goto("/paipai/maituo-daily-report");
  await page.locator('input[type="file"]').setInputFiles([
    { name: "2026-07-23-MaiTuo.xlsx", mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer: newer },
    { name: "2026-07-21-MaiTuo.xlsx", mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buffer: older }
  ]);

  const rows = page.locator(".queue-row");
  await expect(rows).toHaveCount(2);
  await expect(rows.nth(0)).toContainText("2026-07-21");
  await expect(rows.nth(1)).toContainText("2026-07-23");
  await page.getByRole("button", { name: "保存 2 个文件" }).click();
  await expect.poll(() => submitted).toEqual(["2026-07-21-MaiTuo.xlsx", "2026-07-23-MaiTuo.xlsx"]);
  await expect(rows.nth(0)).toContainText("保存完成");
  await expect(rows.nth(1)).toContainText("保存完成");
});
