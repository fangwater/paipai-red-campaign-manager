import { expect, test } from "@playwright/test";

const accountID = "6LSm5oi3QQ";

test("shows subaccount report directories on the Maituo daily page", async ({ page }) => {
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({
    status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] })
  }));
  await page.route("**/paipai/api/imports/maituo-subaccount-directories", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ success: true, data: [{
      subaccount: "账户A", account_id: accountID, report_count: 8,
      earliest_report_date: "2026-08-10", latest_report_date: "2026-08-23"
    }] })
  }));

  await page.goto("/paipai/maituo-daily-report");
  const section = page.locator(".subaccount-directories");
  await expect(section.getByRole("heading", { name: "子账户文件目录" })).toBeVisible();
  await expect(section).toContainText("账户A");
  await expect(section).toContainText("8");
  await expect(section.getByRole("textbox", { name: "账户A 文件目录 URL" })).toHaveValue(/\/paipai\/subaccount-files\/6LSm5oi3QQ$/);
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
