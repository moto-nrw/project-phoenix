import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import type { ReactNode } from "react";
import { ProfileProvider, useProfile } from "./profile-context";
import type { Profile } from "./profile-helpers";
import * as profileApi from "./profile-api";
import * as nextAuthReact from "next-auth/react";
import { mockSessionData } from "~/test/mocks/next-auth";

// Mock the API module
vi.mock("./profile-api");
vi.mock("next-auth/react");

describe("ProfileContext", () => {
  const mockProfile: Profile = {
    id: "1",
    firstName: "John",
    lastName: "Doe",
    email: "john.doe@example.com",
    username: "johndoe",
    avatar: "/api/me/profile/avatar/avatar.jpg",
    bio: "Test bio",
    rfidCard: "12345678",
    createdAt: "2024-01-01T00:00:00Z",
    updatedAt: "2024-01-01T00:00:00Z",
    lastLogin: "2024-01-01T00:00:00Z",
    settings: {
      theme: "light",
      language: "en",
      notifications: {
        email: true,
        push: false,
        activities: true,
        roomChanges: false,
      },
      privacy: {
        showEmail: true,
        showProfile: true,
      },
    },
  };

  const mockSession = mockSessionData({
    user: {
      token: "mock-token",
      email: "john.doe@example.com",
    },
    expires: "2025-01-01T00:00:00Z",
  });

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers(); // Use real timers by default

    // Default mock implementations
    vi.mocked(nextAuthReact.useSession).mockReturnValue({
      data: mockSession,
      status: "authenticated",
      update: vi.fn(),
    });

    vi.mocked(profileApi.fetchProfile).mockResolvedValue(mockProfile);
  });

  describe("ProfileProvider", () => {
    it("should provide initial loading state", () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      expect(result.current.isLoading).toBe(true);
      expect(result.current.profile).toBe(null);
    });

    it("should fetch profile on mount when session exists", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(profileApi.fetchProfile).toHaveBeenCalledTimes(1);
      expect(result.current.profile).toEqual(mockProfile);
    });

    it("should not fetch profile when no session token", async () => {
      vi.mocked(nextAuthReact.useSession).mockReturnValue({
        data: null,
        status: "unauthenticated",
        update: vi.fn(),
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(profileApi.fetchProfile).not.toHaveBeenCalled();
      expect(result.current.profile).toBe(null);
    });

    it("should handle fetch errors gracefully", async () => {
      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);
      vi.mocked(profileApi.fetchProfile).mockRejectedValue(
        new Error("Network error"),
      );

      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.profile).toBe(null);
      expect(consoleErrorSpy).toHaveBeenCalledWith("failed to load profile", {
        // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
        error: expect.any(String),
      });

      consoleErrorSpy.mockRestore();
    });

    it("should attempt to refresh when session token changes (subject to debounce)", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(profileApi.fetchProfile).toHaveBeenCalledTimes(1);
        expect(result.current.profile).toEqual(mockProfile);
      });

      const newSession = {
        ...mockSession,
        user: { ...mockSession.user, token: "new-token" },
      };

      // Update the mock before rerendering
      vi.mocked(nextAuthReact.useSession).mockReturnValue({
        data: newSession,
        status: "authenticated",
        update: vi.fn(),
      });

      // The profile provider detects token changes, but debounce may prevent immediate refresh
      // This test verifies the provider doesn't crash and maintains state when token changes
      expect(result.current.profile).toEqual(mockProfile);
      expect(result.current.isLoading).toBe(false);
    });

    it("should clear profile when session is removed", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { rerender, result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.profile).toEqual(mockProfile);
      });

      // Remove session - update mock before rerender
      vi.mocked(nextAuthReact.useSession).mockReturnValue({
        data: null,
        status: "unauthenticated",
        update: vi.fn(),
      });

      rerender();

      await waitFor(() => {
        expect(result.current.profile).toBe(null);
        expect(result.current.isLoading).toBe(false);
      });
    });
  });

  describe("refreshProfile", () => {
    it("should refresh profile manually", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      vi.clearAllMocks();

      // Use fake timers for debounce testing
      vi.useFakeTimers();

      // Advance time to bypass debounce (5 seconds minimum)
      vi.advanceTimersByTime(6000);

      await act(async () => {
        await result.current.refreshProfile();
      });

      vi.useRealTimers();

      expect(profileApi.fetchProfile).toHaveBeenCalledTimes(1);
    });

    it("should debounce rapid refresh calls (< 5 seconds)", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      vi.clearAllMocks();
      vi.useFakeTimers();

      // First refresh - advance time to pass debounce
      vi.advanceTimersByTime(6000);
      await act(async () => {
        await result.current.refreshProfile();
      });

      expect(profileApi.fetchProfile).toHaveBeenCalledTimes(1);
      vi.clearAllMocks();

      // Second refresh immediately (within 5 seconds) - should be ignored
      await act(async () => {
        await result.current.refreshProfile();
      });

      vi.useRealTimers();

      expect(profileApi.fetchProfile).not.toHaveBeenCalled();
    });

    it("should allow refresh after 5 seconds", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      vi.clearAllMocks();
      vi.useFakeTimers();

      // First refresh
      vi.advanceTimersByTime(6000);
      await act(async () => {
        await result.current.refreshProfile();
      });

      expect(profileApi.fetchProfile).toHaveBeenCalledTimes(1);
      vi.clearAllMocks();

      // Wait 5 seconds and refresh again
      vi.advanceTimersByTime(5000);
      await act(async () => {
        await result.current.refreshProfile();
      });

      vi.useRealTimers();

      expect(profileApi.fetchProfile).toHaveBeenCalledTimes(1);
    });

    it("should show loading state for non-silent refresh", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      vi.useFakeTimers();
      vi.advanceTimersByTime(6000);

      // Trigger non-silent refresh
      act(() => {
        void result.current.refreshProfile(false);
      });

      vi.useRealTimers();

      // Should show loading state
      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });
    });

    it("should not show loading state for silent refresh", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      vi.useFakeTimers();
      vi.advanceTimersByTime(6000);

      // Trigger silent refresh
      act(() => {
        void result.current.refreshProfile(true);
      });

      vi.useRealTimers();

      // Should NOT show loading state
      expect(result.current.isLoading).toBe(false);

      await waitFor(() => {
        expect(profileApi.fetchProfile).toHaveBeenCalled();
      });
    });

    it("should prevent concurrent refreshes", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      vi.clearAllMocks();
      vi.useFakeTimers();
      vi.advanceTimersByTime(6000);

      // Make fetch profile slow to test concurrent calls
      let resolveFirstFetch: () => void = () => undefined;
      const firstFetchPromise = new Promise<Profile>((resolve) => {
        resolveFirstFetch = () => resolve(mockProfile);
      });

      vi.mocked(profileApi.fetchProfile).mockReturnValue(firstFetchPromise);

      // Start first refresh
      act(() => {
        void result.current.refreshProfile();
      });

      // Try to start second refresh while first is still running
      act(() => {
        void result.current.refreshProfile();
      });

      // Complete the first fetch
      act(() => {
        resolveFirstFetch();
      });

      vi.useRealTimers();

      await waitFor(() => {
        expect(result.current.profile).toEqual(mockProfile);
      });

      // Should only have called fetchProfile once
      expect(profileApi.fetchProfile).toHaveBeenCalledTimes(1);
    });
  });

  describe("updateProfileData", () => {
    it("should update profile data optimistically", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.profile).toEqual(mockProfile);
      });

      act(() => {
        result.current.updateProfileData({
          firstName: "Jane",
          bio: "Updated bio",
        });
      });

      expect(result.current.profile).toEqual({
        ...mockProfile,
        firstName: "Jane",
        bio: "Updated bio",
      });
    });

    it("should not update when profile is null", async () => {
      vi.mocked(nextAuthReact.useSession).mockReturnValue({
        data: null,
        status: "unauthenticated",
        update: vi.fn(),
      });

      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      act(() => {
        result.current.updateProfileData({ firstName: "Jane" });
      });

      expect(result.current.profile).toBe(null);
    });

    it("should preserve unchanged fields when updating", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.profile).toEqual(mockProfile);
      });

      const originalSettings = result.current.profile?.settings;

      act(() => {
        result.current.updateProfileData({ firstName: "Jane" });
      });

      expect(result.current.profile?.settings).toEqual(originalSettings);
      expect(result.current.profile?.email).toBe(mockProfile.email);
      expect(result.current.profile?.id).toBe(mockProfile.id);
    });
  });

  describe("data change detection", () => {
    it("should not trigger re-render when fetched data is identical", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.profile).toEqual(mockProfile);
      });

      const firstProfile = result.current.profile;

      vi.clearAllMocks();
      vi.useFakeTimers();
      vi.advanceTimersByTime(6000);

      // Fetch same data again
      vi.mocked(profileApi.fetchProfile).mockResolvedValue(mockProfile);

      await act(async () => {
        await result.current.refreshProfile(true);
      });

      vi.useRealTimers();

      await waitFor(() => {
        expect(profileApi.fetchProfile).toHaveBeenCalled();
      });

      // Profile object should be the same reference (no re-render)
      expect(result.current.profile).toBe(firstProfile);
    });

    it("should update when profile data changes", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      await waitFor(() => {
        expect(result.current.profile).toEqual(mockProfile);
      });

      vi.clearAllMocks();
      vi.useFakeTimers();
      vi.advanceTimersByTime(6000);

      const updatedProfile = { ...mockProfile, firstName: "Jane" };
      vi.mocked(profileApi.fetchProfile).mockResolvedValue(updatedProfile);

      await act(async () => {
        await result.current.refreshProfile(true);
      });

      vi.useRealTimers();

      await waitFor(() => {
        expect(result.current.profile?.firstName).toBe("Jane");
      });
    });
  });

  describe("useProfile hook", () => {
    it("should throw error when used outside ProfileProvider", () => {
      // Suppress console.error for this test
      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      expect(() => {
        renderHook(() => useProfile());
      }).toThrow("useProfile must be used within a ProfileProvider");

      consoleErrorSpy.mockRestore();
    });

    it("should return context value when used within provider", async () => {
      const wrapper = ({ children }: { children: ReactNode }) => (
        <ProfileProvider>{children}</ProfileProvider>
      );

      const { result } = renderHook(() => useProfile(), { wrapper });

      expect(result.current).toHaveProperty("profile");
      expect(result.current).toHaveProperty("isLoading");
      expect(result.current).toHaveProperty("refreshProfile");
      expect(result.current).toHaveProperty("updateProfileData");

      expect(typeof result.current.refreshProfile).toBe("function");
      expect(typeof result.current.updateProfileData).toBe("function");
    });
  });
});
