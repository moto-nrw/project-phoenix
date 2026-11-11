import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import {
  BRIGHT_COLOR_PATTERN,
  GRADIENT_PATTERN,
  HATCH_MARKER,
  countMatches,
  isProductionSourceFile,
  listTsxFiles,
  readSource,
} from "~/test/plan-design-guards";

/**
 * Design guards for the shared planning kit primitives (`ui/planning/plan*.tsx`,
 * `coverage*.tsx`, `resource*.tsx`, `capacity*.tsx`): zero gradients, zero
 * generic bright-color classes, no allowlist entries, ever. The screens built
 * from them guard themselves next to their files
 * (`timetable/timetable-design-guards.test.ts`,
 * `staff/dienstplan-design-guards.test.ts`). Patterns: `~/test/plan-design-guards`.
 */

const UI_DIR = path.dirname(fileURLToPath(import.meta.url));

// The one sanctioned hatch pattern,
// centralized in plan-block.tsx. It is CSS (`repeating-linear-gradient(...)`
// as an inline style value), not a Tailwind gradient utility class, so it
// never actually matches GRADIENT_PATTERN — but we guard the encapsulation
// explicitly below rather than relying on that coincidence.
const HATCH_LITERAL =
  "repeating-linear-gradient(45deg, transparent 0 4px, rgba(107,114,128,0.15) 4px 8px)";
const PLAN_BLOCK_FILE = "plan-block.tsx";

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
        expect(countMatches(readSource(UI_DIR, file), GRADIENT_PATTERN)).toBe(
          0,
        );
      },
    );

    it.each(files)(
      "%s uses zero generic Tailwind bright-color classes",
      (file) => {
        expect(
          countMatches(readSource(UI_DIR, file), BRIGHT_COLOR_PATTERN),
        ).toBe(0);
      },
    );
  });

  describe("repeating-linear-gradient hatch encapsulation", () => {
    it("only plan-block.tsx references repeating-linear-gradient", () => {
      const offenders = planPrimitiveFiles()
        .filter(isProductionSourceFile)
        .filter((file) => file !== PLAN_BLOCK_FILE)
        .filter((file) => readSource(UI_DIR, file).includes(HATCH_MARKER));
      expect(offenders).toEqual([]);
    });

    it("plan-block.tsx contains exactly the sanctioned hatch literal", () => {
      expect(readSource(UI_DIR, PLAN_BLOCK_FILE)).toContain(HATCH_LITERAL);
    });
  });
});
