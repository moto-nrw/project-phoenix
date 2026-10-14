import { describe, expect, it } from "vitest";

import { waitExpired, type NetworkState } from "./network-activity";

const LIMITS = { idleMs: 30_000, maxMs: 120_000 };
const IDLE: NetworkState = { inFlight: 0, lastActivityAt: -5_000 };

describe("waitExpired", () => {
  it("gibt eine Seite, die nichts lädt, nach der Ruhezeit auf wie ein festes Limit", () => {
    expect(waitExpired(29_999, 0, IDLE, LIMITS)).toBe(false);
    expect(waitExpired(30_000, 0, IDLE, LIMITS)).toBe(true);
  });

  it("wartet weiter, solange Anfragen an den Stack offen sind", () => {
    // Die Kinderseite im CI: nach 31 s laden noch Datenabfragen.
    const loading: NetworkState = { inFlight: 5, lastActivityAt: 2_000 };
    expect(waitExpired(31_000, 0, loading, LIMITS)).toBe(false);
    expect(waitExpired(90_000, 0, loading, LIMITS)).toBe(false);
  });

  it("zählt die Ruhezeit ab der letzten Antwort", () => {
    const answered: NetworkState = { inFlight: 0, lastActivityAt: 40_000 };
    expect(waitExpired(69_999, 0, answered, LIMITS)).toBe(false);
    expect(waitExpired(70_000, 0, answered, LIMITS)).toBe(true);
  });

  it("gibt an der Obergrenze auf, auch wenn eine Anfrage nie endet", () => {
    const hanging: NetworkState = { inFlight: 1, lastActivityAt: 0 };
    expect(waitExpired(119_999, 0, hanging, LIMITS)).toBe(false);
    expect(waitExpired(120_000, 0, hanging, LIMITS)).toBe(true);
  });
});
