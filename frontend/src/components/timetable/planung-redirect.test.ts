import { describe, expect, it } from "vitest";

import { resolvePlanungRedirect } from "./planung-redirect";

// Fixed "today" so week-offset expectations stay deterministic
// (2026-07-13 is a Monday).
const TODAY = "2026-07-13";

function params(query: string): URLSearchParams {
  return new URLSearchParams(query);
}

describe("resolvePlanungRedirect", () => {
  // Übersetzung der alten Planungs-URLs einschließlich der Randfälle.
  const cases: Array<{
    name: string;
    query: string;
    target?: "betreuungsplan" | "dienstplan" | "vertretung";
    expected: string;
  }> = [
    // --- /planung tab dispatch ---
    {
      name: "tab=dienstplan routes to /dienstplan without params",
      query: "tab=dienstplan",
      expected: "/dienstplan",
    },
    {
      name: "missing tab defaults to /betreuungsplan",
      query: "",
      expected: "/betreuungsplan",
    },
    {
      name: "unknown tab defaults to /betreuungsplan",
      query: "tab=unsinn",
      expected: "/betreuungsplan",
    },
    {
      name: "tab=vertretung routes to /vertretung",
      query: "tab=vertretung",
      expected: "/vertretung",
    },
    // --- Betreuungsplan translation ---
    {
      name: "acceptance case: view=week&week=1&instance=42",
      query: "view=week&week=1&instance=42",
      target: "betreuungsplan",
      expected: "/betreuungsplan?d=2026-07-20&view=woche&block=42",
    },
    {
      name: "day wins over week",
      query: "day=2026-09-01&week=3",
      target: "betreuungsplan",
      expected: "/betreuungsplan?d=2026-09-01",
    },
    {
      name: "impossible day (2026-02-31) is rejected and falls through to week",
      query: "day=2026-02-31&week=1",
      target: "betreuungsplan",
      expected: "/betreuungsplan?d=2026-07-20",
    },
    {
      name: "negative week offset",
      query: "week=-2",
      target: "betreuungsplan",
      expected: "/betreuungsplan?d=2026-06-29",
    },
    {
      name: "month anchor becomes first of month",
      query: "view=month&month=2026-09",
      target: "betreuungsplan",
      expected: "/betreuungsplan?d=2026-09-01&view=monat",
    },
    {
      name: "year view maps to monat with first of year",
      query: "view=year&year=2027",
      target: "betreuungsplan",
      expected: "/betreuungsplan?d=2027-01-01&view=monat",
    },
    {
      name: "view=series maps to serien",
      query: "view=series",
      target: "betreuungsplan",
      expected: "/betreuungsplan?view=serien",
    },
    {
      name: "view=templates maps to serien",
      query: "view=templates",
      target: "betreuungsplan",
      expected: "/betreuungsplan?view=serien",
    },
    {
      name: "period dies without replacement",
      query: "period=5",
      target: "betreuungsplan",
      expected: "/betreuungsplan",
    },
    {
      name: "history dies on the betreuungsplan target",
      query: "instance=7&history=1",
      target: "betreuungsplan",
      expected: "/betreuungsplan?block=7",
    },
    {
      name: "malformed week is ignored",
      query: "week=abc",
      target: "betreuungsplan",
      expected: "/betreuungsplan",
    },
    {
      name: "view=day dies (the day lives in d)",
      query: "view=day&day=2026-08-15",
      target: "betreuungsplan",
      expected: "/betreuungsplan?d=2026-08-15",
    },
    {
      name: "unknown view dies",
      query: "view=bogus",
      target: "betreuungsplan",
      expected: "/betreuungsplan",
    },
    // --- Vertretung translation ---
    {
      name: "acceptance case: instance=42&history=1",
      query: "instance=42&history=1",
      target: "vertretung",
      expected: "/vertretung?block=42&verlauf=1",
    },
    {
      name: "vertretung: day becomes d, view dies",
      query: "day=2026-08-15&view=week",
      target: "vertretung",
      expected: "/vertretung?d=2026-08-15",
    },
    {
      name: "vertretung: week offset becomes d",
      query: "week=1",
      target: "vertretung",
      expected: "/vertretung?d=2026-07-20",
    },
    {
      name: "vertretung: month/year never anchor (day view only)",
      query: "month=2026-09&year=2026",
      target: "vertretung",
      expected: "/vertretung",
    },
    // --- Dienstplan ---
    {
      name: "dienstplan drops every legacy param",
      query: "week=2&view=week&instance=9",
      target: "dienstplan",
      expected: "/dienstplan",
    },
    // --- tab + view params combined (old /planung deep link) ---
    {
      name: "planung deep link with betreuung tab and params",
      query: "tab=betreuung&view=week&week=1&instance=42",
      expected: "/betreuungsplan?d=2026-07-20&view=woche&block=42",
    },
    // Zusätzliche Redirect-Randfälle.
    {
      name: "criterion 13: /planung?tab=betreuung&instance=123 lands on /betreuungsplan?block=123",
      query: "tab=betreuung&instance=123",
      expected: "/betreuungsplan?block=123",
    },
  ];

  it.each(cases)("$name", ({ query, target, expected }) => {
    expect(resolvePlanungRedirect(params(query), TODAY, target)).toBe(expected);
  });

  it("never emits a legacy parameter", () => {
    const legacy = [
      "week",
      "month",
      "year",
      "day",
      "period",
      "instance",
      "history",
      "tab",
    ];
    for (const { query, target } of cases) {
      const href = resolvePlanungRedirect(params(query), TODAY, target);
      const queryString = href.split("?")[1] ?? "";
      const emitted = new URLSearchParams(queryString);
      for (const key of legacy) {
        expect(emitted.has(key), `${href} must not carry ${key}`).toBe(false);
      }
      // Höchstens drei Parameter pro Bereich (Synthese 1.3).
      expect([...emitted.keys()].length).toBeLessThanOrEqual(3);
    }
  });
});

// Der `?week=N`-Offset muss auf den MONTAG der Zielwoche zeigen (docs/07 §1,
// invers zu weekOffsetForISO), nicht N*7 Tage auf den heutigen Wochentag
// addieren. Das Haupt-Fixture TODAY="2026-07-13" ist selbst ein Montag, dort
// ändert der Snap nichts; erst ein nicht-montägliches "heute" deckt den Bug ab.
describe("resolvePlanungRedirect week offset snaps to Monday", () => {
  // Mittwoch — Montag dieser Woche ist 2026-07-13.
  const WED = "2026-07-15";

  const cases: Array<{
    name: string;
    query: string;
    target: "betreuungsplan" | "dienstplan" | "vertretung";
    expected: string;
  }> = [
    {
      name: "week=1 anchors to next week's Monday, not Wednesday + 7",
      query: "week=1",
      target: "vertretung",
      expected: "/vertretung?d=2026-07-20",
    },
    {
      name: "week=0 anchors to the current week's Monday, not today",
      query: "week=0",
      target: "vertretung",
      expected: "/vertretung?d=2026-07-13",
    },
    {
      name: "negative offset anchors to a past Monday",
      query: "week=-1",
      target: "vertretung",
      expected: "/vertretung?d=2026-07-06",
    },
    {
      name: "betreuungsplan week offset snaps to Monday too",
      query: "view=week&week=1",
      target: "betreuungsplan",
      expected: "/betreuungsplan?d=2026-07-20&view=woche",
    },
  ];

  it.each(cases)("$name", ({ query, target, expected }) => {
    expect(resolvePlanungRedirect(params(query), WED, target)).toBe(expected);
  });
});
