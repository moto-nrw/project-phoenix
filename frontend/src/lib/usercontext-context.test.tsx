/**
 * Tests for UserContextContext provider and hooks.
 *
 * Covers:
 * - UserContextProvider initialization and state management
 * - useUserContext hook (throws when used outside provider)
 * - useHasEducationalGroups convenience hook
 * - Integration with SupervisionContext for group data
 * - Session authentication handling
 * - Auth page detection for "/" and "/register"
 * - Group mapping via mapEducationalGroupResponse
 * - Refetch functionality
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import {
  UserContextProvider,
  useUserContext,
  useHasEducationalGroups,
} from "./usercontext-context";
import type { BackendEducationalGroup } from "./usercontext-helpers";

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

// Mock supervision-context
const mockSupervisionContext = vi.fn();
vi.mock("./supervision-context", () => ({
  useSupervision: () => mockSupervisionContext(),
}));

// Mock usercontext-helpers
vi.mock("./usercontext-helpers", () => ({
  mapEducationalGroupResponse: (data: BackendEducationalGroup) => ({
    id: data.id.toString(),
    name: data.name,
    room_id: data.room_id?.toString(),
    room: data.room
      ? {
          id: data.room.id.toString(),
          name: data.room.name,
        }
      : undefined,
    viaSubstitution: data.via_substitution ?? false,
  }),
}));

// Test component that uses the full context
function TestConsumer() {
  const context = useUserContext();
  return (
    <div>
      <span data-testid="groupCount">{context.educationalGroups.length}</span>
      <span data-testid="hasEducationalGroups">
        {String(context.hasEducationalGroups)}
      </span>
      <span data-testid="isLoading">{String(context.isLoading)}</span>
      <span data-testid="error">{context.error ?? "none"}</span>
      <button onClick={() => context.refetch()}>Refetch</button>
      {context.educationalGroups.map((g) => (
        <span key={g.id} data-testid={`group-${g.id}`}>
          {g.name}
        </span>
      ))}
    </div>
  );
}

// Test component for the convenience hook
function TestHasEducationalGroupsConsumer() {
  const { hasEducationalGroups, isLoading, error } = useHasEducationalGroups();
  return (
    <div>
      <span data-testid="hasEduGroups">{String(hasEducationalGroups)}</span>
      <span data-testid="isLoadingHook">{String(isLoading)}</span>
      <span data-testid="errorHook">{error ?? "none"}</span>
    </div>
  );
}

// Helper to render with provider
function renderWithProvider(ui: React.ReactElement) {
  return render(<UserContextProvider>{ui}</UserContextProvider>);
}

describe("usercontext-context", () => {
  beforeEach(() => {
    vi.resetAllMocks();

    // Default mock values
    mockPathname.mockReturnValue("/dashboard");
    mockSession.mockReturnValue({
      data: { user: { id: "user-123", name: "Test User" } },
      isPending: false,
    });
    mockSupervisionContext.mockReturnValue({
      groups: [],
      isLoadingGroups: false,
      refresh: vi.fn().mockResolvedValue(undefined),
    });
  });

  afterEach(() => {
    vi.resetAllMocks();
  });

  describe("UserContextProvider", () => {
    it("should render children", () => {
      renderWithProvider(<div data-testid="child">Child Content</div>);

      expect(screen.getByTestId("child")).toHaveTextContent("Child Content");
    });

    it("should provide initial state with no groups", () => {
      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toHaveTextContent("0");
      expect(screen.getByTestId("hasEducationalGroups")).toHaveTextContent(
        "false",
      );
      expect(screen.getByTestId("error")).toHaveTextContent("none");
    });

    it("should show loading when session is pending", () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: true,
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("isLoading")).toHaveTextContent("true");
    });

    it("should show loading when groups are loading", () => {
      mockSupervisionContext.mockReturnValue({
        groups: [],
        isLoadingGroups: true,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("isLoading")).toHaveTextContent("true");
    });

    it("should not be loading when session pending completes", () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: false,
      });
      mockSupervisionContext.mockReturnValue({
        groups: [],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
    });
  });

  describe("authentication page detection", () => {
    it("should return empty groups on root path (/)", () => {
      mockPathname.mockReturnValue("/");
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group 1" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toHaveTextContent("0");
      expect(screen.getByTestId("hasEducationalGroups")).toHaveTextContent(
        "false",
      );
    });

    it("should return empty groups on register path (/register)", () => {
      mockPathname.mockReturnValue("/register");
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group 1" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toHaveTextContent("0");
      expect(screen.getByTestId("hasEducationalGroups")).toHaveTextContent(
        "false",
      );
    });

    it("should provide groups on non-auth pages", () => {
      mockPathname.mockReturnValue("/dashboard");
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group 1" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toHaveTextContent("1");
      expect(screen.getByTestId("hasEducationalGroups")).toHaveTextContent(
        "true",
      );
    });
  });

  describe("session handling", () => {
    it("should return empty groups when session is pending", () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: true,
      });
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group 1" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toHaveTextContent("0");
    });

    it("should return empty groups when no user session", () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: false,
      });
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group 1" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toHaveTextContent("0");
    });

    it("should return empty groups when session data exists but no user", () => {
      mockSession.mockReturnValue({
        data: {},
        isPending: false,
      });
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group 1" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toHaveTextContent("0");
    });

    it("should provide groups when authenticated with user", () => {
      mockSession.mockReturnValue({
        data: { user: { id: "123" } },
        isPending: false,
      });
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group 1" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toHaveTextContent("1");
    });
  });

  describe("group mapping", () => {
    it("should map supervision groups to educational groups", () => {
      const backendGroups = [
        {
          id: 1,
          name: "Class 1A",
          room_id: 10,
          room: { id: 10, name: "R101" },
        },
        { id: 2, name: "Class 1B" },
      ];
      mockSupervisionContext.mockReturnValue({
        groups: backendGroups,
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toHaveTextContent("2");
      expect(screen.getByTestId("group-1")).toHaveTextContent("Class 1A");
      expect(screen.getByTestId("group-2")).toHaveTextContent("Class 1B");
    });

    it("should update hasEducationalGroups based on mapped groups", () => {
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("hasEducationalGroups")).toHaveTextContent(
        "true",
      );
    });

    it("should set hasEducationalGroups to false when no groups", () => {
      mockSupervisionContext.mockReturnValue({
        groups: [],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("hasEducationalGroups")).toHaveTextContent(
        "false",
      );
    });
  });

  describe("refetch functionality", () => {
    it("should call supervision refresh on refetch", async () => {
      const mockRefresh = vi.fn().mockResolvedValue(undefined);
      mockSupervisionContext.mockReturnValue({
        groups: [],
        isLoadingGroups: false,
        refresh: mockRefresh,
      });

      renderWithProvider(<TestConsumer />);

      const refetchButton = screen.getByText("Refetch");
      await act(async () => {
        refetchButton.click();
      });

      expect(mockRefresh).toHaveBeenCalled();
    });

    it("should handle refetch errors gracefully", async () => {
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});
      const mockRefresh = vi
        .fn()
        .mockRejectedValue(new Error("Refresh failed"));
      mockSupervisionContext.mockReturnValue({
        groups: [],
        isLoadingGroups: false,
        refresh: mockRefresh,
      });

      renderWithProvider(<TestConsumer />);

      const refetchButton = screen.getByText("Refetch");
      await act(async () => {
        refetchButton.click();
      });

      // Should log the error but not throw
      await waitFor(() => {
        expect(consoleSpy).toHaveBeenCalledWith(
          "Failed to refresh supervision context:",
          expect.any(Error),
        );
      });

      consoleSpy.mockRestore();
    });
  });

  describe("useUserContext hook", () => {
    it("should throw error when used outside provider", () => {
      // Suppress console.error for this test
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});

      expect(() => {
        render(<TestConsumer />);
      }).toThrow("useUserContext must be used within a UserContextProvider");

      consoleSpy.mockRestore();
    });

    it("should provide context when used within provider", () => {
      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toBeDefined();
      expect(screen.getByTestId("hasEducationalGroups")).toBeDefined();
      expect(screen.getByTestId("isLoading")).toBeDefined();
      expect(screen.getByTestId("error")).toBeDefined();
    });
  });

  describe("useHasEducationalGroups hook", () => {
    it("should throw error when used outside provider", () => {
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});

      expect(() => {
        render(<TestHasEducationalGroupsConsumer />);
      }).toThrow("useUserContext must be used within a UserContextProvider");

      consoleSpy.mockRestore();
    });

    it("should return hasEducationalGroups from context", () => {
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestHasEducationalGroupsConsumer />);

      expect(screen.getByTestId("hasEduGroups")).toHaveTextContent("true");
    });

    it("should return isLoading from context", () => {
      mockSession.mockReturnValue({
        data: { user: { id: "123" } },
        isPending: true,
      });

      renderWithProvider(<TestHasEducationalGroupsConsumer />);

      expect(screen.getByTestId("isLoadingHook")).toHaveTextContent("true");
    });

    it("should return error from context", () => {
      renderWithProvider(<TestHasEducationalGroupsConsumer />);

      expect(screen.getByTestId("errorHook")).toHaveTextContent("none");
    });

    it("should return false when no groups", () => {
      mockSupervisionContext.mockReturnValue({
        groups: [],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestHasEducationalGroupsConsumer />);

      expect(screen.getByTestId("hasEduGroups")).toHaveTextContent("false");
    });
  });

  describe("loading state combinations", () => {
    it("should show loading when session pending and groups not loading", () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: true,
      });
      mockSupervisionContext.mockReturnValue({
        groups: [],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("isLoading")).toHaveTextContent("true");
    });

    it("should show loading when session ready but groups loading", () => {
      mockSession.mockReturnValue({
        data: { user: { id: "123" } },
        isPending: false,
      });
      mockSupervisionContext.mockReturnValue({
        groups: [],
        isLoadingGroups: true,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("isLoading")).toHaveTextContent("true");
    });

    it("should not show loading when authenticated but on auth page", () => {
      mockPathname.mockReturnValue("/");
      mockSession.mockReturnValue({
        data: { user: { id: "123" } },
        isPending: false,
      });
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group" }],
        isLoadingGroups: true,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      // On auth page, shouldProvideData is false, so isLoadingGroups doesn't affect overall loading
      expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
    });

    it("should not show loading when not authenticated", () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: false,
      });
      mockSupervisionContext.mockReturnValue({
        groups: [],
        isLoadingGroups: true,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      // Not authenticated, so shouldProvideData is false, isLoadingGroups doesn't matter
      expect(screen.getByTestId("isLoading")).toHaveTextContent("false");
    });
  });

  describe("memoization", () => {
    it("should memoize mapped groups based on dependencies", () => {
      const groups = [{ id: 1, name: "Group 1" }];
      mockSupervisionContext.mockReturnValue({
        groups,
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      const { rerender } = renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toHaveTextContent("1");

      // Rerender with same props
      rerender(
        <UserContextProvider>
          <TestConsumer />
        </UserContextProvider>,
      );

      expect(screen.getByTestId("groupCount")).toHaveTextContent("1");
    });

    it("should update when supervision groups change", () => {
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group 1" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      const { rerender } = renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("groupCount")).toHaveTextContent("1");

      // Update mock and rerender
      mockSupervisionContext.mockReturnValue({
        groups: [
          { id: 1, name: "Group 1" },
          { id: 2, name: "Group 2" },
        ],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      rerender(
        <UserContextProvider>
          <TestConsumer />
        </UserContextProvider>,
      );

      expect(screen.getByTestId("groupCount")).toHaveTextContent("2");
    });
  });

  describe("error state", () => {
    it("should always return null for error", () => {
      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("error")).toHaveTextContent("none");
    });
  });

  describe("pathname variations", () => {
    it("should handle /students path (non-auth)", () => {
      mockPathname.mockReturnValue("/students");
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("hasEducationalGroups")).toHaveTextContent(
        "true",
      );
    });

    it("should handle /settings path (non-auth)", () => {
      mockPathname.mockReturnValue("/settings");
      mockSupervisionContext.mockReturnValue({
        groups: [{ id: 1, name: "Group" }],
        isLoadingGroups: false,
        refresh: vi.fn(),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("hasEducationalGroups")).toHaveTextContent(
        "true",
      );
    });
  });
});
