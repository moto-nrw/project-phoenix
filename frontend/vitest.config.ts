import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";
import { availableParallelism } from "node:os";
import { nodeLogicTestFiles } from "./src/test/node-test-files.ts";
import { leanModuleGraphs } from "./src/test/lean-module-graphs.ts";
import { phosphorPerIconImports } from "./src/test/phosphor-imports-plugin.ts";
import {
  collectMainProcessGarbage,
  reusedThreads,
} from "./src/test/reused-threads-pool.ts";

// Test files of a reused worker share the natively loaded packages. These
// keep per-file state (Testing Library registers its cleanup hooks on
// import, SWR's cache, next-auth's session, user-event's document setup) and
// are evaluated again for every file, as a fresh worker would.
const perFileDependencies = [
  /@testing-library\/(react|user-event)/,
  /\/swr\//,
  /\/next-auth\//,
];

// Reused threads (src/test/reused-threads-pool.ts), measured on the full
// suite (1547 files, 11/2026): with 640 MB old space, a 16 MB young
// generation and a new thread once one ends a file above 200 MB heap, two
// workers peak at 1.7 GB process-tree RSS instead of 3.6 GB.
const reusedPool = reusedThreads({
  maxOldGenerationSizeMb: 640,
  maxYoungGenerationSizeMb: 16,
  recycleHeapMb: 200,
});

// The main process collects lazily up to a multi-gigabyte heap of which
// 0.2–0.3 GB is live; above 200 MB it collects every half second.
collectMainProcessGarbage({ aboveHeapMb: 200, everyMs: 500 });

const schoolDateTestFiles = [
  "src/app/traeger/einstellungen/zugriffsprotokoll/page.test.tsx",
  "src/app/operator/organizations/[slug]/traeger-erasure-order-section.test.tsx",
  "src/app/[tenant]/(protected)/students/[id]/feedback-history/page.test.tsx",
  "src/components/students/care-weekly-plan-editor.test.tsx",
  "src/app/[tenant]/(protected)/calendar/page.test.tsx",
  "src/app/[tenant]/(protected)/time-tracking/page.test.tsx",
  "src/components/students/class-trip-bulk-status-modal.test.tsx",
  "src/components/students/care-schedule-manager.test.tsx",
  "src/components/timetable/period-switcher-dropdown.test.tsx",
  "src/components/traeger/billing-letters-modal.test.tsx",
  "src/components/staff/arbeitszeitmodell-tab.preview.test.tsx",
  "src/components/database/grade-transitions/grade-transitions-manager.test.tsx",
  "src/components/students/class-arrival-exception-panel.test.tsx",
  "src/components/students/planned-status-days-modal.test.tsx",
  "src/components/calendar/personal-calendar.test.tsx",
  "src/app/school/kalender/page.test.tsx",
  "src/components/time-tracking/vacation-request-modal.test.tsx",
  "src/components/ui/date-picker.test.tsx",
  "src/components/enrollment/phases-editor.test.tsx",
  "src/components/time-tracking/leave-requests-card.test.tsx",
  "src/components/staff/stammdaten-tab.test.tsx",
  "src/app/[tenant]/(protected)/students/[id]/room-history/page.test.tsx",
  "src/components/class-day/class-day-class.test.tsx",
  "src/components/staff/abwesenheiten-tab.allowances.test.tsx",
  "src/components/ui/date-range-picker.test.tsx",
  "src/components/display/display-clock.test.tsx",
  "src/app/[tenant]/(protected)/students/[id]/page.test.tsx",
  "src/app/[tenant]/(protected)/staff/[id]/page.permissions.test.tsx",
  "src/components/staff/sick-report-modal.test.tsx",
  // Browser outside Germany, school times stay Berlin (#6293).
  "src/components/staff/admin-session-edit-modal.timezone.test.tsx",
  "src/components/staff/staff-session-table.timezone.test.tsx",
  "src/components/time-tracking/edit-history-accordion.timezone.test.tsx",
  "src/components/staff/staff-audit-log.timezone.test.tsx",
  // Shared date/time helpers show school time in every browser zone (#6471).
  "src/lib/date-helpers.school-time.test.ts",
  "src/components/rooms/room-detail-content.timezone.test.tsx",
  // Presence badges show „seit …“ in Berlin time (#6563).
  "src/components/students/presence/presence-badges.timezone.test.tsx",
];

