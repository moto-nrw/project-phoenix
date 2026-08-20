import { describe, expect, it } from "vitest";

import type { StaffHistorySession } from "~/lib/staff-api";

import { indexSessionNetMinutesByBerlinDate } from "./uebersicht-tab";

describe("UebersichtTab Nachtblöcke", () => {
  it("teilt Nettozeit und Pause an der Berliner Tagesgrenze", () => {
    const session: StaffHistorySession = {
      date: "2026-07-20",
      net_minutes: 210,
      check_in_time: "2026-07-20T20:00:00.000Z", // 22:00 CEST
      check_out_time: "2026-07-21T00:00:00.000Z", // 02:00 CEST
      break_minutes: 30,
      breaks: [
        {
          started_at: "2026-07-20T22:30:00.000Z", // 00:30 CEST
          ended_at: "2026-07-20T23:00:00.000Z", // 01:00 CEST
        },
      ],
    };

    expect(indexSessionNetMinutesByBerlinDate([session])).toEqual(
      new Map([
        ["2026-07-20", 120],
        ["2026-07-21", 90],
      ]),
    );
  });
});
