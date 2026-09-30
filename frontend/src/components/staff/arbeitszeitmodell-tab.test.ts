import { describe, expect, it } from "vitest";
import {
  formatDecimalHours,
  parseDecimalHours,
  isStaleAfterModelSave,
} from "./arbeitszeitmodell-tab";

describe("arbeitszeitmodell-tab.decimal-hours", () => {
  describe("parseDecimalHours", () => {
    it.each([
      ["4,45", 267],
      ["4.45", 267],
      ["0,33", 20],
      ["0,025", 2],
      ["1,025", 62],
      [",5", 30],
      [".5", 30],
      [" 12,00 ", 720],
      ["0", 0],
    ])("converts %s to integer minutes", (value, minutes) => {
      expect(parseDecimalHours(value)).toEqual({ status: "valid", minutes });
    });

    it("treats an empty value as an empty work day", () => {
      expect(parseDecimalHours("  ")).toEqual({ status: "empty" });
    });

    it.each([
      "-0,01",
      "12,01",
      "12.0000000000000001",
      "13",
      "abc",
      "4,4,5",
      "1e2",
      "NaN",
      "Infinity",
    ])("rejects %s", (value) => {
      expect(parseDecimalHours(value)).toEqual({ status: "invalid" });
    });
  });

  describe("formatDecimalHours", () => {
    it.each([
      [1, "0,02"],
      [20, "0,33"],
      [59, "0,98"],
      [60, "1"],
      [267, "4,45"],
      [719, "11,98"],
      [720, "12"],
    ])("formats %i existing minutes as %s", (minutes, value) => {
      expect(formatDecimalHours(minutes)).toBe(value);
      expect(parseDecimalHours(value)).toEqual({ status: "valid", minutes });
    });
  });
});

describe("arbeitszeitmodell-tab.invalidation", () => {
  // After a work-time model save the contractual Soll changes, so the sibling
  // Zeiterfassung / Übersicht tabs' cached targets and month aggregates go stale
  // and must be invalidated — otherwise the open staff page shows the old Soll
  // until a reload (#1842). useSWRAuth prefixes keys with the tenant slug, so the
  // predicate matches with includes.
  describe("isStaleAfterModelSave", () => {
    it("invalidates the date-valid targets and month aggregate caches", () => {
      for (const key of [
        "phoenix:staff-schedule-targets-42-2026-06-01-2026-06-30",
        "phoenix:staff-schedule-targets-account-42-2026-01-01-2026-07-16",
        "phoenix:staff-month-summary-42-2026-7",
        "phoenix:time-tracking-month-summary-2026-7",
        // Own-service portal keys (no staff id): a manager editing their OWN
        // model must also refresh the self-service daily table and weekly KPI,
        // which key without an id (#1842).
        "phoenix:time-tracking-schedule-targets-2026-06-01-2026-06-30",
      ]) {
        expect(isStaleAfterModelSave(key)).toBe(true);
      }
    });

    it("leaves unrelated caches alone", () => {
      for (const key of [
        "phoenix:staff-schedule-42",
        "phoenix:staff-history-42-2026-06-01-2026-06-30",
        "phoenix:staff-absences-42-2026-06-01-2026-06-30",
        "time-tracking-config",
      ]) {
        expect(isStaleAfterModelSave(key)).toBe(false);
      }
    });

    it("ignores non-string SWR keys", () => {
      expect(isStaleAfterModelSave(null)).toBe(false);
      expect(isStaleAfterModelSave(["staff-month-summary-42"])).toBe(false);
      expect(isStaleAfterModelSave(undefined)).toBe(false);
    });
  });
});
