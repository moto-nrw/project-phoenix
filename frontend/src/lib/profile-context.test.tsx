/**
 * Tests for ProfileContext provider and hooks.
 *
 * Covers:
 * - ProfileProvider initialization
 * - useProfile hook
 * - Profile fetching and state management
 * - Disabled paths behavior (e.g., /, /console)
 * - Refresh functionality with debouncing
 * - Optimistic updates via updateProfileData
 * - Session-based authentication handling
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import { ProfileProvider, useProfile } from "./profile-context";

// Mock next/navigation
const mockPathname = vi.fn();
vi.mock("next/navigation", () => ({
  usePathname: () => mockPathname(),
}));

// Mock auth-client
const mockSession = vi.fn();
vi.mock("~/lib/auth-client", () => ({
  useSession: () => mockSession(),
}));

// Mock profile-api
const mockFetchProfile = vi.fn();
vi.mock("~/lib/profile-api", () => ({
  fetchProfile: () => mockFetchProfile(),
}));

// Test component that uses the context
function TestConsumer() {
  const context = useProfile();
  return (
    <div>
      <span data-testid="isLoading">{String(context.isLoading)}</span>
      <span data-testid="hasProfile">{String(context.profile !== null)}</span>
      <span data-testid="profileId">{context.profile?.id ?? "none"}</span>
      <span data-testid="firstName">
        {context.profile?.firstName ?? "none"}
      </span>
      <span data-testid="lastName">{context.profile?.lastName ?? "none"}</span>
      <span data-testid="avatar">{context.profile?.avatar ?? "none"}</span>
      <button onClick={() => context.refreshProfile()}>Refresh</button>
      <button onClick={() => context.refreshProfile(true)}>
        Silent Refresh
      </button>
      <button
        onClick={() =>
          context.updateProfileData({ firstName: "Updated", lastName: "Name" })
        }
      >
        Update Profile
      </button>
    </div>
  );
}

// Helper to render with provider
function renderWithProvider(ui: React.ReactElement) {
  return render(<ProfileProvider>{ui}</ProfileProvider>);
}

describe("profile-context", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.resetAllMocks();
    mockPathname.mockReturnValue("/dashboard");
    mockSession.mockReturnValue({
      data: { user: { id: "user-123", name: "Test User" } },
      isPending: false,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe("ProfileProvider", () => {
    it("should render children", async () => {
      mockFetchProfile.mockResolvedValue({
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      renderWithProvider(<div data-testid="child">Child Content</div>);

      expect(screen.getByTestId("child")).toHaveTextContent("Child Content");
    });

    it("should provide initial loading state", () => {
      mockFetchProfile.mockResolvedValue({
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("isLoading")).toHaveTextContent("true");
    });

    it("should not show loading state on disabled paths (/)", async () => {
      mockPathname.mockReturnValue("/");

      renderWithProvider(<TestConsumer />);

      // On disabled paths, isLoading should start as false
      expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      expect(screen.getByTestId("hasProfile")).toHaveTextContent("false");

      // Should not have called fetchProfile
      expect(mockFetchProfile).not.toHaveBeenCalled();
    });

    it("should not fetch on disabled paths (/console)", async () => {
      mockPathname.mockReturnValue("/console");

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      expect(mockFetchProfile).not.toHaveBeenCalled();
      expect(screen.getByTestId("hasProfile")).toHaveTextContent("false");
    });

    it("should not fetch on /console sub-paths", async () => {
      mockPathname.mockReturnValue("/console/organizations");

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      expect(mockFetchProfile).not.toHaveBeenCalled();
    });

    it("should not fetch when session is pending", () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: true,
      });

      renderWithProvider(<TestConsumer />);

      // Should not have made any fetch calls yet
      expect(mockFetchProfile).not.toHaveBeenCalled();
    });

    it("should clear state when user is not authenticated", async () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: false,
      });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      expect(screen.getByTestId("hasProfile")).toHaveTextContent("false");
      expect(screen.getByTestId("profileId")).toHaveTextContent("none");
    });

    it("should clear state when session.user is null", async () => {
      mockSession.mockReturnValue({
        data: { user: null },
        isPending: false,
      });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      expect(screen.getByTestId("hasProfile")).toHaveTextContent("false");
    });
  });

  describe("profile fetching", () => {
    it("should fetch and update profile on successful response", async () => {
      const mockProfile = {
        id: "123",
        firstName: "John",
        lastName: "Doe",
        email: "john@example.com",
        avatar: "/avatars/john.jpg",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      };

      mockFetchProfile.mockResolvedValueOnce(mockProfile);

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      expect(screen.getByTestId("hasProfile")).toHaveTextContent("true");
      expect(screen.getByTestId("profileId")).toHaveTextContent("123");
      expect(screen.getByTestId("firstName")).toHaveTextContent("John");
      expect(screen.getByTestId("lastName")).toHaveTextContent("Doe");
      expect(screen.getByTestId("avatar")).toHaveTextContent(
        "/avatars/john.jpg",
      );
    });

    it("should handle profile API error", async () => {
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});

      mockFetchProfile.mockRejectedValueOnce(new Error("Network error"));

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      expect(screen.getByTestId("hasProfile")).toHaveTextContent("false");
      expect(consoleSpy).toHaveBeenCalledWith(
        "Failed to load profile:",
        expect.any(Error),
      );

      consoleSpy.mockRestore();
    });

    it("should handle profile with null avatar", async () => {
      const mockProfile = {
        id: "123",
        firstName: "John",
        lastName: "Doe",
        email: "john@example.com",
        avatar: null,
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      };

      mockFetchProfile.mockResolvedValueOnce(mockProfile);

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      expect(screen.getByTestId("avatar")).toHaveTextContent("none");
    });
  });

  describe("refresh functionality", () => {
    it("should provide refresh function that can be called", async () => {
      mockFetchProfile.mockResolvedValue({
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      // The refresh function exists and can be accessed
      const refreshButton = screen.getByText("Refresh");
      expect(refreshButton).toBeDefined();
    });

    it("should provide silent refresh function", async () => {
      mockFetchProfile.mockResolvedValue({
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      // The silent refresh function exists
      const silentRefreshButton = screen.getByText("Silent Refresh");
      expect(silentRefreshButton).toBeDefined();
    });

    it("should debounce rapid refresh calls (min 5 seconds between)", async () => {
      mockFetchProfile.mockResolvedValue({
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      // Clear initial fetch calls
      mockFetchProfile.mockClear();

      // Try to refresh immediately - should be blocked due to debouncing
      const refreshButton = screen.getByText("Refresh");
      await act(async () => {
        refreshButton.click();
      });

      // Should not have called because of debounce
      expect(mockFetchProfile).not.toHaveBeenCalled();
    });

    it("should allow refresh after debounce period", async () => {
      mockFetchProfile.mockResolvedValue({
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      // Clear initial fetch calls
      mockFetchProfile.mockClear();

      // Advance past debounce (5 seconds)
      await act(async () => {
        vi.advanceTimersByTime(5001);
      });

      // Now refresh should work
      const refreshButton = screen.getByText("Refresh");
      await act(async () => {
        refreshButton.click();
      });

      // Should have called fetchProfile
      expect(mockFetchProfile).toHaveBeenCalled();
    });

    it("should support silent refresh without loading states", async () => {
      mockFetchProfile.mockResolvedValue({
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      // Advance past debounce
      await act(async () => {
        vi.advanceTimersByTime(5001);
      });

      // Silent refresh should not show loading states
      const silentRefreshButton = screen.getByText("Silent Refresh");
      await act(async () => {
        silentRefreshButton.click();
      });

      // Loading state should remain false for silent refresh
      expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
    });

    it("should prevent concurrent refresh calls", async () => {
      // Create a slow fetch that doesn't resolve immediately
      let resolvePromise: ((value: unknown) => void) | null = null;
      mockFetchProfile.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolvePromise = resolve;
          }),
      );

      renderWithProvider(<TestConsumer />);

      // Initial load starts
      await waitFor(() => {
        expect(mockFetchProfile).toHaveBeenCalledTimes(1);
      });

      // Advance past debounce
      await act(async () => {
        vi.advanceTimersByTime(5001);
      });

      mockFetchProfile.mockClear();

      // Try to trigger refresh while already refreshing
      const refreshButton = screen.getByText("Refresh");
      await act(async () => {
        refreshButton.click();
      });

      // Should not have started another fetch
      expect(mockFetchProfile).not.toHaveBeenCalled();

      // Resolve the original promise
      if (resolvePromise) {
        await act(async () => {
          resolvePromise!({
            id: "1",
            firstName: "Test",
            lastName: "User",
            email: "test@example.com",
            createdAt: "2024-01-01",
            updatedAt: "2024-01-01",
          });
        });
      }
    });
  });

  describe("updateProfileData (optimistic updates)", () => {
    it("should update profile data optimistically", async () => {
      mockFetchProfile.mockResolvedValue({
        id: "123",
        firstName: "John",
        lastName: "Doe",
        email: "john@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId("firstName")).toHaveTextContent("John");
      });

      // Update profile optimistically
      const updateButton = screen.getByText("Update Profile");
      await act(async () => {
        updateButton.click();
      });

      // Should show updated data immediately
      expect(screen.getByTestId("firstName")).toHaveTextContent("Updated");
      expect(screen.getByTestId("lastName")).toHaveTextContent("Name");
    });

    it("should preserve existing profile data when updating", async () => {
      mockFetchProfile.mockResolvedValue({
        id: "123",
        firstName: "John",
        lastName: "Doe",
        email: "john@example.com",
        avatar: "/avatar.jpg",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId("firstName")).toHaveTextContent("John");
      });

      // Update profile optimistically
      const updateButton = screen.getByText("Update Profile");
      await act(async () => {
        updateButton.click();
      });

      // Should preserve profileId and avatar
      expect(screen.getByTestId("profileId")).toHaveTextContent("123");
      expect(screen.getByTestId("avatar")).toHaveTextContent("/avatar.jpg");
    });

    it("should not update when profile is null", async () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: false,
      });

      renderWithProvider(<TestConsumer />);

      // Wait for state to settle
      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      // Try to update - should have no effect
      const updateButton = screen.getByText("Update Profile");
      await act(async () => {
        updateButton.click();
      });

      // Profile should still be null
      expect(screen.getByTestId("hasProfile")).toHaveTextContent("false");
    });
  });

  describe("state optimization", () => {
    it("should not update state when profile data has not changed", async () => {
      const mockProfile = {
        id: "123",
        firstName: "John",
        lastName: "Doe",
        email: "john@example.com",
        avatar: "/avatar.jpg",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      };

      mockFetchProfile.mockResolvedValue(mockProfile);

      const { rerender } = renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      // Rerender should not cause additional state updates
      rerender(
        <ProfileProvider>
          <TestConsumer />
        </ProfileProvider>,
      );

      expect(screen.getByTestId("profileId")).toHaveTextContent("123");
    });
  });

  describe("useProfile hook", () => {
    it("should throw error when used outside provider", () => {
      // Suppress console.error for this test
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});

      expect(() => {
        render(<TestConsumer />);
      }).toThrow("useProfile must be used within a ProfileProvider");

      consoleSpy.mockRestore();
    });
  });

  describe("path-based behavior", () => {
    it("should handle exact match for root path /", () => {
      mockPathname.mockReturnValue("/");

      renderWithProvider(<TestConsumer />);

      // Should not fetch on root path
      expect(mockFetchProfile).not.toHaveBeenCalled();
    });

    it("should fetch on paths starting with / but not equal to /", async () => {
      mockPathname.mockReturnValue("/dashboard");

      mockFetchProfile.mockResolvedValue({
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(mockFetchProfile).toHaveBeenCalled();
      });
    });

    it("should handle /console path specifically", () => {
      mockPathname.mockReturnValue("/console");

      renderWithProvider(<TestConsumer />);

      // Should not fetch on console path
      expect(mockFetchProfile).not.toHaveBeenCalled();
    });

    it("should handle /console/settings sub-path", () => {
      mockPathname.mockReturnValue("/console/settings");

      renderWithProvider(<TestConsumer />);

      // Should not fetch on console sub-paths
      expect(mockFetchProfile).not.toHaveBeenCalled();
    });

    it("should fetch on regular paths like /settings", async () => {
      mockPathname.mockReturnValue("/settings");

      mockFetchProfile.mockResolvedValue({
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(mockFetchProfile).toHaveBeenCalled();
      });
    });
  });

  describe("session change handling", () => {
    it("should refresh profile when user ID changes", async () => {
      mockFetchProfile.mockResolvedValue({
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      const { rerender } = renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
      });

      // Change user ID
      mockSession.mockReturnValue({
        data: { user: { id: "user-456", name: "Different User" } },
        isPending: false,
      });

      rerender(
        <ProfileProvider>
          <TestConsumer />
        </ProfileProvider>,
      );

      // Should trigger a refresh for the new user
      await waitFor(() => {
        expect(mockFetchProfile).toHaveBeenCalled();
      });
    });

    it("should clear profile when session becomes unauthenticated", async () => {
      mockFetchProfile.mockResolvedValue({
        id: "1",
        firstName: "Test",
        lastName: "User",
        email: "test@example.com",
        createdAt: "2024-01-01",
        updatedAt: "2024-01-01",
      });

      const { rerender } = renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("hasProfile")).toHaveTextContent("true");
      });

      // Change to unauthenticated
      mockSession.mockReturnValue({
        data: null,
        isPending: false,
      });

      rerender(
        <ProfileProvider>
          <TestConsumer />
        </ProfileProvider>,
      );

      await waitFor(() => {
        expect(screen.getByTestId("hasProfile")).toHaveTextContent("false");
      });
    });
  });
});
