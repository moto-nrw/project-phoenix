import { describe, it, expect } from "vitest";
import { getRelativeTime, getInitial, getInitials } from "./format-utils";

describe("getRelativeTime", () => {
  it("returns 'gerade eben' for timestamps less than 1 minute ago", () => {
    const now = new Date().toISOString();
    expect(getRelativeTime(now)).toBe("gerade eben");
  });

  it("returns 'vor 1 Minute' for 1 minute ago", () => {
    const oneMinuteAgo = new Date(Date.now() - 60 * 1000).toISOString();
    expect(getRelativeTime(oneMinuteAgo)).toBe("vor 1 Minute");
  });

  it("returns 'vor X Minuten' for multiple minutes", () => {
    const fiveMinutesAgo = new Date(Date.now() - 5 * 60 * 1000).toISOString();
    expect(getRelativeTime(fiveMinutesAgo)).toBe("vor 5 Minuten");
  });

  it("returns 'vor 1 Stunde' for 1 hour ago", () => {
    const oneHourAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString();
    expect(getRelativeTime(oneHourAgo)).toBe("vor 1 Stunde");
  });

  it("returns 'vor X Stunden' for multiple hours", () => {
    const threeHoursAgo = new Date(
      Date.now() - 3 * 60 * 60 * 1000,
    ).toISOString();
    expect(getRelativeTime(threeHoursAgo)).toBe("vor 3 Stunden");
  });

  it("returns 'vor 1 Tag' for 1 day ago", () => {
    const oneDayAgo = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString();
    expect(getRelativeTime(oneDayAgo)).toBe("vor 1 Tag");
  });

  it("returns 'vor X Tagen' for multiple days", () => {
    const threeDaysAgo = new Date(
      Date.now() - 3 * 24 * 60 * 60 * 1000,
    ).toISOString();
    expect(getRelativeTime(threeDaysAgo)).toBe("vor 3 Tagen");
  });

  it("returns 'vor 1 Woche' for 1 week ago", () => {
    const oneWeekAgo = new Date(
      Date.now() - 7 * 24 * 60 * 60 * 1000,
    ).toISOString();
    expect(getRelativeTime(oneWeekAgo)).toBe("vor 1 Woche");
  });

  it("returns 'vor X Wochen' for multiple weeks", () => {
    const twoWeeksAgo = new Date(
      Date.now() - 14 * 24 * 60 * 60 * 1000,
    ).toISOString();
    expect(getRelativeTime(twoWeeksAgo)).toBe("vor 2 Wochen");
  });

  it("returns 'vor 1 Monat' for 1 month ago", () => {
    // 35 days needed: 30 days = 4 weeks (< 5), so still returns "Wochen"
    // 35 days = 5 weeks (>= 5), falls through to months: floor(35/30) = 1
    const oneMonthAgo = new Date(
      Date.now() - 35 * 24 * 60 * 60 * 1000,
    ).toISOString();
    expect(getRelativeTime(oneMonthAgo)).toBe("vor 1 Monat");
  });

  it("returns 'vor X Monaten' for multiple months", () => {
    const sixMonthsAgo = new Date(
      Date.now() - 180 * 24 * 60 * 60 * 1000,
    ).toISOString();
    expect(getRelativeTime(sixMonthsAgo)).toBe("vor 6 Monaten");
  });

  it("returns 'vor 1 Jahr' for 1 year ago", () => {
    const oneYearAgo = new Date(
      Date.now() - 365 * 24 * 60 * 60 * 1000,
    ).toISOString();
    expect(getRelativeTime(oneYearAgo)).toBe("vor 1 Jahr");
  });

  it("returns 'vor X Jahren' for multiple years", () => {
    const twoYearsAgo = new Date(
      Date.now() - 2 * 365 * 24 * 60 * 60 * 1000,
    ).toISOString();
    expect(getRelativeTime(twoYearsAgo)).toBe("vor 2 Jahren");
  });

  it("handles edge case at exactly 60 minutes", () => {
    const sixtyMinutesAgo = new Date(Date.now() - 60 * 60 * 1000).toISOString();
    expect(getRelativeTime(sixtyMinutesAgo)).toBe("vor 1 Stunde");
  });

  it("handles edge case at exactly 24 hours", () => {
    const twentyFourHoursAgo = new Date(
      Date.now() - 24 * 60 * 60 * 1000,
    ).toISOString();
    expect(getRelativeTime(twentyFourHoursAgo)).toBe("vor 1 Tag");
  });

  it("handles edge case at exactly 7 days", () => {
    const sevenDaysAgo = new Date(
      Date.now() - 7 * 24 * 60 * 60 * 1000,
    ).toISOString();
    expect(getRelativeTime(sevenDaysAgo)).toBe("vor 1 Woche");
  });
});

describe("getInitial", () => {
  it("returns uppercase first character of non-empty string", () => {
    expect(getInitial("Alice")).toBe("A");
  });

  it("returns uppercase letter for lowercase input", () => {
    expect(getInitial("john")).toBe("J");
  });

  it("returns '?' for empty string", () => {
    expect(getInitial("")).toBe("?");
  });

  it("handles single character string", () => {
    expect(getInitial("x")).toBe("X");
  });

  it("handles string with leading space", () => {
    expect(getInitial(" Bob")).toBe(" "); // First character is space
  });

  it("handles special characters", () => {
    expect(getInitial("@user")).toBe("@");
  });

  it("handles Unicode characters", () => {
    expect(getInitial("Übermensch")).toBe("Ü");
  });
});

describe("getInitials", () => {
  it("returns first and last letter for two-word name", () => {
    expect(getInitials("John Doe")).toBe("JD");
  });

  it("returns uppercased initials", () => {
    expect(getInitials("alice smith")).toBe("AS");
  });

  it("returns '?' for empty string", () => {
    expect(getInitials("")).toBe("?");
  });

  it("returns single uppercase letter for single word", () => {
    expect(getInitials("Alice")).toBe("A");
  });

  it("returns first and last letter for three-word name", () => {
    expect(getInitials("John Paul Jones")).toBe("JJ");
  });

  it("returns first and last letter for multiple spaces", () => {
    expect(getInitials("Mary   Jane   Watson")).toBe("MW");
  });

  it("filters out empty parts from multiple spaces", () => {
    expect(getInitials("  Alice   Bob  ")).toBe("AB");
  });

  it("returns '?' for string with only spaces", () => {
    expect(getInitials("   ")).toBe("?");
  });

  it("handles hyphenated names (treated as single word)", () => {
    expect(getInitials("Mary-Jane Watson")).toBe("MW");
  });

  it("handles single character names", () => {
    expect(getInitials("A B")).toBe("AB");
  });

  it("returns first initial only for single letter name", () => {
    expect(getInitials("x")).toBe("X");
  });

  it("handles Unicode characters", () => {
    expect(getInitials("Ödön von Horváth")).toBe("ÖH");
  });

  it("handles names with numbers", () => {
    expect(getInitials("Agent 007")).toBe("A0");
  });
});
