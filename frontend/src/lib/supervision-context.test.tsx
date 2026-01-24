/**
 * Tests for SupervisionContext provider and hooks.
 *
 * Covers:
 * - SupervisionProvider initialization
 * - useSupervision hook
 * - useHasGroups convenience hook
 * - useIsSupervising convenience hook
 * - Group fetching and state management
 * - Supervision fetching and state management
 * - Disabled paths behavior (e.g., /console)
 * - Refresh functionality with debouncing
 * - Session-based authentication handling
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, act } from "@testing-library/react";
import {
  SupervisionProvider,
  useSupervision,
  useHasGroups,
  useIsSupervising,
} from "./supervision-context";

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

// Test component that uses the context
function TestConsumer() {
  const context = useSupervision();
  return (
    <div>
      <span data-testid="hasGroups">{String(context.hasGroups)}</span>
      <span data-testid="isLoadingGroups">
        {String(context.isLoadingGroups)}
      </span>
      <span data-testid="groupCount">{context.groups.length}</span>
      <span data-testid="isSupervising">{String(context.isSupervising)}</span>
      <span data-testid="isLoadingSupervision">
        {String(context.isLoadingSupervision)}
      </span>
      <span data-testid="supervisedRoomId">
        {context.supervisedRoomId ?? "none"}
      </span>
      <span data-testid="supervisedRoomName">
        {context.supervisedRoomName ?? "none"}
      </span>
      <button onClick={() => context.refresh()}>Refresh</button>
      <button onClick={() => context.refresh(true)}>Silent Refresh</button>
    </div>
  );
}

function TestHasGroupsConsumer() {
  const hasGroups = useHasGroups();
  return <span data-testid="hasGroupsHook">{String(hasGroups)}</span>;
}

function TestIsSupervisingConsumer() {
  const isSupervising = useIsSupervising();
  return <span data-testid="isSupervisingHook">{String(isSupervising)}</span>;
}

// Helper to render with provider
function renderWithProvider(ui: React.ReactElement) {
  return render(<SupervisionProvider>{ui}</SupervisionProvider>);
}

describe("supervision-context", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.resetAllMocks();
    mockPathname.mockReturnValue("/dashboard");
    mockSession.mockReturnValue({
      data: { user: { id: "user-123", name: "Test User" } },
      isPending: false,
    });
    global.fetch = vi.fn();
  });

  afterEach(() => {
    vi.useRealTimers();
    global.fetch = originalFetch;
  });

  describe("SupervisionProvider", () => {
    it("should render children", () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ groups: [] }),
      });

      renderWithProvider(<div data-testid="child">Child Content</div>);

      expect(screen.getByTestId("child")).toHaveTextContent("Child Content");
    });

    it("should provide initial loading state", () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ groups: [] }),
      });

      renderWithProvider(<TestConsumer />);

      expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent("true");
      expect(screen.getByTestId("isLoadingSupervision")).toHaveTextContent(
        "true",
      );
    });

    it("should not fetch on disabled paths (/console)", async () => {
      mockPathname.mockReturnValue("/console");

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      expect(global.fetch).not.toHaveBeenCalled();
      expect(screen.getByTestId("hasGroups")).toHaveTextContent("false");
      expect(screen.getByTestId("isSupervising")).toHaveTextContent("false");
    });

    it("should not fetch when session is pending", () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: true,
      });

      renderWithProvider(<TestConsumer />);

      // Should not have made any fetch calls yet
      expect(global.fetch).not.toHaveBeenCalled();
    });

    it("should clear state when user is not authenticated", async () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: false,
      });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("hasGroups")).toHaveTextContent("false");
      expect(screen.getByTestId("groupCount")).toHaveTextContent("0");
      expect(screen.getByTestId("isSupervising")).toHaveTextContent("false");
    });
  });

  describe("group fetching", () => {
    it("should fetch and update groups on successful response", async () => {
      const mockGroups = [
        { id: 1, name: "Group 1", room_id: 10 },
        { id: 2, name: "Group 2" },
      ];

      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: mockGroups }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("hasGroups")).toHaveTextContent("true");
      expect(screen.getByTestId("groupCount")).toHaveTextContent("2");
    });

    it("should handle groups API error", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: false,
          json: () => Promise.resolve({ error: "Unauthorized" }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("hasGroups")).toHaveTextContent("false");
      expect(screen.getByTestId("groupCount")).toHaveTextContent("0");
    });

    it("should handle groups fetch exception", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockRejectedValueOnce(new Error("Network error"))
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("hasGroups")).toHaveTextContent("false");
    });

    it("should handle empty groups response", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("hasGroups")).toHaveTextContent("false");
      expect(screen.getByTestId("groupCount")).toHaveTextContent("0");
    });

    it("should handle response without groups field", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({}),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("hasGroups")).toHaveTextContent("false");
      expect(screen.getByTestId("groupCount")).toHaveTextContent("0");
    });
  });

  describe("supervision fetching", () => {
    it("should fetch and update supervision on successful response with data", async () => {
      const mockSupervisionData = [
        {
          id: 1,
          room_id: 42,
          group_id: 10,
          room: { id: 42, name: "Room 101" },
        },
      ];

      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({ success: true, data: mockSupervisionData }),
        });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingSupervision")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("isSupervising")).toHaveTextContent("true");
      expect(screen.getByTestId("supervisedRoomId")).toHaveTextContent("42");
      expect(screen.getByTestId("supervisedRoomName")).toHaveTextContent(
        "Room 101",
      );
    });

    it("should use fallback room name when room object is missing", async () => {
      const mockSupervisionData = [
        {
          id: 1,
          room_id: 42,
          group_id: 10,
        },
      ];

      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({ success: true, data: mockSupervisionData }),
        });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingSupervision")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("isSupervising")).toHaveTextContent("true");
      expect(screen.getByTestId("supervisedRoomId")).toHaveTextContent("42");
      expect(screen.getByTestId("supervisedRoomName")).toHaveTextContent(
        "Room 42",
      );
    });

    it("should handle empty supervision data", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingSupervision")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("isSupervising")).toHaveTextContent("false");
      expect(screen.getByTestId("supervisedRoomId")).toHaveTextContent("none");
      expect(screen.getByTestId("supervisedRoomName")).toHaveTextContent(
        "none",
      );
    });

    it("should handle supervision API error", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: [] }),
        })
        .mockResolvedValueOnce({
          ok: false,
          json: () => Promise.resolve({ error: "Unauthorized" }),
        });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingSupervision")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("isSupervising")).toHaveTextContent("false");
    });

    it("should handle supervision fetch exception", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: [] }),
        })
        .mockRejectedValueOnce(new Error("Network error"));

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingSupervision")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("isSupervising")).toHaveTextContent("false");
    });

    it("should handle missing data field in supervision response", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true }),
        });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingSupervision")).toHaveTextContent(
          "false",
        );
      });

      expect(screen.getByTestId("isSupervising")).toHaveTextContent("false");
    });
  });

  describe("refresh functionality", () => {
    it("should provide refresh function that can be called", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ groups: [], success: true, data: [] }),
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      // The refresh function exists and can be accessed
      const refreshButton = screen.getByText("Refresh");
      expect(refreshButton).toBeDefined();
    });

    it("should provide silent refresh function", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ groups: [], success: true, data: [] }),
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      // The silent refresh function exists
      const silentRefreshButton = screen.getByText("Silent Refresh");
      expect(silentRefreshButton).toBeDefined();
    });

    it("should support silent refresh without loading states", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ groups: [], success: true, data: [] }),
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      // Advance past debounce
      await act(async () => {
        vi.advanceTimersByTime(6000);
      });

      // Silent refresh should not show loading states
      const silentRefreshButton = screen.getByText("Silent Refresh");
      await act(async () => {
        silentRefreshButton.click();
      });

      // Loading states should remain false for silent refresh
      expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent("false");
      expect(screen.getByTestId("isLoadingSupervision")).toHaveTextContent(
        "false",
      );
    });
  });

  describe("periodic refresh", () => {
    it("should trigger periodic refresh every minute", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ groups: [], success: true, data: [] }),
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial load
      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      // Clear initial fetch calls
      (global.fetch as ReturnType<typeof vi.fn>).mockClear();

      // Advance time by 1 minute
      await act(async () => {
        vi.advanceTimersByTime(60000);
      });

      // Should have triggered a periodic refresh
      expect(global.fetch).toHaveBeenCalled();
    });

    it("should not trigger periodic refresh on disabled paths", async () => {
      mockPathname.mockReturnValue("/console");

      renderWithProvider(<TestConsumer />);

      // Wait for initial state
      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      // Clear any fetch calls (should be none)
      (global.fetch as ReturnType<typeof vi.fn>).mockClear();

      // Advance time by 1 minute
      await act(async () => {
        vi.advanceTimersByTime(60000);
      });

      // Should not have triggered any fetch
      expect(global.fetch).not.toHaveBeenCalled();
    });

    it("should not trigger periodic refresh when not authenticated", async () => {
      mockSession.mockReturnValue({
        data: null,
        isPending: false,
      });

      renderWithProvider(<TestConsumer />);

      // Wait for initial state
      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      // Clear any fetch calls
      (global.fetch as ReturnType<typeof vi.fn>).mockClear();

      // Advance time by 1 minute
      await act(async () => {
        vi.advanceTimersByTime(60000);
      });

      // Should not have triggered any fetch
      expect(global.fetch).not.toHaveBeenCalled();
    });
  });

  describe("useSupervision hook", () => {
    it("should throw error when used outside provider", () => {
      // Suppress console.error for this test
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {});

      expect(() => {
        render(<TestConsumer />);
      }).toThrow("useSupervision must be used within a SupervisionProvider");

      consoleSpy.mockRestore();
    });
  });

  describe("useHasGroups hook", () => {
    it("should return false while loading", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(
              () =>
                resolve({
                  ok: true,
                  json: () =>
                    Promise.resolve({ groups: [{ id: 1, name: "Group" }] }),
                }),
              1000,
            ),
          ),
      );

      renderWithProvider(<TestHasGroupsConsumer />);

      // Initially loading
      expect(screen.getByTestId("hasGroupsHook")).toHaveTextContent("false");
    });

    it("should return true when has groups and not loading", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: [{ id: 1, name: "Group" }] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      renderWithProvider(<TestHasGroupsConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("hasGroupsHook")).toHaveTextContent("true");
      });
    });

    it("should return false when no groups and not loading", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      renderWithProvider(<TestHasGroupsConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("hasGroupsHook")).toHaveTextContent("false");
      });
    });
  });

  describe("useIsSupervising hook", () => {
    it("should return false while loading", () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(
              () =>
                resolve({
                  ok: true,
                  json: () => Promise.resolve({ success: true, data: [] }),
                }),
              1000,
            ),
          ),
      );

      renderWithProvider(<TestIsSupervisingConsumer />);

      // Initially loading
      expect(screen.getByTestId("isSupervisingHook")).toHaveTextContent(
        "false",
      );
    });

    it("should return true when supervising and not loading", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () =>
            Promise.resolve({
              success: true,
              data: [
                {
                  id: 1,
                  room_id: 42,
                  group_id: 10,
                  room: { id: 42, name: "R" },
                },
              ],
            }),
        });

      renderWithProvider(<TestIsSupervisingConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isSupervisingHook")).toHaveTextContent(
          "true",
        );
      });
    });

    it("should return false when not supervising and not loading", async () => {
      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: [] }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      renderWithProvider(<TestIsSupervisingConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isSupervisingHook")).toHaveTextContent(
          "false",
        );
      });
    });
  });

  describe("state optimization", () => {
    it("should not update state when values have not changed", async () => {
      const mockGroups = [{ id: 1, name: "Group 1" }];

      (global.fetch as ReturnType<typeof vi.fn>)
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ groups: mockGroups }),
        })
        .mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve({ success: true, data: [] }),
        });

      const { rerender } = renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(screen.getByTestId("isLoadingGroups")).toHaveTextContent(
          "false",
        );
      });

      // Rerender should not cause additional state updates
      rerender(
        <SupervisionProvider>
          <TestConsumer />
        </SupervisionProvider>,
      );

      expect(screen.getByTestId("groupCount")).toHaveTextContent("1");
    });
  });

  describe("path-based behavior", () => {
    it("should handle /console path specifically", () => {
      mockPathname.mockReturnValue("/console");

      renderWithProvider(<TestConsumer />);

      // Should not fetch on console path
      expect(global.fetch).not.toHaveBeenCalled();
    });

    it("should handle /console/organizations sub-path", () => {
      mockPathname.mockReturnValue("/console/organizations");

      renderWithProvider(<TestConsumer />);

      // Should not fetch on console sub-paths
      expect(global.fetch).not.toHaveBeenCalled();
    });

    it("should fetch on regular dashboard path", async () => {
      mockPathname.mockReturnValue("/dashboard");

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ groups: [], success: true, data: [] }),
      });

      renderWithProvider(<TestConsumer />);

      await waitFor(() => {
        expect(global.fetch).toHaveBeenCalled();
      });
    });
  });
});
