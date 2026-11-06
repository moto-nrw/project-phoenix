import { act, renderHook, waitFor } from "@testing-library/react";
import type { Session } from "next-auth";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SWRConfig, useSWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { useSWRAuth } from "~/lib/swr";
import {
  useConfirmedResource,
  useRefusalGuard,
} from "./use-traeger-confirmed-resource";

vi.unmock("swr");
const { fetchFigures, sessionState } = vi.hoisted(() => ({
  fetchFigures: vi.fn(),
  sessionState: { data: null as Session | null },
}));
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: sessionState.data,
    status: sessionState.data ? "authenticated" : "unauthenticated",
  }),
}));

const KEY = "traeger-test-figures";
const CACHE_KEY = `traeger:7:A:${KEY}`;
const CONFIRMED = { children: 127 };
const AGAIN = { children: 130 };

function setup() {
  const cache = new Map();
  function Wrapper({ children }: Readonly<{ children: React.ReactNode }>) {
    return (
      <SWRConfig value={{ provider: () => cache, dedupingInterval: 0 }}>
        <ToastProvider>
          <SWRScopeBoundary />
          {children}
        </ToastProvider>
      </SWRConfig>
    );
  }
  return { cache, wrapper: Wrapper };
}

/** The hook under test plus the cache writer another view of the key has. */
function useFigures() {
  // The shape every Träger hook uses (test files may key freely).
  const guard = useRefusalGuard(KEY);
  const response = useSWRAuth<{ children: number }>(KEY, fetchFigures, {
    ...guard.callbacks,
  });
  const resource = useConfirmedResource(response, guard, "die Zahlen");
  const { mutate } = useSWRConfig();
  return { resource, writeCache: mutate };
}

async function refetch(reload: () => void) {
  await act(async () => {
    reload();
    await Promise.resolve();
  });
}

beforeEach(() => {
  sessionState.data = {
    user: { id: "A", orgId: 7, scope: "org" },
    expires: "2099-01-01",
  } as Session;
  fetchFigures.mockReset().mockResolvedValue(CONFIRMED);
});

describe("Träger reads after a refusal", () => {
  it.each([401, 403, 404])(
    "removes the refused data from the cache itself on %i (invariant 1)",
    async (status) => {
      const options = setup();
      const view = renderHook(() => useFigures(), options);
      await waitFor(() =>
        expect(view.result.current.resource.data).toEqual(CONFIRMED),
      );
      expect(options.cache.get(CACHE_KEY)?.data).toEqual(CONFIRMED);

      fetchFigures.mockRejectedValue(new ApiError("refused", status));
      await refetch(view.result.current.resource.reload);

      await waitFor(() =>
        expect(options.cache.get(CACHE_KEY)?.data).toBeUndefined(),
      );
      expect(view.result.current.resource.data).toBeNull();
      expect(view.result.current.resource.refused).toBe(true);
    },
  );

  it("shows data again only after a successful fetch: 403, then 503, then 200 (invariant 2)", async () => {
    const options = setup();
    const view = renderHook(() => useFigures(), options);
    await waitFor(() =>
      expect(view.result.current.resource.data).toEqual(CONFIRMED),
    );

    fetchFigures.mockRejectedValue(new ApiError("forbidden", 403));
    await refetch(view.result.current.resource.reload);
    await waitFor(() =>
      expect(view.result.current.resource.refused).toBe(true),
    );

    // Another view of the key writes the old answer back into the cache.
    await act(async () => {
      await view.result.current.writeCache(CACHE_KEY, CONFIRMED, {
        revalidate: false,
      });
    });
    fetchFigures.mockRejectedValue(new ApiError("unavailable", 503));
    await refetch(view.result.current.resource.reload);
    await waitFor(() => expect(fetchFigures).toHaveBeenCalledTimes(3));

    expect(view.result.current.resource.data).toBeNull();
    expect(view.result.current.resource.refused).toBe(true);
    expect(view.result.current.resource.failure).toMatchObject({
      status: 403,
    });

    fetchFigures.mockResolvedValue(AGAIN);
    await refetch(view.result.current.resource.reload);
    await waitFor(() =>
      expect(view.result.current.resource.data).toEqual(AGAIN),
    );
    expect(view.result.current.resource.refused).toBe(false);
    expect(view.result.current.resource.failure).toBeUndefined();
  });

  it("keeps confirmed data through a transient failure without a refusal (invariant 3)", async () => {
    const options = setup();
    const view = renderHook(() => useFigures(), options);
    await waitFor(() =>
      expect(view.result.current.resource.data).toEqual(CONFIRMED),
    );

    fetchFigures.mockRejectedValue(new ApiError("unavailable", 503));
    await refetch(view.result.current.resource.reload);
    await waitFor(() =>
      expect(view.result.current.resource.failure).toMatchObject({
        status: 503,
      }),
    );

    expect(view.result.current.resource.data).toEqual(CONFIRMED);
    expect(view.result.current.resource.refused).toBe(false);
    expect(options.cache.get(CACHE_KEY)?.data).toEqual(CONFIRMED);
  });
});
