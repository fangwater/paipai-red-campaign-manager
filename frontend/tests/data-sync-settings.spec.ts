import { expect, test } from "@playwright/test";

const providers = [
  { provider_code: "manjie", provider_name: "曼杰", sheet_name: "达人笔记执行表", status: "succeeded", last_synced_at: "2026-07-24T13:42:58+08:00" },
  { provider_code: "youyiyouer", provider_name: "有一有二", sheet_name: "达人笔记执行表", status: "succeeded", last_synced_at: "2026-07-24T13:43:13+08:00" },
  { provider_code: "zhiyuan", provider_name: "智元", sheet_name: "koc稿件审核表", status: "succeeded", last_synced_at: "2026-07-24T13:42:33+08:00" }
];

test.beforeEach(async ({ page }) => {
  await page.route("**/paipai/healthz", (route) => route.fulfill({ status: 200, contentType: "application/json", body: '{"success":true}' }));
  await page.route("**/paipai/api/lark/sync/dandelion/status", (route) => route.fulfill({
    status: 200, contentType: "application/json",
    body: JSON.stringify({ success: true, data: { recent: [{ run_id: 3, status: "succeeded", fetched_count: 3103, upserted_count: 3103, deleted_count: 0, started_at: "2026-07-24T13:40:00+08:00", completed_at: "2026-07-24T13:40:10+08:00" }] } })
  }));
  await page.route("**/paipai/api/lark/sync/manuscripts/status", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: { providers } }) }));
  await page.route("**/paipai/api/xhs-jg/sync/status", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: { running: false, recent: [] } }) }));
});

test("syncs Dandelion and manuscript targets from real sidebar routes", async ({ page }) => {
  let dandelionCalls = 0;
  let manuscriptBody: unknown;
  await page.route("**/paipai/api/lark/sync/dandelion", async (route) => {
    dandelionCalls += 1;
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: { tables: 1, fetched: 3103, upserted: 18, deleted: 0 } }) });
  });
  await page.route("**/paipai/api/lark/sync/manuscripts", async (route) => {
    manuscriptBody = route.request().postDataJSON();
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ success: true, data: { providers: 3, fetched: 172, upserted: 172, deleted: 0, notes: 172, note_errors: 0 } }) });
  });

  await page.goto("/paipai/");
  await page.getByRole("button", { name: "蒲公英数据", exact: true }).click();
  await expect(page).toHaveURL(/\/paipai\/data-sync\/dandelion$/);
  await expect(page.getByRole("heading", { name: "飞书数据同步" })).toBeVisible();
  await expect(page.locator(".data-sync-target")).toHaveCount(2);
  await expect(page.locator(".provider-status-row")).toHaveCount(3);
  await page.getByRole("button", { name: "同步蒲公英", exact: true }).click();
  await expect.poll(() => dandelionCalls).toBe(1);
  await expect(page.locator(".data-sync-result")).toContainText("蒲公英数据同步完成");

  await page.getByRole("button", { name: "稿件数据", exact: true }).click();
  await expect(page).toHaveURL(/\/paipai\/data-sync\/manuscripts$/);
  await expect(page.locator(".data-sync-target.active")).toContainText("稿件数据");
  await page.getByRole("button", { name: "同步稿件", exact: true }).click();
  await expect.poll(() => manuscriptBody).toEqual({});
});

test("shows a functional read-only system status page", async ({ page }) => {
  await page.goto("/paipai/settings");
  await expect(page.getByRole("heading", { name: "系统设置" })).toBeVisible();
  await expect(page.locator(".service-status-row")).toHaveCount(3);
  await expect(page.locator(".service-health.online")).toHaveCount(3);
  await page.getByRole("button", { name: /聚光数据同步/ }).click();
  await expect(page).toHaveURL(/\/paipai\/xhs-jg-sync\/campaigns$/);
});
