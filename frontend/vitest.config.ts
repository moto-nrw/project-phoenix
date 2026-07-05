import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

// Pin the test process to Berlin time. The app reasons in Europe/Berlin
// wall-clock (backend timestamps, reminder thresholds, calendar-date handling),
// and several tests drive fake system time and assert against wall-clock
// boundaries. Without a fixed zone those assertions depend on the CI machine's
// timezone; forcing Berlin makes them deterministic and matches production
// semantics. Set before workers spawn so Date's local-zone cache picks it up.
process.env.TZ = "Europe/Berlin";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "happy-dom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    exclude: ["**/node_modules/**", "**/e2e/**"], // Exclude Playwright tests
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html", "lcov"],
      reportOnFailure: true, // Generate coverage even when tests fail
      exclude: [
        "node_modules/",
        "src/test/",
        "**/*.config.*",
        "**/types.ts",
        "**/*.d.ts",
        "src/env.js",
      ],
    },
  },
  resolve: {
    alias: {
      "~": path.resolve(__dirname, "./src"),
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
