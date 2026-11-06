import { act, renderHook, waitFor } from "@testing-library/react";
import type { Session } from "next-auth";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SWRConfig, useSWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { resetSWRScope } from "~/lib/swr/scope";
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
  // Refusals are tab-wide; the session boundary lifts them between tests.
  resetSWRScope({ revalidate: false });
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

/** One reader of the key: what a second component of the same key shows. */
function useReader() {
  const guard = useRefusalGuard(KEY);
  const response = useSWRAuth<{ children: number }>(KEY, fetchFigures, {
    ...guard.callbacks,
  });
  return useConfirmedResource(response, guard, "die Zahlen");
}

describe("a refusal per key, for every reader (invariant 4)", () => {
  it("keeps a second reader empty after the first one's 403, also when another view writes back on 503", async () => {
    const options = setup();
    const first = renderHook(() => useFigures(), options);
    await waitFor(() =>
      expect(first.result.current.resource.data).toEqual(CONFIRMED),
    );

    fetchFigures.mockRejectedValue(new ApiError("forbidden", 403));
    await refetch(first.result.current.resource.reload);
    await waitFor(() =>
      expect(first.result.current.resource.refused).toBe(true),
    );

    // Another view writes the old answer back; the server is unavailable.
    await act(async () => {
      await first.result.current.writeCache(CACHE_KEY, CONFIRMED, {
        revalidate: false,
      });
    });
    fetchFigures.mockRejectedValue(new ApiError("unavailable", 503));
    const second = renderHook(() => useReader(), options);
    await waitFor(() => expect(fetchFigures).toHaveBeenCalledTimes(3));

    expect(second.result.current.data).toBeNull();
    expect(second.result.current.refused).toBe(true);
    expect(second.result.current.failure).toMatchObject({ status: 403 });
    expect(first.result.current.resource.data).toBeNull();
  });

  it("shows data again in every reader after 403 and then 200", async () => {
    const options = setup();
    const first = renderHook(() => useFigures(), options);
    await waitFor(() =>
      expect(first.result.current.resource.data).toEqual(CONFIRMED),
    );
    fetchFigures.mockRejectedValue(new ApiError("forbidden", 403));
    await refetch(first.result.current.resource.reload);
    await waitFor(() =>
      expect(first.result.current.resource.refused).toBe(true),
    );
    const second = renderHook(() => useReader(), options);
    await waitFor(() => expect(second.result.current.refused).toBe(true));
    expect(second.result.current.data).toBeNull();

    fetchFigures.mockResolvedValue(AGAIN);
    await refetch(second.result.current.reload);

    await waitFor(() => expect(second.result.current.data).toEqual(AGAIN));
    await waitFor(() =>
      expect(first.result.current.resource.data).toEqual(AGAIN),
    );
    expect(first.result.current.resource.refused).toBe(false);
  });

  it("lifts every refusal when the session ends or the account changes", async () => {
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

    // Another account signs in; its first read fails without a refusal.
    fetchFigures.mockRejectedValue(new ApiError("unavailable", 503));
    sessionState.data = {
      user: { id: "B", orgId: 7, scope: "org" },
      expires: "2099-01-01",
    } as Session;
    view.rerender();

    await waitFor(() =>
      expect(view.result.current.resource.failure).toMatchObject({
        status: 503,
      }),
    );
    expect(view.result.current.resource.refused).toBe(false);
  });
});
