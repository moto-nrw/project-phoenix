import { describe, it, expect } from "vitest";
import {
  allowedDepartureModesEqual,
  allowedDepartureModesFingerprint,
  allowedDepartureToBusDays,
  allowedDepartureToDepartureDays,
  allowedDepartureToPickupDays,
  busDaysHaveAny,
  departureDaysFromLegacy,
  departureMatrixRows,
  formatAllowedDepartureDays,
  hasAnyDepartureArrangement,
  normalizeAllowedDepartureModes,
  normalizeBusDays,
  normalizePickupDays,
  pickupDaysHaveAny,
  type AllowedDepartureModes,
} from "~/lib/departure-plan";

describe("bus day helpers", () => {
  it("normalizes only enabled known weekdays", () => {
    const result = normalizeBusDays({
      mon: true,
      tue: false,
      wed: true,
      sat: true,
    } as Parameters<typeof normalizeBusDays>[0] & { sat: boolean });

    expect(result).toEqual({ mon: true, wed: true });
  });

  it("normalizes null and undefined to an empty object", () => {
    expect(normalizeBusDays(null)).toEqual({});
    expect(normalizeBusDays(undefined)).toEqual({});
  });

  it("detects any enabled known weekday", () => {
    expect(busDaysHaveAny({ fri: true })).toBe(true);
    expect(busDaysHaveAny({ mon: false, thu: false })).toBe(false);
    expect(busDaysHaveAny(null)).toBe(false);
    expect(busDaysHaveAny(undefined)).toBe(false);
  });
});

describe("pickup day helpers", () => {
  it("normalizes only enabled known weekdays", () => {
    const result = normalizePickupDays({
      mon: true,
      tue: false,
      wed: true,
      sat: true,
    } as Parameters<typeof normalizePickupDays>[0] & { sat: boolean });

    expect(result).toEqual({ mon: true, wed: true });
  });

  it("detects any enabled known weekday", () => {
    expect(pickupDaysHaveAny({ fri: true })).toBe(true);
    expect(pickupDaysHaveAny({ mon: false, thu: false })).toBe(false);
    expect(pickupDaysHaveAny(null)).toBe(false);
  });
});

describe("allowed departure mode helpers", () => {
  it("supports the accompanied mode end to end (#1694)", () => {
    const normalized = normalizeAllowedDepartureModes({
      mon: ["accompanied"],
      wed: ["accompanied", "pickup"],
    });
    expect(normalized).toEqual({
      mon: ["accompanied"],
      wed: ["pickup", "accompanied"],
    });
    // Exclusive derivation: pickup outranks accompanied; an accompanied-only
    // day stays accompanied.
    expect(allowedDepartureToDepartureDays(normalized)).toEqual({
      mon: "accompanied",
      wed: "pickup",
    });
    // Accompanied must never leak into the legacy bus/pickup mirrors.
    expect(allowedDepartureToBusDays({ mon: ["accompanied"] })).toEqual({});
    expect(allowedDepartureToPickupDays({ mon: ["accompanied"] })).toEqual({});
  });

  it("lets pickup win over the bus on a day that allows both (#1610)", () => {
    expect(allowedDepartureToDepartureDays({ tue: ["bus", "pickup"] })).toEqual(
      { tue: "pickup" },
    );
    expect(
      departureDaysFromLegacy({ tue: true, wed: true }, { tue: true }),
    ).toEqual({ tue: "pickup", wed: "bus" });
  });

  it("treats shape noise as the same plan", () => {
    const a: AllowedDepartureModes = { mon: ["pickup", "bus"], tue: [] };
    const b: AllowedDepartureModes = { mon: ["bus", "pickup", "bus"] };
    expect(allowedDepartureModesEqual(a, b)).toBe(true);
    expect(allowedDepartureModesFingerprint(a)).toBe(
      allowedDepartureModesFingerprint(b),
    );
    expect(
      allowedDepartureModesEqual({ mon: ["bus"] }, { mon: ["pickup"] }),
    ).toBe(false);
  });

  it("summarizes only the weekdays as a compact day list", () => {
    // The badge variant drops the per-day modes so it stays single-line even
    // when every day carries both bus and pickup.
    expect(
      formatAllowedDepartureDays({
        mon: ["bus", "pickup"],
        tue: ["bus", "pickup"],
        wed: ["bus", "pickup"],
        thu: ["bus", "pickup"],
        fri: ["bus", "pickup"],
      }),
    ).toBe("Mo, Di, Mi, Do, Fr");
    expect(formatAllowedDepartureDays({ mon: ["bus"], fri: ["pickup"] })).toBe(
      "Mo, Fr",
    );
  });

  it("formats an empty day summary as going alone", () => {
    expect(formatAllowedDepartureDays({})).toBe("Geht immer alleine");
    expect(formatAllowedDepartureDays(null)).toBe("Geht immer alleine");
  });

  it("folds a day without an arrangement to going alone in the matrix", () => {
    const rows = departureMatrixRows({ wed: ["pickup", "bus"] });
    expect(rows.map((row) => [row.short, row.modes])).toEqual([
      ["Mo", ["alone"]],
      ["Di", ["alone"]],
      ["Mi", ["bus", "pickup"]],
      ["Do", ["alone"]],
      ["Fr", ["alone"]],
    ]);
    expect(hasAnyDepartureArrangement({ mon: ["alone"] })).toBe(false);
    expect(hasAnyDepartureArrangement({ mon: ["accompanied"] })).toBe(true);
  });
});