// Shared desktop viewport and resource isolation for every DOM project.
const domEnvironmentOptions = {
  happyDOM: {
    width: 1920,
    height: 1080,
    // Component markup must not fetch marketing pages or vendor
    // resources. Tests mock fetch explicitly when that is their seam.
    settings: {
      navigation: {
        disableMainFrameNavigation: true,
        disableChildFrameNavigation: true,
        disableChildPageNavigation: true,
      },
      disableCSSFileLoading: true,
      disableJavaScriptFileLoading: true,
      enableImageFileLoading: false,
      handleDisabledFileLoadingAsSuccess: true,
    },
  },
};

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
  plugins: [phosphorPerIconImports(), react(), leanModuleGraphs()],
  // Vitest empties every CSS import before Vite's CSS plugin sees it
  // (test.css is off), yet Vite would still load postcss.config.mjs and with
  // it Tailwind's native scanner and lightningcss into the main process.
  // The inline config with no plugins skips that lookup.
  css: { postcss: {} },
  test: {
    globals: true,
    silent: "passed-only",
    // Transformed modules are kept in node_modules/.experimental-vitest-cache,
    // keyed by file content, Vite config (this file and its imports) and
    // lockfile; a repeated run reads them instead of transforming again
    // (full suite, 11/2026: main process 26 → 9 CPU-s). A plugin whose
    // output depends on other files must add those to the key
    // (defineCacheKeyGenerator). `pnpm vitest --clearCache` empties it.
    experimental: { fsModuleCache: true },
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
    // Vitests eigenes isolate: false allein bleibt verworfen: Module samt
    // vi.mock-Ersatz bleiben über Dateigrenzen im Cache (shift-api*.test.ts
    // mocken session-cache verschieden), api-node scheiterte so in 165 von
    // 274 Dateien. Die Projekte unten verwenden Worker deshalb nur mit dem
    // Reset pro Datei aus src/test/setup-worker-reuse.ts wieder (Module,
    // Mocks, Globals, DOM, Listener, Timer); gemessen auf der vollen Suite
    // (1547 Dateien, 11/2026) 850 → 330 CPU-s.
    pool: "threads",
    // Höchstens die Hälfte der CPUs, lokal nie mehr als zwei Worker, in CI
    // (eigener Runner, RAM kein Engpass) nie mehr als vier. Subprozess-Tests
    // brauchen CPU-Spielraum, damit weder einzelne Tests noch neu gestartete
    // Worker verhungern. Mit wiederverwendeten Workern (volle Suite, 11/2026):
    // 4 → 2 Worker senkt den Peak-RSS um rund 0,8 GB bei 88 → 115 s Wandzeit.
    maxWorkers: Math.max(
      1,
      Math.min(process.env.CI ? 4 : 2, Math.floor(availableParallelism() / 2)),
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
          isolate: false,
          pool: reusedPool,
          setupFiles: [
            "./src/test/setup-worker-reuse.ts",
            "./src/test/setup-common.ts",
          ],
          sequence: { groupOrder: 1 },
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
          isolate: false,
          pool: reusedPool,
          setupFiles: [
            "./src/test/setup-worker-reuse.ts",
            "./src/test/setup-common.ts",
          ],
          sequence: { groupOrder: 1 },
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
            ...schoolDateTestFiles,
          ],
          environment: "happy-dom",
          environmentOptions: domEnvironmentOptions,
          isolate: false,
          pool: reusedPool,
          server: { deps: { inline: perFileDependencies } },
          setupFiles: [
            "./src/test/setup-worker-reuse.ts",
            "./src/test/setup-common.ts",
            "./src/test/setup.ts",
          ],
          sequence: { groupOrder: 2 },
        },
      },
      {
        extends: true,
        test: {
          name: "school-date-dom",
          include: [
            ...schoolDateTestFiles,
            "src/test/resource-isolation.test.ts",
          ],
          environment: "happy-dom",
          environmentOptions: domEnvironmentOptions,
          // Node only applies runtime TZ changes in processes, not threads.
          // Keep real non-Berlin browser zones in this regression suite.
          // Reused processes run first, while the main process is still
          // small; process.env (TZ included) is restored after every file.
          pool: "forks",
          isolate: false,
          execArgv: ["--max-old-space-size=512"],
          server: { deps: { inline: perFileDependencies } },
          setupFiles: [
            "./src/test/setup-worker-reuse.ts",
            "./src/test/setup-common.ts",
            "./src/test/setup.ts",
          ],
          sequence: { groupOrder: 0 },
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
