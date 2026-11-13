import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import {
  BRIGHT_COLOR_PATTERN,
  GRADIENT_PATTERN,
  HATCH_MARKER,
  countMatches,
  readSource,
} from "~/test/plan-design-guards";

/**
 * Planning design guards for the Dienstplan screens (patterns and rationale:
 * `~/test/plan-design-guards`).
 */

const STAFF_DIR = path.dirname(fileURLToPath(import.meta.url));

// Additive zero-tolerance scope (Planung-Redesign Inkrement 3, Dienstplan):
// these screen files consume PlanBlock/ResourceGrid end-to-end and probed
// clean against both patterns before being added here — same zero-tolerance
// bar as the kit primitives, no allowlist. Explicit file list, not a
// directory glob: components/staff/ also holds unrelated screens that were
// never audited for this guard.
const DIENSTPLAN_PLANNING_FILES = [
  "dienstplan-halbjahr-grid.tsx",
  "dienstplan-resource-grid.tsx",
  "shift-move-dialog.tsx",
];

describe("plan-design-guards", () => {
  describe("Dienstplan planning screens (components/staff/)", () => {
    const files = DIENSTPLAN_PLANNING_FILES;

    it("found the Dienstplan planning source files (sanity check on the list)", () => {
      for (const file of files) {
        expect(() => readSource(STAFF_DIR, file)).not.toThrow();
      }
    });

    it.each(files)(
      "%s has zero gradient violations (bg-gradient|from-[|to-[)",
      (file) => {
        expect(
          countMatches(readSource(STAFF_DIR, file), GRADIENT_PATTERN),
        ).toBe(0);
      },
    );

    it.each(files)(
      "%s uses zero generic Tailwind bright-color classes",
      (file) => {
        expect(
          countMatches(readSource(STAFF_DIR, file), BRIGHT_COLOR_PATTERN),
        ).toBe(0);
      },
    );
  });

  describe("repeating-linear-gradient hatch encapsulation", () => {
    it("no Dienstplan screen references repeating-linear-gradient", () => {
      const offenders = DIENSTPLAN_PLANNING_FILES.filter((file) =>
        readSource(STAFF_DIR, file).includes(HATCH_MARKER),
      );
      expect(offenders).toEqual([]);
    });
  });
});
