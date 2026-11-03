import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ useSWRAuth: vi.fn() }));

vi.mock("~/lib/swr", () => mocks);

import { fetchHolidayGuestBadges } from "~/lib/holiday-host-api";
import { useHolidayGuestLabels } from "./use-holiday-guest-labels";

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
      "holiday-guest-badges",
      fetchHolidayGuestBadges,
      expect.anything(),
    );
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
