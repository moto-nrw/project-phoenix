import { describe, it, expect, vi } from "vitest";
import { isValidDateString, isDateExpired } from "./date-helpers";

describe("date-helpers", () => {
  describe("isValidDateString", () => {
    it("should return false for null", () => {
      expect(isValidDateString(null)).toBe(false);
    });

    it("should return false for undefined", () => {
      expect(isValidDateString(undefined)).toBe(false);
    });

    it("should return false for empty string", () => {
      expect(isValidDateString("")).toBe(false);
    });

    it("should return false for invalid date string", () => {
      expect(isValidDateString("not-a-date")).toBe(false);
      expect(isValidDateString("2024-13-01")).toBe(false); // Invalid month
      expect(isValidDateString("invalid")).toBe(false);
    });

    it("should return true for valid ISO date string", () => {
      expect(isValidDateString("2024-01-15")).toBe(true);
      expect(isValidDateString("2024-01-15T10:30:00Z")).toBe(true);
      expect(isValidDateString("2024-12-31T23:59:59.999Z")).toBe(true);
    });

    it("should return true for other valid date formats", () => {
      expect(isValidDateString("January 15, 2024")).toBe(true);
      expect(isValidDateString("2024/01/15")).toBe(true);
    });
  });

  describe("isDateExpired", () => {
    it("should return false for null", () => {
      expect(isDateExpired(null)).toBe(false);
    });

    it("should return false for undefined", () => {
      expect(isDateExpired(undefined)).toBe(false);
    });

    it("should return false for invalid date string", () => {
      expect(isDateExpired("not-a-date")).toBe(false);
      expect(isDateExpired("")).toBe(false);
    });

    it("should return true for date in the past", () => {
      // Mock Date.now to return a fixed timestamp (Feb 1, 2024)
      const mockNow = new Date("2024-02-01T12:00:00Z").getTime();
      vi.spyOn(Date, "now").mockReturnValue(mockNow);

      expect(isDateExpired("2024-01-01T00:00:00Z")).toBe(true);
      expect(isDateExpired("2023-12-31T23:59:59Z")).toBe(true);

      // Restore
      vi.spyOn(Date, "now").mockRestore();
    });

    it("should return false for date in the future", () => {
      // Mock Date.now to return a fixed timestamp (Jan 1, 2024)
      const mockNow = new Date("2024-01-01T12:00:00Z").getTime();
      vi.spyOn(Date, "now").mockReturnValue(mockNow);

      expect(isDateExpired("2024-12-31T23:59:59Z")).toBe(false);
      expect(isDateExpired("2025-01-01T00:00:00Z")).toBe(false);

      // Restore
      vi.spyOn(Date, "now").mockRestore();
    });

    it("should return true for current time (edge case)", () => {
      const now = new Date().toISOString();
      // Need to mock Date.now to a time slightly in the future
      const mockNow = Date.now() + 100;
      vi.spyOn(Date, "now").mockReturnValue(mockNow);

      expect(isDateExpired(now)).toBe(true);

      vi.spyOn(Date, "now").mockRestore();
    });
  });
});
