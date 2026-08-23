import { expect, test } from "@playwright/test";

test("shows provider report directories on Maituo daily page", async ({ page }) => {
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({
    status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] })
  }));
  await page.route("**/paipai/api/imports/maituo-provider-directories", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ success: true, data: [{
      provider_code: "manjie", provider_name: "曼杰", report_count: 8, note_count: 42,
      earliest_report_date: "2026-08-10", latest_report_date: "2026-08-23"
    }] })
  }));

  await page.goto("/paipai/maituo-daily-report");
  const section = page.locator(".provider-directories");
  await expect(section.getByRole("heading", { name: "服务商日报目录" })).toBeVisible();
  await expect(section).toContainText("曼杰");
  await expect(section).toContainText("42");
  await expect(section.getByRole("textbox", { name: "曼杰 日报目录 URL" })).toHaveValue(/\/paipai\/provider-files\/manjie$/);
});

test("downloads a provider note-grained daily report", async ({ page }) => {
  await page.route("**/paipai/api/downloads/maituo-provider/manjie", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ success: true, data: {
      provider_code: "manjie", provider_name: "曼杰",
      reports: [{ report_date: "2026-08-23", file_name: "2026-08-23-Maituo-客户日报-曼杰.xlsx", note_count: 19 }]
    } })
  }));

  await page.goto("/paipai/provider-files/manjie");
  await expect(page.getByRole("heading", { name: "曼杰" })).toBeVisible();
  await expect(page.getByRole("cell", { name: "19" })).toBeVisible();
  await expect(page.getByRole("link", { name: "下载 2026-08-23 报表" })).toHaveAttribute(
    "href", "/paipai/api/downloads/maituo-provider/manjie/2026-08-23.xlsx"
  );
});
