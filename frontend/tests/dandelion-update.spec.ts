import { expect, test } from "@playwright/test";

test("updates Dandelion data without the field-alignment UI", async ({ page }) => {
  await page.route("**/paipai/api/imports/dandelion-excel", async (route) => {
    if (route.request().method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ success: true, data: [] })
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: {
          run_id: 22,
          file_name: "蒲公英数据.xlsx",
          sheet_name: "笔记批量数据",
          header_row: 3,
          report_date: "2026-08-06",
          fetched: 2011,
          inserted: 0,
          updated: 0,
          unchanged: 2011,
          deleted: 0,
          completed_at: "2026-08-06T18:30:00+08:00"
        }
      })
    });
  });
  await page.goto("/paipai/dandelion-upload");
  await expect(page.getByRole("heading", { name: "蒲公英数据更新" })).toBeVisible();
  await page.locator('input[type="file"]').setInputFiles({
    name: "蒲公英数据.xlsx",
    mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    buffer: Buffer.from("test workbook")
  });
  await expect(page.getByText("蒲公英数据.xlsx", { exact: true })).toBeVisible();
  await page.getByRole("button", { name: "更新数据" }).click();
  await expect(page.getByRole("heading", { name: "更新完成" })).toBeVisible();
  await expect(page.locator(".dandelion-update-result")).toContainText("2,011");
  await expect(page.getByText("字段对齐结果")).toHaveCount(0);
  await expect(page.getByText("Excel 额外字段")).toHaveCount(0);
  await expect(page.getByText("数据预览")).toHaveCount(0);
});

test("shows an archived workbook error returned by the backend", async ({ page }) => {
  await page.route("**/paipai/api/imports/dandelion-excel", async (route) => {
    if (route.request().method() === "GET") {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ success: true, data: [] })
      });
      return;
    }
    await route.fulfill({
      status: 400,
      contentType: "application/json",
      body: JSON.stringify({
        success: false,
        error: "工作表缺少核心字段；文件已保存，编号：upload-test"
      })
    });
  });
  await page.goto("/paipai/dandelion-upload");
  await page.locator('input[type="file"]').setInputFiles({
    name: "蒲公英数据.xlsx",
    mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
    buffer: Buffer.from("test workbook")
  });
  await page.getByRole("button", { name: "更新数据" }).click();
  await expect(page.getByText(/文件已保存，编号：upload-test/)).toBeVisible();
});

test("shows dated upload history and defaults non-working days", async ({ page }) => {
  const savedUpload = (date: string, runID: number) => ({
    run_id: runID,
    file_name: `蒲公英-${date}.xlsx`,
    file_sha256: `hash-${date}`,
    report_date: date,
    sheet_name: "笔记批量数据",
    fetched: 250,
    inserted: 10,
    updated: 15,
    unchanged: 225,
    completed_at: "2026-07-05T18:30:00+08:00"
  });
  await page.route("**/paipai/api/imports/dandelion-excel", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        success: true,
        data: [savedUpload("2026-07-05", 2), savedUpload("2026-07-02", 1)]
      })
    });
  });

  await page.goto("/paipai/dandelion-upload");
  await expect(page.getByRole("heading", { name: "历史上传" })).toBeVisible();
  const rows = page.locator(".dandelion-history-table tbody tr");
  await expect(rows).toHaveCount(4);
  await expect(rows.nth(0)).toContainText("2026-07-05");
  await expect(rows.nth(0)).toContainText("蒲公英-2026-07-05.xlsx");
  await expect(rows.nth(1)).toContainText("2026-07-04");
  await expect(rows.nth(1)).toContainText("周六");
  await expect(rows.nth(1)).toContainText("无需上传");
  await expect(rows.nth(2)).toContainText("2026-07-03");
  await expect(rows.nth(2)).toContainText("周五");
  await expect(rows.nth(2)).toContainText("无需上传");
  await expect(page.getByText("缺少文件")).toHaveCount(0);
});
