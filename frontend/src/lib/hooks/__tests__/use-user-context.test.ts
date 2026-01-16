/**
 * Tests for use-user-context.ts hook
 *
 * Tests:
 * - useUserContext hook
 * - Correct API response handling
 */

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";

// Mock useImmutableSWR
vi.mock("~/lib/swr", () => ({
  useImmutableSWR: vi.fn(() => ({
    data: undefined,
    isLoading: true,
    error: undefined,
  })),
}));

// Import after mocking
import { useUserContext } from "../use-user-context";
import { useImmutableSWR } from "~/lib/swr";

describe("useUserContext", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("hook initialization", () => {
    it("returns loading state initially", () => {
      vi.mocked(useImmutableSWR).mockReturnValue({
        data: undefined,
        isLoading: true,
        error: undefined,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useUserContext());

      expect(result.current.isLoading).toBe(true);
      expect(result.current.userContext).toBeUndefined();
      expect(result.current.error).toBeUndefined();
      expect(result.current.isReady).toBe(false);
    });

    it("returns user context data when loaded", () => {
      const mockContext = {
        educationalGroups: [{ id: "1", name: "Group 1" }],
        supervisedGroups: [{ id: "2", name: "Supervised 1" }],
        currentStaff: { id: "100", personId: "200" },
        educationalGroupIds: ["1"],
        educationalGroupRoomNames: ["Raum 1"],
        supervisedRoomNames: ["Raum 2"],
      };

      vi.mocked(useImmutableSWR).mockReturnValue({
        data: mockContext,
        isLoading: false,
        error: undefined,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useUserContext());

      expect(result.current.isLoading).toBe(false);
      expect(result.current.userContext).toEqual(mockContext);
      expect(result.current.isReady).toBe(true);
    });

    it("returns error state on fetch failure", () => {
      const error = new Error("User context fetch failed: 500");

      vi.mocked(useImmutableSWR).mockReturnValue({
        data: undefined,
        isLoading: false,
        error,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useUserContext());

      expect(result.current.isLoading).toBe(false);
      expect(result.current.error).toBe(error);
      expect(result.current.isReady).toBe(true); // Ready even on error
    });

    it("is ready when data is present even if empty", () => {
      const emptyContext = {
        educationalGroups: [],
        supervisedGroups: [],
        currentStaff: null,
        educationalGroupIds: [],
        educationalGroupRoomNames: [],
        supervisedRoomNames: [],
      };

      vi.mocked(useImmutableSWR).mockReturnValue({
        data: emptyContext,
        isLoading: false,
        error: undefined,
        isValidating: false,
        mutate: vi.fn(),
      });

      const { result } = renderHook(() => useUserContext());

      expect(result.current.isReady).toBe(true);
      expect(result.current.userContext).toEqual(emptyContext);
    });

    it("calls useImmutableSWR with correct cache key", () => {
      renderHook(() => useUserContext());

      expect(useImmutableSWR).toHaveBeenCalledWith(
        "user-context",
        expect.any(Function),
      );
    });
  });

  describe("fetcher function", () => {
    it("fetches from correct endpoint", async () => {
      const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            data: {
              educationalGroups: [],
              supervisedGroups: [],
              currentStaff: null,
              educationalGroupIds: [],
              educationalGroupRoomNames: [],
              supervisedRoomNames: [],
            },
          }),
      } as Response);

      // Get the fetcher from useImmutableSWR call
      renderHook(() => useUserContext());
      const fetcher = vi.mocked(useImmutableSWR).mock.calls[0]?.[1];

      if (fetcher) {
        await fetcher();
      }

      expect(fetchMock).toHaveBeenCalledWith("/api/user-context", {
        credentials: "include",
      });

      fetchMock.mockRestore();
    });

    it("throws error on non-ok response", async () => {
      const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue({
        ok: false,
        status: 401,
      } as Response);

      renderHook(() => useUserContext());
      const fetcher = vi.mocked(useImmutableSWR).mock.calls[0]?.[1];

      if (fetcher) {
        await expect(fetcher()).rejects.toThrow(
          "User context fetch failed: 401",
        );
      }

      fetchMock.mockRestore();
    });
  });
});
