import { defineConfig, devices } from "@playwright/test";

const externalBaseURL = process.env.PLAYWRIGHT_BASE_URL;

export default defineConfig({
  testDir: "./tests",
  fullyParallel: true,
  use: {
    baseURL: externalBaseURL ?? "http://127.0.0.1:5173/paipai/",
    trace: "retain-on-failure"
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } }
  ],
  webServer: externalBaseURL ? undefined : {
    command: "npm run dev",
    url: "http://127.0.0.1:5173/paipai/",
    reuseExistingServer: true
  }
});
