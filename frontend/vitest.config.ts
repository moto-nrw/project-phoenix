import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { availableParallelism } from "node:os";

const apiTestFiles = ["src/app/api/**/*.{test,spec}.ts"];
const baseTestExcludes = ["**/node_modules/**", "**/e2e/**"];

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
    globals: true,
    silent: "passed-only",
    // threads statt des Default-Pools "forks": gemessen auf der vollen Suite
    // (1027 Dateien / 14160 Tests, 16-Core-MacBook) 124s → 76s Wandzeit und
    // -24% CPU — der Unterschied ist reiner Prozess-Spawn-/IPC-Overhead.
    pool: "threads",
    // Lokal höchstens die Hälfte der CPUs und nie mehr als vier Worker.
    // Gemessen auf 231 Dateien: 8 → 4 Worker senkt CPU um 19% und Peak-RSS
    // von 3,8 auf 2,7 GB; die Wandzeit steigt von 53 auf 67 Sekunden. CI hat
    // einen isolierten Runner und nutzt deshalb alle bezahlten CPUs.
    maxWorkers: process.env.CI
      ? availableParallelism()
      : Math.max(1, Math.min(4, Math.floor(availableParallelism() / 2))),
    projects: [
      {
        extends: true,
        test: {
          name: "api-node",
          include: apiTestFiles,
          exclude: baseTestExcludes,
          environment: "node",
          setupFiles: ["./src/test/setup-common.ts"],
          sequence: { groupOrder: 0 },
        },
      },
      {
        extends: true,
        test: {
          name: "app-dom",
          exclude: [...baseTestExcludes, ...apiTestFiles],
          environment: "happy-dom",
          // happy-dom simuliert sonst 1024x768; Komponenten mit
          // Viewport-abhängigen Defaults (z. B. die einklappbare
          // Seitenleiste, #2825) sollen in Tests den Desktop-Zustand
          // rendern, wie ihn auch der Server-Snapshot annimmt.
          environmentOptions: {
            happyDOM: { width: 1920, height: 1080 },
          },
          setupFiles: ["./src/test/setup-common.ts", "./src/test/setup.ts"],
          sequence: { groupOrder: 1 },
        },
      },
    ],
    coverage: {
      provider: "v8",
      // CI uploads only lcov.info for SonarCloud. Building JSON/HTML trees and
      // printing a 1,000-file table wastes CPU, disk, and log bandwidth.
      reporter: process.env.CI
        ? ["lcovonly"]
        : ["text", "json", "html", "lcov"],
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
      "~": path.resolve(import.meta.dirname, "./src"),
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
});
