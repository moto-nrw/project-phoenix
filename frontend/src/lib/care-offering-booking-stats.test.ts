import { afterEach, describe, expect, it, vi } from "vitest";
import {
  type CareOfferingBookingStats,
  fetchCareOfferingBookingStats,
  formatOfferingOccupancy,
  offeringIsFull,
} from "./care-offering-booking-stats";

const stats = (
  overrides: Partial<CareOfferingBookingStats> = {},
): CareOfferingBookingStats => ({
  offering_id: "1",
  capacity: 20,
  booked: 5,
  grade_levels: {},
  unknown_grade_count: 0,
  ...overrides,
});

function mockFetch(response: Partial<Response> & { json?: () => unknown }) {
  const spy = vi.fn().mockResolvedValue({
    ok: true,
    status: 200,
    json: () => Promise.resolve([]),
    ...response,
  });
  vi.stubGlobal("fetch", spy);
  return spy;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("fetchCareOfferingBookingStats", () => {
  it("requests the phase and keys the result by offering id", async () => {
    const spy = mockFetch({
      json: () =>
        Promise.resolve({
          status: "success",
          data: [stats({ offering_id: "7" }), stats({ offering_id: "9" })],
        }),
    });

    const result = await fetchCareOfferingBookingStats("42");

    expect(spy).toHaveBeenCalledWith(
      "/api/enrollment/care-offerings/booking-stats?phase_id=42",
      { cache: "no-store" },
    );
    expect(Object.keys(result)).toEqual(["7", "9"]);
    expect(result["7"]?.booked).toBe(5);
  });

  it("accepts a bare array as well as an envelope", async () => {
    mockFetch({ json: () => Promise.resolve([stats({ offering_id: "3" })]) });
    await expect(fetchCareOfferingBookingStats("1")).resolves.toHaveProperty(
      "3",
    );
  });

  it("tolerates an envelope without data", async () => {
    mockFetch({ json: () => Promise.resolve({ status: "success" }) });
    await expect(fetchCareOfferingBookingStats("1")).resolves.toEqual({});
  });

  it("encodes the phase id", async () => {
    const spy = mockFetch({ json: () => Promise.resolve([]) });
    await fetchCareOfferingBookingStats("a b");
    expect(spy).toHaveBeenCalledWith(
      "/api/enrollment/care-offerings/booking-stats?phase_id=a%20b",
      { cache: "no-store" },
    );
  });

  it("throws a readable error on a failed response", async () => {
    mockFetch({
      ok: false,
      status: 500,
      text: () => Promise.resolve(JSON.stringify({ error: "boom" })),
    });
    await expect(fetchCareOfferingBookingStats("1")).rejects.toThrow(
      /Belegung konnte nicht geladen werden|boom/,
    );
  });
});

describe("formatOfferingOccupancy", () => {
  it("renders nothing without stats", () => {
    expect(formatOfferingOccupancy(null)).toBeNull();
    expect(formatOfferingOccupancy(undefined)).toBeNull();
  });

  it("reports occupancy against a capacity", () => {
    expect(formatOfferingOccupancy(stats({ booked: 18, capacity: 20 }))).toBe(
      "18 von 20 Plätzen belegt",
    );
  });

  it("calls out a full offering before the save fails", () => {
    expect(formatOfferingOccupancy(stats({ booked: 20, capacity: 20 }))).toBe(
      "Ausgebucht (20 von 20 Plätzen belegt)",
    );
    expect(formatOfferingOccupancy(stats({ booked: 21, capacity: 20 }))).toBe(
      "Ausgebucht (21 von 20 Plätzen belegt)",
    );
  });

  it("handles unlimited offerings", () => {
    expect(formatOfferingOccupancy(stats({ capacity: null, booked: 0 }))).toBe(
      "Unbegrenzte Plätze",
    );
    expect(formatOfferingOccupancy(stats({ capacity: null, booked: 7 }))).toBe(
      "7 gebucht · unbegrenzte Plätze",
    );
  });
});

describe("offeringIsFull", () => {
  it("is false without stats or without a capacity limit", () => {
    expect(offeringIsFull(null)).toBe(false);
    expect(offeringIsFull(undefined)).toBe(false);
    expect(offeringIsFull(stats({ capacity: null, booked: 99 }))).toBe(false);
  });

  it("is true once bookings reach the capacity", () => {
    expect(offeringIsFull(stats({ booked: 19, capacity: 20 }))).toBe(false);
    expect(offeringIsFull(stats({ booked: 20, capacity: 20 }))).toBe(true);
  });
});
