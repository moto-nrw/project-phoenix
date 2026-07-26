import { renderHook } from "@testing-library/react";
import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";

vi.mock("swr");

vi.mock("~/lib/parent-api", () => ({
  listMyChildren: vi.fn(),
  getChildFeatures: vi.fn(),
}));

import useSWR, { type SWRResponse } from "swr";
import { useParentMealPlanEnabled } from "./use-parent-meal-plan-enabled";
import { getChildFeatures, listMyChildren } from "~/lib/parent-api";

const mockedUseSWR = useSWR as unknown as Mock;
const mockedListChildren = listMyChildren as unknown as Mock;
const mockedGetFeatures = getChildFeatures as unknown as Mock;

function swrResult(data: boolean | undefined): SWRResponse<boolean> {
  return {
    data,
    error: undefined,
    isLoading: false,
    isValidating: false,
    mutate: vi.fn(),
  } as unknown as SWRResponse<boolean>;
}

function child(tenantId: string, studentId: string) {
  return { tenant_id: tenantId, student_id: studentId };
}

// Pull the fetcher useSWR was invoked with so we can exercise the resolution
// logic (dedup-by-tenant + some-enabled) directly.
type Fetcher = () => Promise<boolean>;
function capturedFetcher(): Fetcher {
  const call = mockedUseSWR.mock.calls.at(-1);
  return call?.[1] as Fetcher;
}

beforeEach(() => {
  mockedUseSWR.mockReset();
  mockedListChildren.mockReset();
  mockedGetFeatures.mockReset();
  mockedUseSWR.mockReturnValue(swrResult(undefined));
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("useParentMealPlanEnabled", () => {
  it("passes a null key (no fetch) when disabled", () => {
    renderHook(() => useParentMealPlanEnabled(false));
    expect(mockedUseSWR).toHaveBeenCalled();
    // First arg is the SWR key: null in disabled mode so the fetcher never runs.
    expect(mockedUseSWR.mock.calls.at(-1)?.[0]).toBeNull();
  });

  it("passes a stable key when enabled", () => {
    renderHook(() => useParentMealPlanEnabled(true));
    expect(mockedUseSWR.mock.calls.at(-1)?.[0]).toBe(
      "parent-meal-plan-enabled",
    );
  });

  it("returns true only when SWR has resolved data === true", () => {
    mockedUseSWR.mockReturnValue(swrResult(true));
    const { result: on } = renderHook(() => useParentMealPlanEnabled(true));
    expect(on.current).toBe(true);

    mockedUseSWR.mockReturnValue(swrResult(false));
    const { result: off } = renderHook(() => useParentMealPlanEnabled(true));
    expect(off.current).toBe(false);

    mockedUseSWR.mockReturnValue(swrResult(undefined));
    const { result: loading } = renderHook(() =>
      useParentMealPlanEnabled(true),
    );
    // Still loading → hidden (false), never undefined.
    expect(loading.current).toBe(false);
  });
});

describe("fetchAnyMealPlanEnabled (via the captured SWR fetcher)", () => {
  it("resolves true when at least one linked school runs a meal plan", async () => {
    renderHook(() => useParentMealPlanEnabled(true));
    mockedListChildren.mockResolvedValue([child("1", "10"), child("2", "20")]);
    mockedGetFeatures.mockImplementation(async (studentId: string) => ({
      meal_plan_enabled: studentId === "20",
    }));

    await expect(capturedFetcher()()).resolves.toBe(true);
  });

  it("queries only one representative student per tenant", async () => {
    renderHook(() => useParentMealPlanEnabled(true));
    mockedListChildren.mockResolvedValue([
      child("1", "10"),
      child("1", "11"), // same tenant → deduped
      child("2", "20"),
    ]);
    mockedGetFeatures.mockResolvedValue({ meal_plan_enabled: false });

    await expect(capturedFetcher()()).resolves.toBe(false);
    expect(mockedGetFeatures).toHaveBeenCalledTimes(2);
    const askedStudents = mockedGetFeatures.mock.calls.map((c) => c[0]);
    expect(askedStudents).toEqual(["10", "20"]);
  });

  it("resolves false when no linked school runs a meal plan", async () => {
    renderHook(() => useParentMealPlanEnabled(true));
    mockedListChildren.mockResolvedValue([child("1", "10")]);
    mockedGetFeatures.mockResolvedValue({ meal_plan_enabled: false });

    await expect(capturedFetcher()()).resolves.toBe(false);
  });

  it("rejects (does not swallow) when a feature lookup fails", async () => {
    renderHook(() => useParentMealPlanEnabled(true));
    mockedListChildren.mockResolvedValue([child("1", "10")]);
    mockedGetFeatures.mockRejectedValue(new Error("500"));

    // A rejected fetch must propagate so SWR can retry, not cache `false`.
    await expect(capturedFetcher()()).rejects.toThrow("500");
  });
});
