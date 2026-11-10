import { afterEach, describe, expect, it, vi } from "vitest";

import {
  berlinClockFromISO,
  formatBerlinDate,
  formatDate,
  formatTime,
} from "./date-helpers";
import { formatTime as formatAttendanceTime } from "./attendance-history-helpers";
import { formatTime as formatTrackedTime } from "./time-tracking-helpers";

// Runs in `school-date-dom`: only forked processes apply a runtime TZ change.
// Every case first checks the browser's own reading of the instant, so a
// zone change that did not take effect cannot hide a wrong result (#6471).
afterEach(() => vi.unstubAllEnvs());

const ZONES = ["Europe/Berlin", "Pacific/Honolulu", "Asia/Tokyo"] as const;

/** Browser-local calendar day and clock of an instant, "YYYY-MM-DD HH:MM". */
function localReading(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

describe("formatDate and formatTime show school time (Europe/Berlin)", () => {
  // Each instant with its reading in Berlin, Honolulu (UTC−10) and Tokyo
  // (UTC+9). The Berlin reading is what every browser must show.
  const cases = [
    {
      name: "summer time, Berlin already on the next day",
      instant: "2026-10-20T23:30:00Z",
      local: {
        "Europe/Berlin": "2026-10-21 01:30",
        "Pacific/Honolulu": "2026-10-20 13:30",
        "Asia/Tokyo": "2026-10-21 08:30",
      },
      date: "21.10.2026",
      weekday: "Mittwoch, 21. Oktober 2026",
      time: "01:30",
    },
    {
      name: "summer time, Tokyo already on the next day",
      instant: "2026-07-20T15:30:00Z",
      local: {
        "Europe/Berlin": "2026-07-20 17:30",
        "Pacific/Honolulu": "2026-07-20 05:30",
        "Asia/Tokyo": "2026-07-21 00:30",
      },
      date: "20.07.2026",
      weekday: "Montag, 20. Juli 2026",
      time: "17:30",
    },
    {
      name: "winter time, Berlin already on the next day",
      instant: "2026-01-20T23:30:00Z",
      local: {
        "Europe/Berlin": "2026-01-21 00:30",
        "Pacific/Honolulu": "2026-01-20 13:30",
        "Asia/Tokyo": "2026-01-21 08:30",
      },
      date: "21.01.2026",
      weekday: "Mittwoch, 21. Januar 2026",
      time: "00:30",
    },
    {
      name: "winter time, Tokyo already on the next day",
      instant: "2026-01-20T15:30:00Z",
      local: {
        "Europe/Berlin": "2026-01-20 16:30",
        "Pacific/Honolulu": "2026-01-20 05:30",
        "Asia/Tokyo": "2026-01-21 00:30",
      },
      date: "20.01.2026",
      weekday: "Dienstag, 20. Januar 2026",
      time: "16:30",
    },
    {
      name: "autumn clock change, first 02:30 (CEST)",
      instant: "2026-10-25T00:30:00Z",
      local: {
        "Europe/Berlin": "2026-10-25 02:30",
        "Pacific/Honolulu": "2026-10-24 14:30",
        "Asia/Tokyo": "2026-10-25 09:30",
      },
      date: "25.10.2026",
      weekday: "Sonntag, 25. Oktober 2026",
      time: "02:30",
    },
    {
      name: "autumn clock change, second 02:30 (CET)",
      instant: "2026-10-25T01:30:00Z",
      local: {
        "Europe/Berlin": "2026-10-25 02:30",
        "Pacific/Honolulu": "2026-10-24 15:30",
        "Asia/Tokyo": "2026-10-25 10:30",
      },
      date: "25.10.2026",
      weekday: "Sonntag, 25. Oktober 2026",
      time: "02:30",
    },
    {
      name: "spring clock change, 01:30 UTC is 03:30 CEST",
      instant: "2026-03-29T01:30:00Z",
      local: {
        "Europe/Berlin": "2026-03-29 03:30",
        "Pacific/Honolulu": "2026-03-28 15:30",
        "Asia/Tokyo": "2026-03-29 10:30",
      },
      date: "29.03.2026",
      weekday: "Sonntag, 29. März 2026",
      time: "03:30",
    },
  ] as const;

  for (const zone of ZONES) {
    describe(`in a browser in ${zone}`, () => {
      it.each(cases)("$name", (c) => {
        vi.stubEnv("TZ", zone);
        expect(localReading(c.instant)).toBe(c.local[zone]);

        expect(formatDate(c.instant)).toBe(c.date);
        expect(formatDate(c.instant, true)).toBe(c.weekday);
        expect(formatTime(c.instant)).toBe(c.time);
      });

      it.each(cases)(
        "matches time tracking and the Berlin helpers: $name",
        (c) => {
          vi.stubEnv("TZ", zone);
          expect(localReading(c.instant)).toBe(c.local[zone]);

          expect(formatTime(c.instant)).toBe(formatTrackedTime(c.instant));
          expect(formatAttendanceTime(new Date(c.instant))).toBe(c.time);
          expect(formatTime(c.instant)).toBe(berlinClockFromISO(c.instant));
          expect(formatDate(c.instant)).toBe(formatBerlinDate(c.instant));
        },
      );

      it("keeps a calendar date (YYYY-MM-DD) unchanged", () => {
        vi.stubEnv("TZ", zone);
        // Local midnight of the day stays on that day in every zone.
        expect(localReading("2026-10-21T00:00:00")).toBe("2026-10-21 00:00");

        expect(formatDate("2026-10-21")).toBe("21.10.2026");
        expect(formatDate("2026-10-21", true)).toBe(
          "Mittwoch, 21. Oktober 2026",
        );
        expect(formatDate("2026-03-29")).toBe("29.03.2026");
        expect(formatDate("2026-10-25", true)).toBe(
          "Sonntag, 25. Oktober 2026",
        );
        expect(formatDate("2026-10-21", false, "en")).toBe("21/10/2026");
      });
    });
  }

  it("reads an instant with an explicit offset as that moment", () => {
    vi.stubEnv("TZ", "Pacific/Honolulu");
    expect(localReading("2026-10-21T01:30:00+02:00")).toBe("2026-10-20 13:30");

    expect(formatDate("2026-10-21T01:30:00+02:00")).toBe("21.10.2026");
    expect(formatTime("2026-10-21T01:30:00+02:00")).toBe("01:30");
  });
});
