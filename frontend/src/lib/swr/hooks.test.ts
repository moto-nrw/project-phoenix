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

      // The write itself goes through the generation lock (#5006).
      expect(mockSwrMutate).toHaveBeenNthCalledWith(
        1,
        "school-a:user-1:ogs-students-42",
        { rows: [] },
        expect.objectContaining({ revalidate: false }),
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
        expect.objectContaining(options),
      );
    });
  });

  // The matching contract (scope, account, substring, except, clear and
  // revalidate across every SWR state family) runs on the real SWR in
  // mutate-matching.test.tsx; it acts on the cache, not on a mocked mutate.
  describe("useTenantMutateMatching", () => {
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
