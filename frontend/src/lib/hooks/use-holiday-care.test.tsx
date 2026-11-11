import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  useSWRAuth: vi.fn(),
  useSwrLoadError: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));
vi.mock("~/lib/swr", () => ({ useSWRAuth: mocks.useSWRAuth }));
vi.mock("~/lib/hooks/use-swr-load-error", () => ({
  useSwrLoadError: mocks.useSwrLoadError,
}));

import { usePlanningHolidayWeeks } from "./use-holiday-care";

function signedInWith(...permissions: string[]) {
  mocks.useSession.mockReturnValue({
    data: { user: { permissions } },
    status: "authenticated",
  });
}

describe("usePlanningHolidayWeeks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useSWRAuth.mockReturnValue({
      data: undefined,
      error: undefined,
      mutate: vi.fn(),
    });
    mocks.useSwrLoadError.mockReturnValue(null);
  });

  it.each([["schedules:read"], ["time_tracking:manage"], ["config:read"]])(
    "loads the hosted weeks for a person with %s",
    (permission) => {
      signedInWith(permission);
      renderHook(() => usePlanningHolidayWeeks());

      expect(mocks.useSWRAuth.mock.calls[0]?.[0]).toBe("holiday-weeks");
    },
  );

  it("loads nothing without a planning permission", () => {
    signedInWith("students:read");
    const { result } = renderHook(() => usePlanningHolidayWeeks());

    expect(mocks.useSWRAuth.mock.calls[0]?.[0]).toBeNull();
    expect(result.current.weeks).toEqual([]);
  });

  it("links to the holiday weeks only with config:read", () => {
    signedInWith("schedules:read");
    expect(
      renderHook(() => usePlanningHolidayWeeks()).result.current.canOpenList,
    ).toBe(false);
    signedInWith("config:read");
    expect(
      renderHook(() => usePlanningHolidayWeeks()).result.current.canOpenList,
    ).toBe(true);
  });

  it("stays silent on a missing permission and shows any other failure", () => {
    signedInWith("schedules:read");
    const forbidden = new ApiError("forbidden", 403);
    mocks.useSWRAuth.mockReturnValue({
      data: undefined,
      error: forbidden,
      mutate: vi.fn(),
    });
    renderHook(() => usePlanningHolidayWeeks());
    expect(mocks.useSwrLoadError.mock.calls.at(-1)?.[0]).toBeNull();

    const broken = new ApiError("server", 500);
    mocks.useSWRAuth.mockReturnValue({
      data: undefined,
      error: broken,
      mutate: vi.fn(),
    });
    renderHook(() => usePlanningHolidayWeeks());
    expect(mocks.useSwrLoadError.mock.calls.at(-1)?.slice(0, 2)).toEqual([
      broken,
      "die Ferienbetreuung",
    ]);
  });
});
