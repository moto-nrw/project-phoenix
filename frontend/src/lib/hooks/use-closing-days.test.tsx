import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  useSWRAuth: vi.fn(),
  useTenantMutate: vi.fn(),
  useTenantMutateMatching: vi.fn(),
}));

vi.mock("~/lib/swr", () => mocks);

import { useInvalidateClosingDays } from "./use-closing-days";

describe("useInvalidateClosingDays", () => {
  const mutateClosingDays = vi.fn();
  const revalidateHints = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mutateClosingDays.mockResolvedValue(undefined);
    revalidateHints.mockResolvedValue([]);
    mocks.useTenantMutate.mockReturnValue(mutateClosingDays);
    mocks.useTenantMutateMatching.mockReturnValue(revalidateHints);
  });

  it("refreshes every date-specific school-vacation hint after a closing-day change", async () => {
    const { result } = renderHook(() => useInvalidateClosingDays());

    await result.current();

    expect(mocks.useTenantMutateMatching).toHaveBeenCalledWith([
      "timetable-school-vacation-hints",
    ]);
    expect(mutateClosingDays).toHaveBeenCalledWith("planning-closing-days");
    expect(revalidateHints).toHaveBeenCalledOnce();
  });
});
