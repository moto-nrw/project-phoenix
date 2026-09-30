import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { availableParallelism } from "node:os";
import { nodeLogicTestFiles } from "./src/test/node-test-files";
import { phosphorPerIconImports } from "./src/test/phosphor-imports-plugin";

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
  // Per-icon phosphor imports instead of the 3,000-module barrels; mirrors
  // optimizePackageImports in next.config.js. See the plugin file.
  plugins: [phosphorPerIconImports(), react()],
  test: {
    globals: true,
    silent: "passed-only",
    // threads statt des Default-Pools "forks": gemessen auf der vollen Suite
    // (1027 Dateien / 14160 Tests, 16-Core-MacBook) 124s → 76s Wandzeit und
    // -24% CPU — der Unterschied ist reiner Prozess-Spawn-/IPC-Overhead.
    //
    // "vmThreads" für app-dom ist verworfen (volle Suite, 1243 Dateien,
    // 4 Worker, 09/2026): Es spart rund 40 % CPU (306–346 s statt 484–532 s
    // user), weil happy-dom nicht pro Datei neu geladen wird. Der Peak-RSS
    // liegt aber bei jedem getesteten vmMemoryLimit über threads
    // (2,24–2,27 GB): 50MB 2,54 GB, 100MB 2,38–2,78 GB, 200MB 3,05 GB,
    // 300MB 2,64–2,82 GB, 500MB 3,28 GB, 1600MB (0,1 von 16 GB) 7,5 GB.
    // Ohne Limit recycelt Vitest erst bei RAM/maxWorkers, auf 16 GB also bei
    // 4 GB pro Worker. Außerdem scheitern darunter 52 Tests in 12 Dateien an
    // Realm-Grenzen (toStrictEqual-Prototypen, happy-dom-URL, File/Blob).
    //
    // isolate: false bleibt aus. logic-node spart damit etwa 14 s CPU, wird
    // aber reihenfolgeabhängig rot, weil Module samt vi.mock-Ersatz über
    // Dateigrenzen im Cache bleiben (shift-api*.test.ts mocken session-cache
    // verschieden). api-node scheitert so in 165 von 274 Dateien.
    pool: "threads",
    // Höchstens die Hälfte der CPUs und nie mehr als vier Worker.
    // Gemessen auf 231 Dateien: 8 → 4 Worker senkt CPU um 19% und Peak-RSS
    // von 3,8 auf 2,7 GB; die Wandzeit steigt von 53 auf 67 Sekunden. Das
    // gilt auch mit CI=true: Subprozess-Tests brauchen CPU-Spielraum, damit
    // weder einzelne Tests noch neu gestartete Worker verhungern.
    maxWorkers: Math.max(
      1,
      Math.min(4, Math.floor(availableParallelism() / 2)),
    ),
    projects: [
      {
        extends: true,
        test: {
          name: "api-node",
          server: { deps: { inline: ["next-auth"] } },
          include: apiTestFiles,
          exclude: baseTestExcludes,
          environment: "node",
          setupFiles: ["./src/test/setup-common.ts"],
          sequence: { groupOrder: 0 },
        },
      },
      {
        // Reine Logik-Tests ohne DOM: spart pro Datei den happy-dom-Aufbau
        // und das Laden von setup.ts (jest-dom, next-intl-Katalog, Mocks).
        extends: true,
        test: {
          name: "logic-node",
          include: nodeLogicTestFiles,
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
          exclude: [
            ...baseTestExcludes,
            ...apiTestFiles,
            ...nodeLogicTestFiles,
          ],
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
      // Coverage is a local tool only; CI does not collect it.
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
      "@testing-library/jest-dom/vitest": path.resolve(
        import.meta.dirname,
        "./src/test/setup-jest-dom.ts",
      ),
      "~": path.resolve(import.meta.dirname, "./src"),
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
});
