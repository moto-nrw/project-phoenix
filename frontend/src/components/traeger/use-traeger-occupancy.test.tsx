import { act, renderHook, waitFor } from "@testing-library/react";
import type { Session } from "next-auth";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { resetSWRScope } from "~/lib/swr/scope";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SWRConfig, useSWRConfig } from "~/lib/swr/testing";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { useTraegerOccupancy } from "./use-traeger-occupancy";
import { setTestClock } from "~/test/clock";

// The occupancy hook goes through useConfirmedResource like the other Träger
// figure hooks (#4676, use-traeger-refusal-wiring.test.tsx). This checks its
// wiring with the sequence the review found: 403, then 503, then 200.

vi.unmock("swr");
const { fetchers, sessionState } = vi.hoisted(() => ({
  fetchers: { occupancy: vi.fn() },
  sessionState: { data: null as Session | null },
}));
vi.mock("~/lib/traeger-occupancy-api", () => ({
  fetchTraegerOccupancy: fetchers.occupancy,
}));
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: sessionState.data,
    status: sessionState.data ? "authenticated" : "unauthenticated",
  }),
}));

function wrapper({ children }: Readonly<{ children: React.ReactNode }>) {
  const cache = new Map();
  return (
    <SWRConfig value={{ provider: () => cache, dedupingInterval: 0 }}>
      <ToastProvider>
        <SWRScopeBoundary />
        {children}
      </ToastProvider>
    </SWRConfig>
  );
}

interface Case {
  readonly name: string;
  readonly fetcher: ReturnType<typeof vi.fn>;
  readonly key: string;
  readonly read: () => { data: unknown; reload: () => Promise<unknown> };
}

/** The hook plus the cache writer another view of the same key has. */
function useWithWriter(read: Case["read"], key: string) {
  const { mutate } = useSWRConfig();
  return {
    ...read(),
    writeBack: (data: unknown) =>
      mutate(`traeger:7:A:${key}`, data, { revalidate: false }),
  };
}

const CASES: readonly Case[] = [
  {
    name: "useTraegerOccupancy",
    fetcher: fetchers.occupancy,
    key: traegerKeys.occupancy(),
    read: () => {
      const { occupancy, reload } = useTraegerOccupancy();
      return { data: occupancy, reload: async () => reload() };
    },
  },
];

beforeEach(() => {
  // A refusal holds per key for the whole tab; start every test clean.
  resetSWRScope({ revalidate: false });
  sessionState.data = {
    user: { id: "A", orgId: 7, scope: "org" },
    expires: "2099-01-01",
  } as Session;
});

it.each(["2026-01-15T23:58:30+01:00", "2026-07-15T23:58:30+02:00"])(
  "refreshes the mounted view at Berlin midnight from %s without focus or reconnect",
  async (instant) => {
    vi.useFakeTimers();
    setTestClock(instant);
    fetchers.occupancy
      .mockReset()
      .mockResolvedValueOnce({ figures: "yesterday" })
      .mockResolvedValue({ figures: "today" });
    const view = renderHook(() => useTraegerOccupancy(), { wrapper });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(view.result.current.occupancy).toEqual({ figures: "yesterday" });
    expect(fetchers.occupancy).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(fetchers.occupancy).toHaveBeenCalledTimes(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });
    expect(fetchers.occupancy).toHaveBeenCalledTimes(2);
    expect(view.result.current.occupancy).toEqual({ figures: "today" });
    view.unmount();
  },
);

describe.each(CASES)("$name after a refusal", ({ fetcher, key, read }) => {
  it("shows nothing after 403 then 503, and data again only after a 200", async () => {
    const confirmed = { figures: "confirmed" };
    fetcher.mockReset().mockResolvedValue(confirmed);
    const view = renderHook(() => useWithWriter(read, key), { wrapper });
    await waitFor(() =>
      expect(view.result.current.data).toEqual({ figures: "confirmed" }),
    );

    fetcher.mockRejectedValue(new ApiError("forbidden", 403));
    await act(async () => {
      await view.result.current.reload();
    });
    await waitFor(() => expect(view.result.current.data).toBeNull());

    // Write the original, stamped answer back, not a new unstamped object
    // that the generation lock could independently discard.
    await act(async () => {
      await view.result.current.writeBack(confirmed);
    });
    fetcher.mockRejectedValue(new ApiError("unavailable", 503));
    await act(async () => {
      await view.result.current.reload();
    });
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(3));
    expect(view.result.current.data).toBeNull();

    fetcher.mockResolvedValue({ figures: "again" });
    await act(async () => {
      await view.result.current.reload();
    });
    await waitFor(() =>
      expect(view.result.current.data).toEqual({ figures: "again" }),
    );
  });
});
