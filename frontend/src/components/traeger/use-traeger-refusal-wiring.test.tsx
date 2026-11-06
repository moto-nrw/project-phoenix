import { act, renderHook, waitFor } from "@testing-library/react";
import type { Session } from "next-auth";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SWRConfig, useSWRConfig } from "~/lib/swr/testing";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { useTraegerKeyDateCounts } from "./use-traeger-key-date-counts";
import { useTraegerSchool } from "./use-traeger-school";
import { useTraegerSchoolsAlerts } from "./use-traeger-schools-alerts";

// Every Träger hook that reads figures goes through useConfirmedResource
// (#4676). This checks the wiring of each one with the sequence the review
// found: 403, then 503, then 200.

vi.unmock("swr");
const { fetchers, sessionState } = vi.hoisted(() => ({
  fetchers: {
    alerts: vi.fn(),
    counts: vi.fn(),
    school: vi.fn(),
  },
  sessionState: { data: null as Session | null },
}));
vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerSchoolsAlerts: fetchers.alerts,
  fetchTraegerKeyDateCounts: fetchers.counts,
  fetchTraegerSchool: fetchers.school,
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
    name: "useTraegerSchoolsAlerts",
    fetcher: fetchers.alerts,
    key: traegerKeys.schoolsAlerts(),
    read: () => {
      const { alerts, reload } = useTraegerSchoolsAlerts();
      return { data: alerts, reload: async () => reload() };
    },
  },
  {
    name: "useTraegerKeyDateCounts",
    fetcher: fetchers.counts,
    key: traegerKeys.keyDateCounts(),
    read: () => {
      const { counts, reload } = useTraegerKeyDateCounts();
      return { data: counts, reload: async () => reload() };
    },
  },
  {
    name: "useTraegerSchool",
    fetcher: fetchers.school,
    key: traegerKeys.school("2"),
    read: () => {
      const { school, reload } = useTraegerSchool("2");
      return { data: school, reload: async () => reload() };
    },
  },
];

beforeEach(() => {
  sessionState.data = {
    user: { id: "A", orgId: 7, scope: "org" },
    expires: "2099-01-01",
  } as Session;
});

describe.each(CASES)("$name after a refusal", ({ fetcher, key, read }) => {
  it("shows nothing after 403 then 503, and data again only after a 200", async () => {
    fetcher.mockReset().mockResolvedValue({ figures: "confirmed" });
    const view = renderHook(() => useWithWriter(read, key), { wrapper });
    await waitFor(() =>
      expect(view.result.current.data).toEqual({ figures: "confirmed" }),
    );

    fetcher.mockRejectedValue(new ApiError("forbidden", 403));
    await act(async () => {
      await view.result.current.reload();
    });
    await waitFor(() => expect(view.result.current.data).toBeNull());

    // Another view of the key writes the refused answer back.
    await act(async () => {
      await view.result.current.writeBack({ figures: "confirmed" });
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
