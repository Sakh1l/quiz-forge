import { defineConfig } from "@playwright/test";
import path from "path";

const repoRoot = path.join(__dirname, "..");

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  workers: 1,
  timeout: 60_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: "http://127.0.0.1:8099",
    trace: "on-first-retry",
  },
  webServer: {
    command:
      "go build -o /tmp/quiz-forge-e2e ./cmd/server && PORT=8099 /tmp/quiz-forge-e2e",
    cwd: repoRoot,
    url: "http://127.0.0.1:8099/",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
