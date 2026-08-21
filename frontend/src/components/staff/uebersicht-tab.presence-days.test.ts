import { describe, expect, it } from "vitest";

import type { StaffHistorySession } from "~/lib/staff-api";

import { countSessionDaysInRange } from "./uebersicht-tab";

const from = new Date(2026, 6, 20);
const to = new Date(2026, 6, 24);

describe("UebersichtTab Anwesenheitstage", () => {
  it("zählt einen Tag, dessen Block komplett aus Pause besteht", () => {
    // Der Block hat null Nettominuten, die Person war aber da. Ohne den
    // Tageseintrag verschwände der Tag aus "Anwesend" (#2402).
    const session: StaffHistorySession = {
      date: "2026-07-20",
      status: "present",
      net_minutes: 0,
      check_in_time: "2026-07-20T06:00:00.000Z", // 08:00 CEST
      check_out_time: "2026-07-20T10:00:00.000Z", // 12:00 CEST
      break_minutes: 240,
      breaks: [
        {
          started_at: "2026-07-20T06:00:00.000Z",
          ended_at: "2026-07-20T10:00:00.000Z",
        },
      ],
    };

    expect(countSessionDaysInRange([session], from, to)).toEqual({
      present: 1,
      homeOffice: 0,
    });
  });

  it("zählt einen frisch eingestempelten Homeoffice-Block als Homeoffice-Tag", () => {
    const now = new Date();
    const todayKey = [
      now.getFullYear(),
      String(now.getMonth() + 1).padStart(2, "0"),
      String(now.getDate()).padStart(2, "0"),
    ].join("-");
    const session: StaffHistorySession = {
      date: todayKey,
      status: "home_office",
      net_minutes: 0,
      check_in_time: new Date(now.getTime() - 20_000).toISOString(),
      check_out_time: null,
      break_minutes: 0,
    };
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());

    expect(countSessionDaysInRange([session], today, today)).toEqual({
      present: 0,
      homeOffice: 1,
    });
  });

  it("entscheidet bei gemischten Blöcken weiterhin über die Minuten", () => {
    const homeOffice: StaffHistorySession = {
      date: "2026-07-22",
      status: "home_office",
      net_minutes: 60,
      check_in_time: "2026-07-22T06:00:00.000Z",
      check_out_time: "2026-07-22T07:00:00.000Z",
      break_minutes: 0,
    };
    const onSite: StaffHistorySession = {
      date: "2026-07-22",
      status: "present",
      net_minutes: 240,
      check_in_time: "2026-07-22T08:00:00.000Z",
      check_out_time: "2026-07-22T12:00:00.000Z",
      break_minutes: 0,
    };

    expect(countSessionDaysInRange([homeOffice, onSite], from, to)).toEqual({
      present: 1,
      homeOffice: 0,
    });
  });
});
