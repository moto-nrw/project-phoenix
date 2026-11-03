import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ useSWRAuth: vi.fn() }));

vi.mock("~/lib/swr", () => mocks);

import { fetchHolidayGuestBadges } from "~/lib/holiday-host-api";
import {
  type HolidayGuestCandidate,
  useHolidayGuestLabels,
} from "./use-holiday-guest-labels";

const ben = { id: "5", school_class: "Feriengast" };
const ida = { id: 6, school_class: "Feriengast" };
const lia = { id: "7", school_class: "2a" };

describe("useHolidayGuestLabels (#5630)", () => {
  beforeEach(() => {
    mocks.useSWRAuth.mockReset().mockReturnValue({ data: undefined });
  });

  it("asks for the hints only when the list holds a holiday guest", () => {
    renderHook(() => useHolidayGuestLabels([lia]));
    expect(mocks.useSWRAuth).toHaveBeenLastCalledWith(
      null,
      fetchHolidayGuestBadges,
      expect.anything(),
    );

    renderHook(() => useHolidayGuestLabels([lia, ben]));
    expect(mocks.useSWRAuth).toHaveBeenLastCalledWith(
      "holiday-guest-badges:5",
      fetchHolidayGuestBadges,
      expect.anything(),
    );
  });

  it("asks again when a live refresh brings another holiday guest (review #5642)", () => {
    const { rerender } = renderHook(
      ({ students }) => useHolidayGuestLabels(students),
      {
        initialProps: {
          students: [ben, lia] as readonly HolidayGuestCandidate[],
        },
      },
    );
    const before = mocks.useSWRAuth.mock.lastCall?.[0];

    rerender({ students: [lia, ida, ben] });

    expect(mocks.useSWRAuth.mock.lastCall?.[0]).not.toBe(before);
    expect(mocks.useSWRAuth.mock.lastCall?.[0]).toBe(
      "holiday-guest-badges:5,6",
    );
  });

  it("asks again on its own while the list stays open, so a renamed school arrives (review #5642)", () => {
    renderHook(() => useHolidayGuestLabels([ben]));

    expect(mocks.useSWRAuth.mock.lastCall?.[2]).toMatchObject({
      refreshInterval: 15 * 60_000,
      keepPreviousData: true,
    });
  });

  it("names the home school, says Feriengast without one and leaves own children alone", () => {
    mocks.useSWRAuth.mockReturnValue({
      data: [
        { studentId: "5", schoolName: "OGS Am Park" },
        { studentId: "6", schoolName: "" },
      ],
    });

    const { result } = renderHook(() => useHolidayGuestLabels([ben, ida, lia]));

    expect(result.current(ben)).toBe("Gast aus OGS Am Park");
    expect(result.current(ida)).toBe("Feriengast");
    expect(result.current(lia)).toBeUndefined();
  });

  it("keeps Feriengast while the hints load or when they fail", () => {
    mocks.useSWRAuth.mockReturnValue({
      data: undefined,
      error: new Error("403"),
    });

    const { result } = renderHook(() => useHolidayGuestLabels([ben]));

    expect(result.current(ben)).toBe("Feriengast");
  });
});
