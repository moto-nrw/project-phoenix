import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  isActiveGroupCurrent,
  isVisitActive,
  isSupervisionActive,
  isCombinedGroupActive,
  formatDuration,
  getCurrentVisit,
  getActiveSupervisions,
  getActiveGroups,
} from "./active-api";

// Mock re-exports to isolate utility functions
vi.mock("./active-service", () => ({ activeService: {} }));
vi.mock("./active-helpers", () => ({}));

describe("active-api utility functions", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2024-01-15T10:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe("isActiveGroupCurrent", () => {
    it("returns true when endTime is undefined", () => {
      expect(isActiveGroupCurrent(undefined)).toBe(true);
    });

    it("returns true when endTime is in the future", () => {
      const futureDate = new Date("2024-01-15T12:00:00Z");
      expect(isActiveGroupCurrent(futureDate)).toBe(true);
    });

    it("returns false when endTime is in the past", () => {
      const pastDate = new Date("2024-01-15T08:00:00Z");
      expect(isActiveGroupCurrent(pastDate)).toBe(false);
    });
  });

  describe("isVisitActive", () => {
    it("returns true when checkOutTime is undefined", () => {
      expect(isVisitActive(undefined)).toBe(true);
    });

    it("returns true when checkOutTime is in the future", () => {
      const futureDate = new Date("2024-01-15T12:00:00Z");
      expect(isVisitActive(futureDate)).toBe(true);
    });

    it("returns false when checkOutTime is in the past", () => {
      const pastDate = new Date("2024-01-15T08:00:00Z");
      expect(isVisitActive(pastDate)).toBe(false);
    });
  });

  describe("isSupervisionActive", () => {
    it("returns true when endTime is undefined", () => {
      expect(isSupervisionActive(undefined)).toBe(true);
    });

    it("returns true when endTime is in the future", () => {
      const futureDate = new Date("2024-01-15T12:00:00Z");
      expect(isSupervisionActive(futureDate)).toBe(true);
    });

    it("returns false when endTime is in the past", () => {
      const pastDate = new Date("2024-01-15T08:00:00Z");
      expect(isSupervisionActive(pastDate)).toBe(false);
    });
  });

  describe("isCombinedGroupActive", () => {
    it("returns true when endTime is undefined", () => {
      expect(isCombinedGroupActive(undefined)).toBe(true);
    });

    it("returns true when endTime is in the future", () => {
      const futureDate = new Date("2024-01-15T12:00:00Z");
      expect(isCombinedGroupActive(futureDate)).toBe(true);
    });

    it("returns false when endTime is in the past", () => {
      const pastDate = new Date("2024-01-15T08:00:00Z");
      expect(isCombinedGroupActive(pastDate)).toBe(false);
    });
  });

  describe("formatDuration", () => {
    it("formats duration with hours and minutes", () => {
      const start = new Date("2024-01-15T08:00:00Z");
      const end = new Date("2024-01-15T10:30:00Z");
      expect(formatDuration(start, end)).toBe("2h 30m");
    });

    it("formats duration with only minutes when less than 1 hour", () => {
      const start = new Date("2024-01-15T09:30:00Z");
      const end = new Date("2024-01-15T10:00:00Z");
      expect(formatDuration(start, end)).toBe("30m");
    });

    it("uses current time when end is not provided", () => {
      const start = new Date("2024-01-15T08:00:00Z");
      // Current time is 10:00:00, so duration is 2h 0m
      expect(formatDuration(start)).toBe("2h 0m");
    });

    it("formats duration with only hours when minutes are zero", () => {
      const start = new Date("2024-01-15T08:00:00Z");
      const end = new Date("2024-01-15T11:00:00Z");
      expect(formatDuration(start, end)).toBe("3h 0m");
    });
  });

  describe("getCurrentVisit", () => {
    it("returns undefined for empty array", () => {
      expect(getCurrentVisit([])).toBeUndefined();
    });

    it("returns undefined when no visit is active", () => {
      const visits = [
        { checkOutTime: new Date("2024-01-15T08:00:00Z") },
        { checkOutTime: new Date("2024-01-15T09:00:00Z") },
      ];
      expect(getCurrentVisit(visits)).toBeUndefined();
    });

    it("returns the active visit (with undefined checkOutTime)", () => {
      const activeVisit = { checkOutTime: undefined };
      const visits = [
        { checkOutTime: new Date("2024-01-15T08:00:00Z") },
        activeVisit,
      ];
      expect(getCurrentVisit(visits)).toBe(activeVisit);
    });

    it("returns the active visit (with future checkOutTime)", () => {
      const activeVisit = { checkOutTime: new Date("2024-01-15T12:00:00Z") };
      const visits = [
        { checkOutTime: new Date("2024-01-15T08:00:00Z") },
        activeVisit,
      ];
      expect(getCurrentVisit(visits)).toBe(activeVisit);
    });

    it("returns the first active visit when multiple are active", () => {
      const firstActive = { checkOutTime: undefined };
      const secondActive = { checkOutTime: new Date("2024-01-15T12:00:00Z") };
      const visits = [firstActive, secondActive];
      expect(getCurrentVisit(visits)).toBe(firstActive);
    });
  });

  describe("getActiveSupervisions", () => {
    it("returns empty array when input is empty", () => {
      expect(getActiveSupervisions([])).toEqual([]);
    });

    it("filters out supervisions with past endTime", () => {
      const supervisions = [
        { endTime: new Date("2024-01-15T08:00:00Z") },
        { endTime: new Date("2024-01-15T09:00:00Z") },
      ];
      expect(getActiveSupervisions(supervisions)).toEqual([]);
    });

    it("returns supervisions with undefined endTime", () => {
      const active = { endTime: undefined };
      const supervisions = [
        { endTime: new Date("2024-01-15T08:00:00Z") },
        active,
      ];
      expect(getActiveSupervisions(supervisions)).toEqual([active]);
    });

    it("returns supervisions with future endTime", () => {
      const active = { endTime: new Date("2024-01-15T12:00:00Z") };
      const supervisions = [
        { endTime: new Date("2024-01-15T08:00:00Z") },
        active,
      ];
      expect(getActiveSupervisions(supervisions)).toEqual([active]);
    });

    it("returns multiple active supervisions", () => {
      const active1 = { endTime: undefined };
      const active2 = { endTime: new Date("2024-01-15T12:00:00Z") };
      const supervisions = [
        active1,
        { endTime: new Date("2024-01-15T08:00:00Z") },
        active2,
      ];
      expect(getActiveSupervisions(supervisions)).toEqual([active1, active2]);
    });
  });

  describe("getActiveGroups", () => {
    it("returns empty array when input is empty", () => {
      expect(getActiveGroups([])).toEqual([]);
    });

    it("filters out groups with past endTime", () => {
      const groups = [
        { endTime: new Date("2024-01-15T08:00:00Z") },
        { endTime: new Date("2024-01-15T09:00:00Z") },
      ];
      expect(getActiveGroups(groups)).toEqual([]);
    });

    it("returns groups with undefined endTime", () => {
      const active = { endTime: undefined };
      const groups = [{ endTime: new Date("2024-01-15T08:00:00Z") }, active];
      expect(getActiveGroups(groups)).toEqual([active]);
    });

    it("returns groups with future endTime", () => {
      const active = { endTime: new Date("2024-01-15T12:00:00Z") };
      const groups = [{ endTime: new Date("2024-01-15T08:00:00Z") }, active];
      expect(getActiveGroups(groups)).toEqual([active]);
    });

    it("returns multiple active groups", () => {
      const active1 = { endTime: undefined };
      const active2 = { endTime: new Date("2024-01-15T12:00:00Z") };
      const groups = [
        active1,
        { endTime: new Date("2024-01-15T08:00:00Z") },
        active2,
      ];
      expect(getActiveGroups(groups)).toEqual([active1, active2]);
    });
  });
});
