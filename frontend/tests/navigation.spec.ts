import { expect, test } from "@playwright/test";

test("opens the Maituo daily report import from the console home", async ({ page }) => {
  await page.goto("/paipai/");
  await expect(page.getByRole("heading", { name: "数据中台" })).toBeVisible();
  await page.getByRole("button", { name: /更新 Maituo 客户日报/ }).click();
  await expect(page).toHaveURL(/\/paipai\/maituo-daily-report$/);
  await expect(page.getByRole("heading", { name: "Maituo 客户日报" })).toBeVisible();
});
