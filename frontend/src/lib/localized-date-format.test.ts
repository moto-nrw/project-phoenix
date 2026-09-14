import { describe, expect, it } from "vitest";

import { SUPPORTED_LOCALES } from "~/i18n/locales";

import {
  formatCalendarDate,
  formatLocalizedDate,
  formatLocalizedDateTime,
} from "./localized-date-format";

describe("formatLocalizedDate", () => {
  it("formats a calendar date in the locale's own pattern", () => {
    expect(formatLocalizedDate("2026-09-14", "de")).toBe("14.09.2026");
    expect(formatLocalizedDate("2026-09-14", "en")).toBe("09/14/2026");
    expect(formatLocalizedDate("2026-09-14", "pl")).toBe("14.09.2026");
    expect(formatLocalizedDate("2026-09-14", "tr")).toBe("14.09.2026");
    expect(formatLocalizedDate("2026-09-14", "uk")).toBe("14.09.2026");
  });

  it("normalizes a region-tagged locale instead of falling back to German", () => {
    expect(formatLocalizedDate("2026-09-14", "en-US")).toBe("09/14/2026");
  });

  it("falls back to German for an unknown locale", () => {
    expect(formatLocalizedDate("2026-09-14", "xx")).toBe("14.09.2026");
  });

  it("returns the input unchanged when it is not a date", () => {
    expect(formatLocalizedDate("kein-datum", "pl")).toBe("kein-datum");
  });
});

describe("formatLocalizedDateTime", () => {
  it("formats every registered locale with its own formatter", () => {
    // The Turkish pattern has no comma between date and time. A formatter map
    // that silently fell back to German would print "14.09.2026, 09:00".
    expect(formatLocalizedDateTime("2026-09-14T07:00:00Z", "tr")).toBe(
      "14.09.2026 09:00",
    );
    for (const { code } of SUPPORTED_LOCALES) {
      expect(formatLocalizedDateTime("2026-09-14T07:00:00Z", code)).toBe(
        new Intl.DateTimeFormat(code, {
          day: "2-digit",
          month: "2-digit",
          year: "numeric",
          hour: "2-digit",
          minute: "2-digit",
          timeZone: "Europe/Berlin",
        }).format(new Date("2026-09-14T07:00:00Z")),
      );
    }
  });
});

describe("formatCalendarDate", () => {
  it("renders month names in the new languages", () => {
    const options = { day: "numeric", month: "long" } as const;
    expect(formatCalendarDate("2026-09-14", "pl", options)).toBe("14 września");
    expect(formatCalendarDate("2026-09-14", "tr", options)).toBe("14 Eylül");
    expect(formatCalendarDate("2026-09-14", "uk", options)).toBe("14 вересня");
  });
});
