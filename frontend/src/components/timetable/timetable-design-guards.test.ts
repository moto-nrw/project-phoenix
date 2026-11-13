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
 * Planning design guards for every `.tsx` file under `components/timetable/`
 * (patterns and rationale: `~/test/plan-design-guards`). The pre-existing
 * timetable files may carry violations that existed before this guard was
 * introduced; those are captured in a shrink-only allowlist below (mirrors
 * the backend ratchet-test pattern in backend/test/*_ratchet_test.go). The
 * allowlist numbers may only go down as files get cleaned up — never up, and
 * never a new entry for a new violation.
 */

const TIMETABLE_DIR = path.dirname(fileURLToPath(import.meta.url));

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
// `LOCATION_COLORS` hex equivalents (#DC2626 for field-error text and the
// series-delete confirm button, #EAB308 for the Dienstplan coverage warning
// panel). Allowlist is empty — zero tolerance restored.
const TIMETABLE_BRIGHT_COLOR_ALLOWLIST: Readonly<Record<string, number>> = {};

describe("plan-design-guards", () => {
  describe("repeating-linear-gradient hatch encapsulation", () => {
    it("no timetable file references repeating-linear-gradient", () => {
      const offenders = listTsxFiles(TIMETABLE_DIR)
        .filter(isProductionSourceFile)
        .filter((file) =>
          readSource(TIMETABLE_DIR, file).includes(HATCH_MARKER),
        );
      expect(offenders).toEqual([]);
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
        expect(
          countMatches(readSource(TIMETABLE_DIR, file), GRADIENT_PATTERN),
        ).toBe(0);
      },
    );

    it.each(files)(
      "%s does not exceed its allowed generic-bright-color count (shrink-only)",
      (file) => {
        const allowed = TIMETABLE_BRIGHT_COLOR_ALLOWLIST[file] ?? 0;
        const found = countMatches(
          readSource(TIMETABLE_DIR, file),
          BRIGHT_COLOR_PATTERN,
        );
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
