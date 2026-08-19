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
    // threads statt des Default-Pools "forks": gemessen auf der vollen Suite
    // (1027 Dateien / 14160 Tests, 16-Core-MacBook) 124s → 76s Wandzeit und
    // -24% CPU — der Unterschied ist reiner Prozess-Spawn-/IPC-Overhead.
    pool: "threads",
    // Lokal auf 8 Worker gedeckelt, damit der Rechner während eines vollen
    // Laufs benutzbar bleibt (Default wäre ~15 auf 16 Cores). Kostet ~4s
    // Wandzeit, spart weitere ~13% CPU. CI bleibt ungedeckelt (läuft dort
    // ohnehin nur --changed).
    maxWorkers: process.env.CI ? undefined : 8,
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
