import { expect, test, type Page } from "@playwright/test";

async function mockShell(page: Page) {
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, body: "ok" }));
  await page.route("**/paipai/api/imports/maituo-customer-daily", (route) => route.fulfill({
    status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: [] })
  }));
}

test("configuration helper explains fields, codes and decision impact", async ({ page }) => {
  await mockShell(page);
  await page.goto("/paipai/delivery/helper?field=bidding_strategy&level=campaign");

  await expect(page.getByRole("heading", { name: "聚光配置助手" })).toBeVisible();
  await expect(page.locator(".nav-item.active")).toHaveText("配置助手");
  await expect(page.getByLabel("搜索聚光字段或码值")).toHaveValue("bidding_strategy");
  const bidding = page.locator("#spotlight-doc-bidding_strategy");
  await expect(bidding).toHaveClass(/selected/);
  await expect(bidding).toContainText("放量还是成本稳定");
  await expect(page.getByRole("table", { name: "出价策略码值说明" })).toContainText("手动出价");
  await expect(page.getByRole("table", { name: "出价策略码值说明" })).toContainText("最大转化");
  await expect(page.getByRole("table", { name: "出价策略码值说明" })).toContainText("稳定成本");
  await expect(bidding.getByRole("link", { name: "查看原文" })).toHaveAttribute("href", /articleId=2722/);

  await page.getByLabel("搜索聚光字段或码值").fill("creation_type");
  const creation = page.locator("#spotlight-doc-creation_type");
  await expect(creation).toContainText("来源属性");
  await expect(page.getByRole("table", { name: "创建类型码值说明" })).toContainText("简单投放半自动");

  await page.getByLabel("搜索聚光字段或码值").fill("creativity_filter_state");
  await page.getByRole("group", { name: "配置层级筛选" }).getByRole("button", { name: "创意" }).click();
  const creativity = page.locator("#spotlight-doc-creativity_filter_state");
  await expect(creativity).toContainText("最终状态");
  await expect(page.getByRole("table", { name: "创意执行状态码值说明" })).toContainText("被计划暂停");
  await expect(page.getByRole("table", { name: "创意执行状态码值说明" })).toContainText("账户日预算不足");
});

test("configuration helper remains readable on mobile", async ({ page }) => {
  await mockShell(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/paipai/delivery/helper?field=creativity_filter_state&level=creativity");
  await expect(page.getByRole("heading", { name: "聚光配置助手" })).toBeVisible();
  await expect(page.locator("#spotlight-doc-creativity_filter_state")).toBeVisible();
  expect(await page.evaluate(() => document.body.scrollWidth <= window.innerWidth)).toBe(true);
});
