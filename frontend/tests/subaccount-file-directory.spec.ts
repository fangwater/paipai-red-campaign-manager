import { expect, test } from "@playwright/test";

const accountID = "6LSm5oi3QQ";

test("does not expose subaccount directories from the daily report page", async ({ page }) => {
  let directoryRequests = 0;
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({
    status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] })
  }));
  await page.route("**/paipai/api/imports/maituo-subaccount-directories", async (route) => {
    directoryRequests += 1;
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] }) });
  });

  await page.goto("/paipai/maituo-daily-report");
  await expect(page.getByRole("heading", { name: "子账户文件目录" })).toHaveCount(0);
  expect(directoryRequests).toBe(0);
});

test("lists and downloads each historical date without exposing another account", async ({ page }) => {
  const requestedFiles: string[] = [];
  await page.route(`**/paipai/api/downloads/maituo-subaccount/${accountID}**`, async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.endsWith(".xlsx")) {
      requestedFiles.push(url.pathname);
      await route.fulfill({
        status: 200,
        contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        headers: { "Content-Disposition": "attachment; filename=2026-08-05-Maituo-account-a.xlsx" },
        body: Buffer.from("workbook")
      });
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ success: true, data: { subaccount: "账户A", reports: [
        { report_date: "2026-08-05", file_name: "2026-08-05-Maituo-客户日报.xlsx" },
        { report_date: "2026-08-04", file_name: "2026-08-04-Maituo-客户日报.xlsx" }
      ] } })
    });
  });

  await page.goto(`/paipai/subaccount-files/${accountID}`);
  await expect(page.getByRole("heading", { name: "账户A" })).toBeVisible();
  await expect(page.locator(".public-file-table tbody tr")).toHaveCount(2);
  await expect(page.locator(".public-file-table tbody tr").first()).toContainText("2026-08-05");
  await expect(page.getByText("账户B")).toHaveCount(0);

  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("link", { name: "下载 2026-08-05 报表" }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toBe("2026-08-05-Maituo-account-a.xlsx");
  expect(requestedFiles).toEqual([`/paipai/api/downloads/maituo-subaccount/${accountID}/2026-08-05.xlsx`]);

  await page.setViewportSize({ width: 360, height: 800 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)).toBe(false);
  const mobileDownload = page.getByRole("link", { name: "下载 2026-08-05 报表" });
  await expect(mobileDownload).toBeVisible();
  const box = await mobileDownload.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.x + box!.width).toBeLessThanOrEqual(360);
});
