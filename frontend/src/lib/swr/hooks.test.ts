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

const { mockSwrMutate } = vi.hoisted(() => ({
  mockSwrMutate: vi.fn(),
}));

// Mock swr
vi.mock("swr", () => ({
  default: vi.fn(),
  mutate: mockSwrMutate,
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
// eslint-disable-next-line no-restricted-imports -- test mock at module boundary
import useSWR from "swr";

// Helper to create mock session data
const createMockSession = (token?: string) => ({
  data: token ? { user: { id: "user-1", token }, expires: "2099-01-01" } : null,
  status: token ? ("authenticated" as const) : ("unauthenticated" as const),
  update: vi.fn(),
});

const createLoadingSession = () => ({
  data: null,
  status: "loading" as const,
  update: vi.fn(),
});

const createSessionWithoutToken = () => ({
  data: { user: { id: "user-1" }, expires: "2099-01-01" },
  status: "authenticated" as const,
  update: vi.fn(),
});

describe("SWR Hooks", () => {
  const mockFetcher = vi.fn(() => Promise.resolve({ data: "test" }));

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseTenantSlugSafe.mockReturnValue(null);

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
        "test-key",
        mockFetcher,
        expect.objectContaining({
          dedupingInterval: 5000,
        }),
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
        "test-key",
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
        "immutable-key",
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
        "entity-123",
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
        "entity-123",
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
        await wrappedFetcher("entity-123");
        expect(mockIdFetcher).toHaveBeenCalledWith("123");
      }
    });
  });

  describe("tenant key prefixing", () => {
    it("prefixes useSWRAuth key with tenant slug", () => {
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRAuth("students-list", mockFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        "school-a:students-list",
        mockFetcher,
        expect.any(Object),
      );
    });

    it("uses plain key when no tenant context", () => {
      mockUseTenantSlugSafe.mockReturnValue(null);
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      renderHook(() => useSWRAuth("students-list", mockFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        "students-list",
        mockFetcher,
        expect.any(Object),
      );
    });

    it("prefixes useSWRWithId key with tenant slug", () => {
      mockUseTenantSlugSafe.mockReturnValue("school-b");
      vi.mocked(useSession).mockReturnValue(
        createMockSession("test-token") as ReturnType<typeof useSession>,
      );

      const idFetcher = vi.fn(() => Promise.resolve({ id: "42" }));
      renderHook(() => useSWRWithId("student", "42", idFetcher));

      expect(useSWR).toHaveBeenCalledWith(
        "school-b:student-42",
        expect.any(Function),
        expect.any(Object),
      );
    });
  });

  describe("useTenantMutate", () => {
    it("calls swr mutate with tenant-prefixed key", () => {
      mockUseTenantSlugSafe.mockReturnValue("school-a");

      const { result } = renderHook(() => useTenantMutate());
      void result.current("database-teachers-list");

      expect(mockSwrMutate).toHaveBeenCalledWith(
        "school-a:database-teachers-list",
      );
    });

    it("calls swr mutate with plain key when no tenant context", () => {
      mockUseTenantSlugSafe.mockReturnValue(null);

      const { result } = renderHook(() => useTenantMutate());
      void result.current("some-key");

      expect(mockSwrMutate).toHaveBeenCalledWith("some-key");
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

      expect(matcher("school-a:active-supervision-dashboard-0")).toBe(true);
      expect(matcher("school-a:ogs-students-42")).toBe(true);
    });

    it("rejects keys for OTHER tenants even when the substring matches", () => {
      // The bug class this guards against: a multi-tab user has tenant A in
      // one tab and tenant B in another. Saving a room in tenant A must not
      // invalidate tenant B's caches — otherwise B's tab refetches data the
      // user there did not ask for.
      const matcher = renderAndCaptureMatcher("school-a", [
        "active-supervision",
      ]);

      expect(matcher("school-b:active-supervision-dashboard-0")).toBe(false);
      expect(matcher("school-c:active-supervision-visits-1")).toBe(false);
    });

    it("rejects keys that match the tenant but no substring", () => {
      const matcher = renderAndCaptureMatcher("school-a", ["ogs-students"]);

      expect(matcher("school-a:database-teachers-list")).toBe(false);
      expect(matcher("school-a:students-search-foo")).toBe(false);
    });

    it("rejects non-string keys (SWR allows arrays/objects as keys)", () => {
      const matcher = renderAndCaptureMatcher("school-a", ["foo"]);

      expect(matcher(["school-a", "foo"])).toBe(false);
      expect(matcher({ slug: "school-a", key: "foo" })).toBe(false);
      expect(matcher(null)).toBe(false);
      expect(matcher(undefined)).toBe(false);
    });

    it("falls back to substring-only matching when no tenant context", () => {
      // Operator dashboard / pre-auth scenarios use bare keys without slug
      // prefix. The hook must still work there without filtering everything
      // out — otherwise a colorless tenant context would silently no-op.
      const matcher = renderAndCaptureMatcher(null, ["operator-foo"]);

      expect(matcher("operator-foo-1")).toBe(true);
      expect(matcher("unrelated-bar")).toBe(false);
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
