import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import {
  useSWRAuth,
  useImmutableSWR,
  useSWRWithId,
  useTenantMutate,
  useTenantMutateMatching,
} from "./hooks";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

const { mockSwrMutate, mockCache } = vi.hoisted(() => ({
  mockSwrMutate: vi.fn(),
  // Stable like SWR's own cache; empty, since the matcher contract below is
  // what these tests check.
  mockCache: new Map(),
}));

// Mock swr
vi.mock("swr", () => ({
  default: vi.fn(),
  mutate: mockSwrMutate,
  useSWRConfig: () => ({ cache: mockCache }),
}));

const { mockUseTenantSlugSafe } = vi.hoisted(() => ({
  mockUseTenantSlugSafe: vi.fn((): string | null => null),
}));

// Mock tenant context — default returns null (no tenant context).
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: mockUseTenantSlugSafe,
  useNFCEnabled: vi.fn(() => true),
}));

// Import mocked modules
import { useSession } from "next-auth/react";
// Boundary-level test: the shared wrapper may import the package directly.
import useSWR from "swr";

// Helper to create mock session data
// The browser session carries no token (#5005); the argument only decides
// whether a session exists.
const createMockSession = (signedIn?: string) => ({
  data: signedIn ? { user: { id: "user-1" }, expires: "2099-01-01" } : null,
  status: signedIn ? ("authenticated" as const) : ("unauthenticated" as const),
  update: vi.fn(),
});

const createLoadingSession = () => ({
  data: null,
  status: "loading" as const,
  update: vi.fn(),
});

// The session callback strips a session whose refresh failed and marks it
// with an error (#5005). No browser session carries a token, including the
// portal layouts' server snapshot (#4990, #4991), which grants access.
const createSessionWithoutToken = () => ({
  data: {
    user: { id: "user-1" },
    error: "RefreshTokenError" as const,
    expires: "2099-01-01",
  },
  status: "authenticated" as const,
  update: vi.fn(),
});

const createSnapshotSession = (user: Record<string, unknown> = {}) => ({
  data: { user: { id: "user-1", ...user }, expires: "2099-01-01" },
  status: "authenticated" as const,
  update: vi.fn(),
});

