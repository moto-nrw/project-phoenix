import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useSWRAuth, useImmutableSWR, useSWRWithId } from "./hooks";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

// Mock swr
vi.mock("swr", () => ({
  default: vi.fn(),
}));

// Import mocked modules
import { useSession } from "next-auth/react";
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
          dedupingInterval: 2000,
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
});
