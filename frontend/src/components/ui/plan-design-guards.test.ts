import { readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * Design guard for docs/planung-redesign/docs/04-designsprache.md Abschnitt 5
 * (Verbote) and the planning-redesign Fertig-Kriterium 9:
 *
 *   (a) No gradients (`bg-gradient`, `from-[`, `to-[`) anywhere in scope,
 *       except the single sanctioned `repeating-linear-gradient` hatch that
 *       is hard-encapsulated in `plan-block.tsx` and nowhere else.
 *   (b) No generic Tailwind bright-color utility classes
 *       ((text|bg|border)-{color}-{shade}) used for semantics. `gray-*` is
 *       exempt — semantic color always routes through LOCATION_COLORS hex
 *       or a color prop, per .claude/rules/frontend-ui-kit.md.
 *
 * Scope: the new planning kit primitives (`ui/plan*.tsx`, `ui/coverage*.tsx`,
 * `ui/resource*.tsx`, `ui/capacity*.tsx`) PLUS every `.tsx` file under
 * `components/timetable/`. The new primitive
 * files get a ZERO-tolerance check — no allowlist entries, ever. The
 * pre-existing timetable files may carry violations that existed before
 * this guard was introduced; those are captured in a shrink-only allowlist
 * below (mirrors the backend ratchet-test pattern in
 * backend/test/*_ratchet_test.go). The allowlist numbers may only go down
 * as files get cleaned up — never up, and never a new entry for a new
 * violation.
 */

const UI_DIR = path.dirname(fileURLToPath(import.meta.url));
const TIMETABLE_DIR = path.resolve(UI_DIR, "../timetable");
const STAFF_DIR = path.resolve(UI_DIR, "../staff");

// Additive zero-tolerance scope (Planung-Redesign Inkrement 3, Dienstplan):
// these screen files consume PlanBlock/ResourceGrid end-to-end and probed
// clean against both patterns before being added here — same zero-tolerance
// bar as the kit primitives below, no allowlist. Explicit file list, not a
// directory glob: components/staff/ also holds unrelated screens that were
// never audited for this guard.
const DIENSTPLAN_PLANNING_FILES = [
  "dienstplan-halbjahr-grid.tsx",
  "dienstplan-resource-grid.tsx",
  "shift-move-dialog.tsx",
];

const GRADIENT_PATTERN = /bg-gradient|from-\[|to-\[/g;
const BRIGHT_COLOR_PATTERN =
  /(text|bg|border)-(red|green|blue|sky|orange|purple|amber|yellow|emerald|pink|rose|indigo|violet|cyan|teal|lime|fuchsia)-\d+/g;

// The one sanctioned hatch pattern (docs/04-designsprache.md 4/5/6.2),
// centralized in plan-block.tsx. It is CSS (`repeating-linear-gradient(...)`
// as an inline style value), not a Tailwind gradient utility class, so it
// never actually matches GRADIENT_PATTERN — but we guard the encapsulation
// explicitly below rather than relying on that coincidence.
const HATCH_LITERAL =
  "repeating-linear-gradient(45deg, transparent 0 4px, rgba(107,114,128,0.15) 4px 8px)";
const PLAN_BLOCK_FILE = "plan-block.tsx";

// Shrink-only allowlist for pre-existing generic-bright-color violations in
// components/timetable/, inventoried via
// `rg -o "(text|bg|border)-(red|green|...)-[0-9]+" frontend/src/components/timetable`
// on 2026-07-14, before this guard existed. Every entry here is a KNOWN
// altlast, not an endorsement — fix and remove the entry when you touch the
// file. New files, and new violations in already-listed files, are NOT
// covered: the guard fails the moment a file's count exceeds what's here.
// Keys are paths relative to components/timetable/ (subdirectories included,
// e.g. "event-form/field.tsx") since the scan below is recursive.
//
// 2026-07-16: all 14 remaining violations (timetable-event-modal.tsx,
// event-form/field.tsx, event-form/step-wiederholung.tsx,
// event-form/step-personal-kinder.tsx) were replaced with the kit-sanctioned
// `LOCATION_COLORS` hex equivalents (#FF3130 for field-error text and the
// series-delete confirm button, #EAB308 for the Dienstplan coverage warning
// panel). Allowlist is empty — zero tolerance restored.
const TIMETABLE_BRIGHT_COLOR_ALLOWLIST: Readonly<Record<string, number>> = {};

function countMatches(source: string, pattern: RegExp): number {
  return [...source.matchAll(pattern)].length;
}

/** Recursively lists .tsx files under `dir`, returning paths relative to `dir` (posix-style). */
function listTsxFiles(dir: string, relativeTo: string = dir): string[] {
  const entries = readdirSync(dir);
  const results: string[] = [];
  for (const name of entries) {
    const fullPath = path.join(dir, name);
    if (statSync(fullPath).isDirectory()) {
      results.push(...listTsxFiles(fullPath, relativeTo));
    } else if (name.endsWith(".tsx")) {
      results.push(
        path.relative(relativeTo, fullPath).split(path.sep).join("/"),
      );
    }
  }
  return results.sort();
}

function planPrimitiveFiles(): string[] {
  return listTsxFiles(UI_DIR).filter((name) =>
    /^(plan|coverage|resource|capacity)/.test(name),
  );
}

describe("plan-design-guards", () => {
  describe("new planning kit primitives (ui/plan*.tsx, ui/coverage*.tsx, ui/resource*.tsx, ui/capacity*.tsx)", () => {
    const files = planPrimitiveFiles();

    it("found the new primitive source files (sanity check on the glob)", () => {
      expect(files).toEqual(
        expect.arrayContaining([
          "capacity-strip.tsx",
          "coverage-indicator.tsx",
          "plan-block.tsx",
          "plan-legend.tsx",
          "planning-context-bar.tsx",
          "resource-grid.tsx",
        ]),
      );
    });

    it.each(files)(
      "%s has zero gradient violations (bg-gradient|from-[|to-[)",
      (file) => {
        const source = readFileSync(path.join(UI_DIR, file), "utf-8");
        expect(countMatches(source, GRADIENT_PATTERN)).toBe(0);
      },
    );

    it.each(files)(
      "%s uses zero generic Tailwind bright-color classes",
      (file) => {
        const source = readFileSync(path.join(UI_DIR, file), "utf-8");
        expect(countMatches(source, BRIGHT_COLOR_PATTERN)).toBe(0);
      },
    );
  });

  describe("Dienstplan planning screens (components/staff/)", () => {
    const files = DIENSTPLAN_PLANNING_FILES;

    it("found the Dienstplan planning source files (sanity check on the list)", () => {
      for (const file of files) {
        expect(() =>
          readFileSync(path.join(STAFF_DIR, file), "utf-8"),
        ).not.toThrow();
      }
    });

    it.each(files)(
      "%s has zero gradient violations (bg-gradient|from-[|to-[)",
      (file) => {
        const source = readFileSync(path.join(STAFF_DIR, file), "utf-8");
        expect(countMatches(source, GRADIENT_PATTERN)).toBe(0);
      },
    );

    it.each(files)(
      "%s uses zero generic Tailwind bright-color classes",
      (file) => {
        const source = readFileSync(path.join(STAFF_DIR, file), "utf-8");
        expect(countMatches(source, BRIGHT_COLOR_PATTERN)).toBe(0);
      },
    );
  });

  describe("repeating-linear-gradient hatch encapsulation", () => {
    // Only production source files are checked here — .test.tsx/.stories.tsx
    // siblings legitimately reference the same literal to verify it, which
    // isn't a second implementation of the hatch, just a test asserting on
    // the one in plan-block.tsx.
    const isProductionSourceFile = (file: string): boolean =>
      !file.endsWith(".test.tsx") && !file.endsWith(".stories.tsx");

    const scannedFiles = [
      ...planPrimitiveFiles()
        .filter(isProductionSourceFile)
        .map((file) => ({ file, dir: UI_DIR })),
      ...listTsxFiles(TIMETABLE_DIR)
        .filter(isProductionSourceFile)
        .map((file) => ({ file, dir: TIMETABLE_DIR })),
      ...DIENSTPLAN_PLANNING_FILES.map((file) => ({ file, dir: STAFF_DIR })),
    ];

    it("only plan-block.tsx references repeating-linear-gradient", () => {
      const offenders = scannedFiles
        .filter(({ file }) => file !== PLAN_BLOCK_FILE)
        .filter(({ file, dir }) => {
          const source = readFileSync(path.join(dir, file), "utf-8");
          return source.includes("repeating-linear-gradient");
        })
        .map(({ file }) => file);
      expect(offenders).toEqual([]);
    });

    it("plan-block.tsx contains exactly the sanctioned hatch literal", () => {
      const source = readFileSync(path.join(UI_DIR, PLAN_BLOCK_FILE), "utf-8");
      expect(source).toContain(HATCH_LITERAL);
    });
  });

  describe("components/timetable/*.tsx", () => {
    const files = listTsxFiles(TIMETABLE_DIR);

    it("found timetable files (sanity check on the directory scan)", () => {
      expect(files.length).toBeGreaterThan(0);
    });

    it.each(files)(
      "%s has zero gradient violations (bg-gradient|from-[|to-[)",
      (file) => {
        const source = readFileSync(path.join(TIMETABLE_DIR, file), "utf-8");
        expect(countMatches(source, GRADIENT_PATTERN)).toBe(0);
      },
    );

    it.each(files)(
      "%s does not exceed its allowed generic-bright-color count (shrink-only)",
      (file) => {
        const source = readFileSync(path.join(TIMETABLE_DIR, file), "utf-8");
        const allowed = TIMETABLE_BRIGHT_COLOR_ALLOWLIST[file] ?? 0;
        const found = countMatches(source, BRIGHT_COLOR_PATTERN);
        expect(found).toBeLessThanOrEqual(allowed);
      },
    );

    it("the allowlist has no stale entries for files that no longer exist", () => {
      for (const file of Object.keys(TIMETABLE_BRIGHT_COLOR_ALLOWLIST)) {
        expect(files).toContain(file);
      }
    });
  });
});