describe("SWR Hooks", () => {
  const mockFetcher = vi.fn(() => Promise.resolve({ data: "test" }));

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseTenantSlugSafe.mockReturnValue(null);
    vi.mocked(useSession).mockReturnValue(
      createMockSession("signed-in") as ReturnType<typeof useSession>,
    );

    // Default SWR mock return value
    vi.mocked(useSWR).mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: true,
      isValidating: false,
      mutate: vi.fn(),
    });
  });

  describe("useSWRAuth", () => {
    it("fetches data when user is authenticated", () => {
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRAuth("test-key", mockFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        "tenant:-:user-1:test-key",
        mockFetcher,
        expect.objectContaining({
          dedupingInterval: 5000,
        }),
      );
    });

    it("scopes school portal cache entries to the school and account", () => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            id: "teacher-1",
            token: "test-token",
            tenantId: 42,
            scope: "school",
          },
          expires: "2099-01-01",
        },
        status: "authenticated",
        update: vi.fn(),
      } as ReturnType<typeof useSession>);

      renderHook(() =>
        useSWRAuth("school-supervisions-2026-08-24", mockFetcher),
      );

      expect(useSWR).toHaveBeenCalledWith(
        "school:42:teacher-1:school-supervisions-2026-08-24",
        mockFetcher,
        expect.any(Object),
      );
    });

    it("does not fetch a school portal cache entry without a tenant binding", () => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: { id: "teacher-1", token: "test-token", scope: "school" },
          expires: "2099-01-01",
        },
        status: "authenticated",
        update: vi.fn(),
      } as ReturnType<typeof useSession>);

      renderHook(() =>
        useSWRAuth("school-supervisions-2026-08-24", mockFetcher),
      );

      expect(useSWR).toHaveBeenCalledWith(
        null,
        mockFetcher,
        expect.any(Object),
      );
    });

    it("does not fetch when session is loading", () => {
      vi.mocked(useSession).mockReturnValue(
        createLoadingSession() as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRAuth("test-key", mockFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        null, // Key should be null when session is loading
        mockFetcher,
        expect.any(Object),
      );
    });

    it("does not fetch when user is unauthenticated", () => {
      vi.mocked(useSession).mockReturnValue(
        createMockSession() as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRAuth("test-key", mockFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        null, // Key should be null when unauthenticated
        mockFetcher,
        expect.any(Object),
      );
    });

    it("does not fetch when user has no token", () => {
      vi.mocked(useSession).mockReturnValue(
        createSessionWithoutToken() as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRAuth("test-key", mockFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        null, // Key should be null when no token
        mockFetcher,
        expect.any(Object),
      );
    });

    it("fetches from a portal layout's server snapshot without a token", () => {
      vi.mocked(useSession).mockReturnValue(
        createSnapshotSession() as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRAuth("test-key", mockFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        "tenant:-:user-1:test-key",
        mockFetcher,
        expect.any(Object),
      );
    });

    it("scopes a school snapshot to the school and account", () => {
      vi.mocked(useSession).mockReturnValue(
        createSnapshotSession({
          id: "teacher-1",
          scope: "school",
          tenantId: 42,
        }) as ReturnType<typeof useSession>,
      );

      renderHook(() =>
        useSWRAuth("school-supervisions-2026-08-24", mockFetcher),
      );

      expect(useSWR).toHaveBeenCalledWith(
        "school:42:teacher-1:school-supervisions-2026-08-24",
        mockFetcher,
        expect.any(Object),
      );
    });

    it("does not fetch when key is null", () => {
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRAuth(null, mockFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        null,
        mockFetcher,
        expect.any(Object),
      );
    });

    it("merges custom options with default config", () => {
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      const customOptions = { refreshInterval: 5000 };
      renderHook(() => useSWRAuth("test-key", mockFetcher, customOptions));

      expect(useSWR).toHaveBeenCalledWith(
        "tenant:-:user-1:test-key",
        mockFetcher,
        expect.objectContaining({
          refreshInterval: 5000,
        }),
      );
    });
  });

  describe("useImmutableSWR", () => {
    it("uses immutable config (no revalidation)", () => {
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() => useImmutableSWR("immutable-key", mockFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        "tenant:-:user-1:immutable-key",
        mockFetcher,
        expect.objectContaining({
          revalidateIfStale: false,
          revalidateOnFocus: false,
          revalidateOnReconnect: false,
        }),
      );
    });
  });

  describe("useSWRWithId", () => {
    const mockIdFetcher = vi.fn((id: string) =>
      Promise.resolve({ id, name: "test" }),
    );

    it("generates cache key with id when authenticated", () => {
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRWithId("entity", "123", mockIdFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        "tenant:-:user-1:entity-123",
        expect.any(Function),
        expect.any(Object),
      );
    });

    it("fetches from a portal layout's server snapshot without a token", () => {
      vi.mocked(useSession).mockReturnValue(
        createSnapshotSession() as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRWithId("entity", "123", mockIdFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        "tenant:-:user-1:entity-123",
        expect.any(Function),
        expect.any(Object),
      );
    });

    it("does not fetch for a session whose token was stripped", () => {
      vi.mocked(useSession).mockReturnValue(
        createSessionWithoutToken() as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRWithId("entity", "123", mockIdFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        null,
        expect.any(Function),
        expect.any(Object),
      );
    });

    it("does not fetch when id is null", () => {
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRWithId("entity", null, mockIdFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        null,
        expect.any(Function),
        expect.any(Object),
      );
    });

    it("does not fetch when id is undefined", () => {
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRWithId("entity", undefined, mockIdFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        null,
        expect.any(Function),
        expect.any(Object),
      );
    });

    it("does not fetch when session is loading", () => {
      vi.mocked(useSession).mockReturnValue(
        createLoadingSession() as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRWithId("entity", "123", mockIdFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        null,
        expect.any(Function),
        expect.any(Object),
      );
    });

    it("merges custom options", () => {
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() =>
        useSWRWithId("entity", "123", mockIdFetcher, { refreshInterval: 3000 }),
      );

      expect(useSWR).toHaveBeenCalledWith(
        "tenant:-:user-1:entity-123",
        expect.any(Function),
        expect.objectContaining({
          refreshInterval: 3000,
        }),
      );
    });

    it("passes id to fetcher when called", async () => {
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRWithId("entity", "123", mockIdFetcher));

      // Get the wrapped fetcher that was passed to useSWR
      const wrappedFetcher = vi.mocked(useSWR).mock.calls[0]?.[1];
      expect(wrappedFetcher).toBeDefined();

      // Call the wrapped fetcher to cover the fetcher(id!) line
      if (wrappedFetcher) {
        await wrappedFetcher("tenant:-:user-1:entity-123");
        expect(mockIdFetcher).toHaveBeenCalledWith("123");
      }
    });
  });

  describe("tenant key prefixing", () => {
    it("prefixes useSWRAuth key with tenant slug and account", () => {
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRAuth("students-list", mockFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        "school-a:user-1:students-list",
        mockFetcher,
        expect.any(Object),
      );
    });

    it("scopes a key without tenant context to the portal and account", () => {
      mockUseTenantSlugSafe.mockReturnValue(null);
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRAuth("students-list", mockFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        "tenant:-:user-1:students-list",
        mockFetcher,
        expect.any(Object),
      );
    });

    it("prefixes useSWRWithId key with tenant slug and account", () => {
      mockUseTenantSlugSafe.mockReturnValue("school-b");
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      const idFetcher = vi.fn(() => Promise.resolve({ id: "42" }));
      renderHook(() => useSWRWithId("student", "42", idFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        "school-b:user-1:student-42",
        expect.any(Function),
        expect.any(Object),
      );
    });
  });

  describe("useTenantMutate", () => {
    it("calls swr mutate with the tenant and account prefixed key", () => {
      mockUseTenantSlugSafe.mockReturnValue("school-a");

      const { result } = renderHook(() => useTenantMutate());
      void result.current("database-teachers-list");

      expect(mockSwrMutate).toHaveBeenCalledWith(
        "school-a:user-1:database-teachers-list",
      );
    });

    it("scopes the key to the portal and account without tenant context", () => {
      mockUseTenantSlugSafe.mockReturnValue(null);

      const { result } = renderHook(() => useTenantMutate());
      void result.current("some-key");

      expect(mockSwrMutate).toHaveBeenCalledWith("tenant:-:user-1:some-key");
    });

    it("mutates nothing without a signed-in session", async () => {
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      vi.mocked(useSession).mockReturnValue(
        createMockSession() as ReturnType<typeof useSession>,
      );

      const { result } = renderHook(() => useTenantMutate());

      await expect(result.current("some-key")).resolves.toBeUndefined();
      expect(mockSwrMutate).not.toHaveBeenCalled();
    });

    it("keeps a mutate of an earlier account on that account's keys", () => {
      // A rollback that resolves after an account switch must not reach the
      // cache of the account after it (#5006).
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      const { result, rerender } = renderHook(() => useTenantMutate());
      const earlier = result.current;
      vi.mocked(useSession).mockReturnValue({
        data: { user: { id: "user-2" }, expires: "2099-01-01" },
        status: "authenticated",
        update: vi.fn(),
      } as ReturnType<typeof useSession>);
      rerender();

      void earlier("ogs-students-42", { rows: [] }, { revalidate: false });
      void result.current("ogs-students-42");

      expect(mockSwrMutate).toHaveBeenNthCalledWith(
        1,
        "school-a:user-1:ogs-students-42",
        { rows: [] },
        { revalidate: false },
      );
      expect(mockSwrMutate).toHaveBeenNthCalledWith(
        2,
        "school-a:user-2:ogs-students-42",
      );
    });

    it("passes seeded data and revalidation options to swr mutate", () => {
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      const data = { groupId: "42" };
      const options = { revalidate: false };

      const { result } = renderHook(() => useTenantMutate());
      void result.current("ogs-students-42", data, options);

      expect(mockSwrMutate).toHaveBeenCalledWith(
        "school-a:user-1:ogs-students-42",
        data,
        options,
      );
    });
  });

  // ===========================================================================
  // useTenantMutateMatching — cross-tenant isolation is the bug magnet here.
  // The hook must invalidate ONLY caches for the active tenant; matching by
  // substring alone would silently invalidate every tenant in a multi-tab
  // session, which is hard to reproduce locally and a privacy footgun in
  // production (forces refetches that may surface in audit logs etc.).
  // ===========================================================================
  describe("useTenantMutateMatching", () => {
    /**
     * Capture the matcher predicate that the hook passes to swr.mutate so we
     * can interrogate it directly. Avoids relying on swr's internal cache
     * iteration in tests — the predicate is the real contract.
     */
    function renderAndCaptureMatcher(
      slug: string | null,
      substrings: string[],
    ): (key: unknown) => boolean {
      mockUseTenantSlugSafe.mockReturnValue(slug);
      const { result } = renderHook(() => useTenantMutateMatching(substrings));
      void result.current();

      const lastCall =
        mockSwrMutate.mock.calls[mockSwrMutate.mock.calls.length - 1];
      const matcher = lastCall?.[0];
      expect(typeof matcher).toBe("function");
      return matcher as (key: unknown) => boolean;
    }

    it("matches keys for the current tenant whose name contains a substring", () => {
      const matcher = renderAndCaptureMatcher("school-a", [
        "active-supervision",
        "ogs-students",
      ]);

      expect(matcher("school-a:user-1:active-supervision-dashboard-0")).toBe(
        true,
      );
      expect(matcher("school-a:user-1:ogs-students-42")).toBe(true);
    });

    it("rejects keys for OTHER tenants even when the substring matches", () => {
      // The bug class this guards against: a multi-tab user has tenant A in
      // one tab and tenant B in another. Saving a room in tenant A must not
      // invalidate tenant B's caches — otherwise B's tab refetches data the
      // user there did not ask for.
      const matcher = renderAndCaptureMatcher("school-a", [
        "active-supervision",
      ]);

      expect(matcher("school-b:user-1:active-supervision-dashboard-0")).toBe(
        false,
      );
      expect(matcher("school-c:user-1:active-supervision-visits-1")).toBe(
        false,
      );
    });

    it("rejects keys of another account of the same school", () => {
      // Another account signed in on this device keeps its own entries; a
      // change of this account does not reach them (#5006).
      const matcher = renderAndCaptureMatcher("school-a", [
        "active-supervision",
      ]);

      expect(matcher("school-a:user-2:active-supervision-dashboard-0")).toBe(
        false,
      );
      expect(matcher("school-a:user-1:active-supervision-dashboard-0")).toBe(
        true,
      );
    });

    it("rejects keys that match the tenant but no substring", () => {
      const matcher = renderAndCaptureMatcher("school-a", ["ogs-students"]);

      expect(matcher("school-a:user-1:database-teachers-list")).toBe(false);
      expect(matcher("school-a:user-1:students-search-foo")).toBe(false);
    });

    it("rejects non-string keys (SWR allows arrays/objects as keys)", () => {
      const matcher = renderAndCaptureMatcher("school-a", ["foo"]);

      expect(matcher(["school-a", "foo"])).toBe(false);
      expect(matcher({ slug: "school-a", key: "foo" })).toBe(false);
      expect(matcher(null)).toBe(false);
      expect(matcher(undefined)).toBe(false);
    });

    it("matches the portal and account scope when no tenant context", () => {
      // Without a TenantProvider the wrappers scope keys to the portal and
      // account (#5006); the matcher follows them, not the bare keys.
      const matcher = renderAndCaptureMatcher(null, ["operator-foo"]);

      expect(matcher("tenant:-:user-1:operator-foo-1")).toBe(true);
      expect(matcher("operator-foo-1")).toBe(false);
      expect(matcher("tenant:-:user-1:unrelated-bar")).toBe(false);
    });

    it("matches nothing without a signed-in session", async () => {
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      vi.mocked(useSession).mockReturnValue(
        createMockSession() as ReturnType<typeof useSession>,
      );
      const { result } = renderHook(() => useTenantMutateMatching(["foo"]));

      await expect(result.current()).resolves.toEqual([]);
      expect(mockSwrMutate).not.toHaveBeenCalled();
    });

    it("passes only the matcher when called without options (revalidate path)", () => {
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      const { result } = renderHook(() => useTenantMutateMatching(["foo"]));
      void result.current();

      const lastCall = mockSwrMutate.mock.calls.at(-1);
      expect(lastCall).toHaveLength(1);
    });

    it("clears matching entries without revalidation when called with { clear: true }", () => {
      // The clear path exists for consumers that must never act on
      // pre-mutation data: with the global keepPreviousData config a plain
      // revalidation still serves the stale entry first, so the entry has to
      // be deleted (data: undefined, revalidate: false).
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      const { result } = renderHook(() =>
        useTenantMutateMatching(["timetable-sammel-vertretung-"]),
      );
      void result.current({ clear: true });

      const lastCall = mockSwrMutate.mock.calls.at(-1);
      expect(lastCall).toHaveLength(3);
      expect(lastCall?.[1]).toBeUndefined();
      expect(lastCall?.[2]).toEqual({ revalidate: false });

      // Same matcher contract as the revalidate path: tenant-scoped.
      const matcher = lastCall?.[0] as (key: unknown) => boolean;
      expect(matcher("school-a:user-1:timetable-sammel-vertretung-x")).toBe(
        true,
      );
      expect(matcher("school-b:user-1:timetable-sammel-vertretung-x")).toBe(
        false,
      );
      expect(matcher("school-a:user-1:unrelated")).toBe(false);
    });

    it("leaves the keys named in `except` out of the revalidation", () => {
      // A decision revalidates every other filter and view variant of its
      // list; the variant it was made in already shows its result (#4988).
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      const { result } = renderHook(() =>
        useTenantMutateMatching(["request-feed:7:"]),
      );
      void result.current({ except: ['request-feed:7:open:[""]'] });

      const lastCall = mockSwrMutate.mock.calls.at(-1);
      // A plain revalidation: no data, so no answer in flight is discarded.
      expect(lastCall).toHaveLength(1);
      const matcher = lastCall?.[0] as (key: unknown) => boolean;
      expect(matcher('school-a:user-1:request-feed:7:open:["Mia"]')).toBe(true);
      expect(matcher('school-a:user-1:request-feed:7:history:[""]')).toBe(true);
      expect(matcher('school-a:user-1:request-feed:7:open:[""]')).toBe(false);
      expect(matcher('school-b:user-1:request-feed:7:open:["Mia"]')).toBe(
        false,
      );
    });

    it("returns the same callback when called with a fresh array of identical contents", () => {
      // Stability guard: callers pass a literal array each render. Without
      // the joined-string dep, useCallback would re-create the function every
      // render and cascade through every consumer using it as a dep —
      // forcing unrelated re-renders on every keystroke in the rooms search
      // box. This test pins the optimisation in place.
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      const { result, rerender } = renderHook(
        ({ subs }: { subs: string[] }) => useTenantMutateMatching(subs),
        { initialProps: { subs: ["a", "b"] } },
      );
      const firstCallback = result.current;

      rerender({ subs: ["a", "b"] }); // fresh array, same contents
      expect(result.current).toBe(firstCallback);

      rerender({ subs: ["a", "c"] }); // genuinely different contents
      expect(result.current).not.toBe(firstCallback);
    });
  });
});
