import { renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useHolidayGuestGroupHint } from "./use-holiday-guest-group-hint";

vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    status: "authenticated",
    data: { user: { id: "1", token: "test-token" } },
  }),
}));

const { hintsMock } = vi.hoisted(() => ({ hintsMock: vi.fn() }));

vi.mock("~/lib/holiday-host-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/holiday-host-api")>()),
  fetchHolidayGuestGroupHints: hintsMock,
}));

function wrapper({ children }: { readonly children: ReactNode }) {
  return (
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      {children}
    </SWRConfig>
  );
}

const january = {
  groupId: "7",
  weekId: "51",
  startsOn: "2027-01-04",
  endsOn: "2027-01-08",
  guests: 3,
};

describe("useHolidayGuestGroupHint (#6680)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useTenantSlugSafe).mockReturnValue("test-tenant");
  });

  it("returns the hint of the group picked as holiday group, the next week first", async () => {
    hintsMock.mockResolvedValue([
      january,
      { ...january, weekId: "52", startsOn: "2027-01-11", guests: 5 },
    ]);
    const { result } = renderHook(() => useHolidayGuestGroupHint("7"), {
      wrapper,
    });

    await waitFor(() => expect(result.current).toEqual(january));
  });

  it("leaves an ordinary group without a hint", async () => {
    hintsMock.mockResolvedValue([january]);
    const { result } = renderHook(() => useHolidayGuestGroupHint("8"), {
      wrapper,
    });

    await waitFor(() => expect(hintsMock).toHaveBeenCalledTimes(1));
    expect(result.current).toBeUndefined();
  });

  it("stays silent when the hints cannot be loaded", async () => {
    hintsMock.mockRejectedValue(new Error("forbidden"));
    const { result } = renderHook(() => useHolidayGuestGroupHint("7"), {
      wrapper,
    });

    await waitFor(() => expect(hintsMock).toHaveBeenCalledTimes(1));
    expect(result.current).toBeUndefined();
  });
});
