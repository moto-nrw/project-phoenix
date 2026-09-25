import { describe, expect, it } from "vitest";

import {
  formatChildCount,
  formatPresentAgainstLimit,
  isOverbooked,
  overbookedHintFor,
  overbookedHint,
} from "./activity-occupancy";

describe("isOverbooked", () => {
  it("is false below the limit", () => {
    expect(isOverbooked(44, 45)).toBe(false);
  });

  it("is false at the limit: full, not overbooked", () => {
    expect(isOverbooked(45, 45)).toBe(false);
  });

  it("is true above the limit", () => {
    expect(isOverbooked(46, 45)).toBe(true);
    expect(isOverbooked(66, 45)).toBe(true);
  });

  it("is false without a limit", () => {
    expect(isOverbooked(66, null)).toBe(false);
    expect(isOverbooked(66, undefined)).toBe(false);
  });
});

describe("formatChildCount", () => {
  it("shows count and limit when the activity has a limit", () => {
    expect(formatChildCount(66, 45)).toBe("66 / 45 Kinder");
    expect(formatChildCount(1, 45)).toBe("1 / 45 Kinder");
  });

  it("shows only the count without a limit", () => {
    expect(formatChildCount(66, null)).toBe("66 Kinder");
    expect(formatChildCount(1, undefined)).toBe("1 Kind");
  });
});

describe("formatPresentAgainstLimit", () => {
  it("shows present children against the limit", () => {
    expect(formatPresentAgainstLimit(66, 45)).toBe("66 / 45 anwesend");
  });

  it("shows only the present children without a limit", () => {
    expect(formatPresentAgainstLimit(66, null)).toBe("66 anwesend");
  });
});

describe("overbookedHint", () => {
  it("names the limit and when the tablet accepts children again", () => {
    expect(overbookedHint(45, true)).toBe(
      "Mehr Kinder als erlaubt (höchstens 45). Am Tablet kann sich jetzt kein Kind anmelden. Das geht wieder unter 45 Kindern oder mit höherer Grenze.",
    );
  });

  it("leaves the tablet out for schools without NFC", () => {
    expect(overbookedHint(45, false)).toBe(
      "Mehr Kinder als erlaubt (höchstens 45).",
    );
  });
});

describe("overbookedHintFor", () => {
  it("gives the hint only above the limit", () => {
    expect(overbookedHintFor({ count: 46, limit: 45 }, false)).toBe(
      "Mehr Kinder als erlaubt (höchstens 45).",
    );
    expect(overbookedHintFor({ count: 45, limit: 45 }, false)).toBeNull();
    expect(overbookedHintFor({ count: 44, limit: 45 }, false)).toBeNull();
    expect(overbookedHintFor({ count: 66, limit: null }, false)).toBeNull();
  });
});
