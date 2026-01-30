import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import type { Session } from "next-auth";
import type { ReactNode } from "react";
import {
  UserContextProvider,
  useUserContext,
  useHasEducationalGroups,
} from "./usercontext-context";
import type { BackendEducationalGroup } from "./usercontext-helpers";
import { mockSessionData } from "~/test/mocks/next-auth";

// Mock dependencies
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  usePathname: vi.fn(),
}));

vi.mock("./supervision-context", () => ({
  useSupervision: vi.fn(),
}));

vi.mock("./usercontext-helpers", () => ({
  mapEducationalGroupResponse: vi.fn(),
}));

// Import mocked modules for type-safe access
import { useSession } from "next-auth/react";
import { usePathname } from "next/navigation";
import { useSupervision } from "./supervision-context";
import { mapEducationalGroupResponse } from "./usercontext-helpers";

describe("UserContextProvider", () => {
  const mockSession: Session = mockSessionData({
    user: {
      token: "mock-token",
    },
    expires: new Date(Date.now() + 1000 * 60 * 60).toISOString(),
  });

  const mockBackendGroups: BackendEducationalGroup[] = [
    {
      id: 1,
      name: "Group A",
      room_id: 101,
      room: { id: 101, name: "Room 101" },
    },
    {
      id: 2,
      name: "Group B",
      room_id: 102,
      room: { id: 102, name: "Room 102" },
      via_substitution: true,
    },
  ];

  const mockMappedGroups = [
    {
      id: "1",
      name: "Group A",
      room_id: "101",
      room: { id: "101", name: "Room 101" },
      viaSubstitution: false,
    },
    {
      id: "2",
      name: "Group B",
      room_id: "102",
      room: { id: "102", name: "Room 102" },
      viaSubstitution: true,
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();

    // Default mock implementations
    vi.mocked(useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });

    vi.mocked(usePathname).mockReturnValue("/dashboard");

    vi.mocked(useSupervision).mockReturnValue({
      hasGroups: true,
      isLoadingGroups: false,
      groups: mockBackendGroups,
      isSupervising: false,
      supervisedRoomId: undefined,
      supervisedRoomName: undefined,
      isLoadingSupervision: false,
      refresh: vi.fn(),
    });

    vi.mocked(mapEducationalGroupResponse).mockImplementation((group) => ({
      id: group.id.toString(),
      name: group.name,
      room_id: group.room_id?.toString(),
      room: group.room
        ? {
            id: group.room.id.toString(),
            name: group.room.name,
          }
        : undefined,
      viaSubstitution: group.via_substitution ?? false,
    }));
  });

  describe("Provider Rendering", () => {
    it("should render children correctly", () => {
      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current).toBeDefined();
    });

    it("should throw error when useUserContext is used outside provider", () => {
      expect(() => {
        renderHook(() => useUserContext());
      }).toThrow("useUserContext must be used within a UserContextProvider");
    });

    it("should throw error when useHasEducationalGroups is used outside provider", () => {
      expect(() => {
        renderHook(() => useHasEducationalGroups());
      }).toThrow("useUserContext must be used within a UserContextProvider");
    });
  });

  describe("Educational Groups", () => {
    it("should provide mapped educational groups when authenticated", () => {
      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.educationalGroups).toEqual(mockMappedGroups);
      expect(result.current.hasEducationalGroups).toBe(true);
      expect(result.current.isLoading).toBe(false);
      expect(result.current.error).toBeNull();
    });

    it("should return empty groups when not authenticated", () => {
      vi.mocked(useSession).mockReturnValue({
        data: null,
        status: "unauthenticated",
        update: vi.fn(),
      });

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.educationalGroups).toEqual([]);
      expect(result.current.hasEducationalGroups).toBe(false);
      expect(result.current.isLoading).toBe(false);
    });

    it("should return empty groups when no token is present", () => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          ...mockSession,
          user: { ...mockSession.user, token: undefined },
        },
        status: "authenticated",
        update: vi.fn(),
      });

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.educationalGroups).toEqual([]);
      expect(result.current.hasEducationalGroups).toBe(false);
    });

    it("should return empty groups on auth pages", () => {
      vi.mocked(usePathname).mockReturnValue("/");

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.educationalGroups).toEqual([]);
      expect(result.current.hasEducationalGroups).toBe(false);
    });

    it("should return empty groups on register page", () => {
      vi.mocked(usePathname).mockReturnValue("/register");

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.educationalGroups).toEqual([]);
      expect(result.current.hasEducationalGroups).toBe(false);
    });

    it("should map all groups from supervision context", () => {
      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(mapEducationalGroupResponse).toHaveBeenCalledTimes(
        mockBackendGroups.length,
      );
      // Array.map passes (item, index, array) - check first argument only
      expect(mapEducationalGroupResponse).toHaveBeenNthCalledWith(
        1,
        mockBackendGroups[0],
        0,
        mockBackendGroups,
      );
      expect(mapEducationalGroupResponse).toHaveBeenNthCalledWith(
        2,
        mockBackendGroups[1],
        1,
        mockBackendGroups,
      );
      expect(result.current.educationalGroups.length).toBe(
        mockBackendGroups.length,
      );
    });
  });

  describe("Loading States", () => {
    it("should show loading when session is loading", () => {
      vi.mocked(useSession).mockReturnValue({
        data: null,
        status: "loading",
        update: vi.fn(),
      });

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.isLoading).toBe(true);
    });

    it("should show loading when supervision groups are loading", () => {
      vi.mocked(useSupervision).mockReturnValue({
        hasGroups: false,
        isLoadingGroups: true,
        groups: [],
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
        refresh: vi.fn(),
      });

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.isLoading).toBe(true);
    });

    it("should not show loading when session is loading but on auth page", () => {
      vi.mocked(useSession).mockReturnValue({
        data: null,
        status: "loading",
        update: vi.fn(),
      });
      vi.mocked(usePathname).mockReturnValue("/");

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.isLoading).toBe(true);
    });

    it("should not be loading when authenticated with loaded groups", () => {
      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.isLoading).toBe(false);
    });
  });

  describe("Refetch Functionality", () => {
    it("should call supervision refresh when refetch is called", async () => {
      const mockRefresh = vi.fn().mockResolvedValue(undefined);
      vi.mocked(useSupervision).mockReturnValue({
        hasGroups: true,
        isLoadingGroups: false,
        groups: mockBackendGroups,
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
        refresh: mockRefresh,
      });

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      await result.current.refetch();

      await waitFor(() => {
        expect(mockRefresh).toHaveBeenCalledTimes(1);
      });
    });

    it("should handle refresh errors gracefully", async () => {
      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);
      const mockRefresh = vi
        .fn()
        .mockRejectedValue(new Error("Refresh failed"));

      vi.mocked(useSupervision).mockReturnValue({
        hasGroups: true,
        isLoadingGroups: false,
        groups: mockBackendGroups,
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
        refresh: mockRefresh,
      });

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      await result.current.refetch();

      await waitFor(() => {
        expect(consoleErrorSpy).toHaveBeenCalledWith(
          "Failed to refresh supervision context:",
          expect.any(Error),
        );
      });

      consoleErrorSpy.mockRestore();
    });

    it("should provide a stable refetch function reference", () => {
      const { result, rerender } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      const firstRefetch = result.current.refetch;
      rerender();
      const secondRefetch = result.current.refetch;

      expect(firstRefetch).toBe(secondRefetch);
    });
  });

  describe("useHasEducationalGroups Hook", () => {
    it("should return correct values when user has groups", () => {
      const { result } = renderHook(() => useHasEducationalGroups(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.hasEducationalGroups).toBe(true);
      expect(result.current.isLoading).toBe(false);
      expect(result.current.error).toBeNull();
    });

    it("should return false when user has no groups", () => {
      vi.mocked(useSupervision).mockReturnValue({
        hasGroups: false,
        isLoadingGroups: false,
        groups: [],
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
        refresh: vi.fn(),
      });

      const { result } = renderHook(() => useHasEducationalGroups(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.hasEducationalGroups).toBe(false);
      expect(result.current.isLoading).toBe(false);
      expect(result.current.error).toBeNull();
    });

    it("should indicate loading state", () => {
      vi.mocked(useSupervision).mockReturnValue({
        hasGroups: false,
        isLoadingGroups: true,
        groups: [],
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
        refresh: vi.fn(),
      });

      const { result } = renderHook(() => useHasEducationalGroups(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.isLoading).toBe(true);
    });
  });

  describe("Edge Cases", () => {
    it("should handle empty supervision groups array", () => {
      vi.mocked(useSupervision).mockReturnValue({
        hasGroups: false,
        isLoadingGroups: false,
        groups: [],
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
        refresh: vi.fn(),
      });

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.educationalGroups).toEqual([]);
      expect(result.current.hasEducationalGroups).toBe(false);
    });

    it("should handle groups without room information", () => {
      const groupsWithoutRooms: BackendEducationalGroup[] = [
        {
          id: 1,
          name: "Group A",
        },
      ];

      vi.mocked(useSupervision).mockReturnValue({
        hasGroups: true,
        isLoadingGroups: false,
        groups: groupsWithoutRooms,
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
        refresh: vi.fn(),
      });

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.educationalGroups).toHaveLength(1);
      expect(result.current.educationalGroups[0]?.room_id).toBeUndefined();
      expect(result.current.educationalGroups[0]?.room).toBeUndefined();
    });

    it("should maintain error as null (future error handling support)", () => {
      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.error).toBeNull();
    });

    it("should handle session without user object", () => {
      vi.mocked(useSession).mockReturnValue({
        data: { expires: new Date().toISOString() } as Session,
        status: "authenticated",
        update: vi.fn(),
      });

      const { result } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      expect(result.current.educationalGroups).toEqual([]);
      expect(result.current.hasEducationalGroups).toBe(false);
    });
  });

  describe("Memoization and Performance", () => {
    it("should memoize educational groups when supervision groups don't change", () => {
      const { result, rerender } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      const firstGroups = result.current.educationalGroups;
      rerender();
      const secondGroups = result.current.educationalGroups;

      expect(firstGroups).toBe(secondGroups);
    });

    it("should update groups when supervision groups change", () => {
      const { result, rerender } = renderHook(() => useUserContext(), {
        wrapper: ({ children }: { children: ReactNode }) => (
          <UserContextProvider>{children}</UserContextProvider>
        ),
      });

      const firstGroups = result.current.educationalGroups;

      // Change the supervision groups
      vi.mocked(useSupervision).mockReturnValue({
        hasGroups: true,
        isLoadingGroups: false,
        groups: [
          {
            id: 3,
            name: "Group C",
            room_id: 103,
            room: { id: 103, name: "Room 103" },
          },
        ],
        isSupervising: false,
        supervisedRoomId: undefined,
        supervisedRoomName: undefined,
        isLoadingSupervision: false,
        refresh: vi.fn(),
      });

      rerender();
      const secondGroups = result.current.educationalGroups;

      expect(firstGroups).not.toBe(secondGroups);
      expect(secondGroups).toHaveLength(1);
      expect(secondGroups[0]?.id).toBe("3");
    });
  });
});
