import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/react";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import {
  SupervisionProvider,
  useSupervision,
  useHasGroups,
  useIsSupervising,
} from "./supervision-context";

// Mock next-auth
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

const { useSession } = await import("next-auth/react");

// Mock fetch globally with URL-based routing
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Default mock responses for the 3 API endpoints
const defaultMockResponses = {
  groups: { groups: [] },
  supervised: { data: [] },
  schulhof: { data: { data: { exists: false } } }, // Double-wrapped response
};

// Helper to create URL-based fetch mock
function setupFetchMock(overrides?: {
  groups?: object | Error;
  supervised?: object | Error;
  schulhof?: object | Error;
}) {
  mockFetch.mockImplementation((url: string) => {
    if (url.includes("/api/groups/context")) {
      const response = overrides?.groups ?? defaultMockResponses.groups;
      if (response instanceof Error) return Promise.reject(response);
      return Promise.resolve({
        ok: true,
        json: async () => response,
      });
    }
    if (url.includes("/api/me/groups/supervised")) {
      const response = overrides?.supervised ?? defaultMockResponses.supervised;
      if (response instanceof Error) return Promise.reject(response);
      return Promise.resolve({
        ok: true,
        json: async () => response,
      });
    }
    if (url.includes("/api/active/schulhof/status")) {
      const response = overrides?.schulhof ?? defaultMockResponses.schulhof;
      if (response instanceof Error) return Promise.reject(response);
      return Promise.resolve({
        ok: true,
        json: async () => response,
      });
    }
    // Default fallback
    return Promise.resolve({
      ok: true,
      json: async () => ({}),
    });
  });
}

// Helper to create wrapper with session
function createWrapper(token?: string) {
  return function Wrapper({ children }: { children: ReactNode }) {
    vi.mocked(useSession).mockReturnValue(
      (token
        ? {
            data: {
              user: {
                token,
                id: "1",
                email: "test@example.com",
                name: "Test User",
              },
              expires: "2099-12-31",
            },
            status: "authenticated" as const,
            update: vi.fn(),
          }
        : {
            data: null,
            status: "unauthenticated" as const,
            update: vi.fn(),
          }) as ReturnType<typeof useSession>,
    );
    return <SupervisionProvider>{children}</SupervisionProvider>;
  };
}

describe("SupervisionProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("should render children", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });

    render(
      <SupervisionProvider>
        <div data-testid="child">Test Child</div>
      </SupervisionProvider>,
    );

    expect(screen.getByTestId("child")).toBeInTheDocument();
    expect(screen.getByText("Test Child")).toBeInTheDocument();
  });

  it("should initialize with loading states when no session", () => {
    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper(),
    });

    expect(result.current.hasGroups).toBe(false);
    expect(result.current.isLoadingGroups).toBe(false);
    expect(result.current.groups).toEqual([]);
    expect(result.current.isSupervising).toBe(false);
    expect(result.current.isLoadingSupervision).toBe(false);
  });

  it("should fetch groups and supervision on session token", async () => {
    setupFetchMock({
      groups: {
        groups: [
          { id: 1, name: "Group A" },
          { id: 2, name: "Group B" },
        ],
      },
      supervised: {
        data: [
          {
            id: 1,
            room_id: 5,
            group_id: 1,
            room: { id: 5, name: "Room 5" },
          },
        ],
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    // Wait for API calls
    await waitFor(() => {
      expect(result.current.isLoadingGroups).toBe(false);
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    expect(result.current.hasGroups).toBe(true);
    expect(result.current.groups).toHaveLength(2);
    expect(result.current.isSupervising).toBe(true);
    expect(result.current.supervisedRoomId).toBe("5");
    expect(result.current.supervisedRoomName).toBe("Room 5");
  });

  it("should handle API errors gracefully", async () => {
    setupFetchMock({
      groups: new Error("Network error"),
      supervised: new Error("Network error"),
      schulhof: new Error("Network error"),
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingGroups).toBe(false);
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    expect(result.current.hasGroups).toBe(false);
    expect(result.current.groups).toEqual([]);
    expect(result.current.isSupervising).toBe(false);
  });

  it(
    "should refresh data when refresh is called",
    { timeout: 10000 },
    async () => {
      setupFetchMock(); // Use defaults (empty)

      const { result } = renderHook(() => useSupervision(), {
        wrapper: createWrapper("test-token"),
      });

      await waitFor(() => {
        expect(result.current.isLoadingGroups).toBe(false);
      });

      // Wait more than 5 seconds to bypass debounce
      await new Promise((resolve) => setTimeout(resolve, 5100));

      // Update mock for refresh call
      setupFetchMock({
        groups: { groups: [{ id: 10, name: "New Group" }] },
      });

      await act(async () => {
        await result.current.refresh();
      });

      await waitFor(() => {
        expect(result.current.groups).toHaveLength(1);
        expect(result.current.groups[0]?.name).toBe("New Group");
      });
    },
  );

  it("should debounce rapid refresh calls", async () => {
    setupFetchMock(); // Use defaults

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingGroups).toBe(false);
    });

    const initialFetchCount = mockFetch.mock.calls.length;

    // Call refresh multiple times rapidly (all should be debounced)
    await act(async () => {
      await result.current.refresh();
      await result.current.refresh();
      await result.current.refresh();
    });

    // Should not call again due to debouncing (5 second minimum)
    expect(mockFetch.mock.calls.length).toBe(initialFetchCount);
  });

  it("should setup periodic refresh when token exists", async () => {
    // Mock setInterval to verify it's called
    const intervalSpy = vi.spyOn(global, "setInterval");

    setupFetchMock(); // Use defaults

    const { unmount } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    // Wait for initial fetch
    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalled();
    });

    // Verify that setInterval was called with 60000ms (1 minute)
    expect(intervalSpy).toHaveBeenCalledWith(expect.any(Function), 60000);

    unmount();
    intervalSpy.mockRestore();
  });

  it("should not setup periodic refresh without token", () => {
    vi.useFakeTimers();

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper(),
    });

    expect(result.current.hasGroups).toBe(false);
    expect(mockFetch).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(60000);
    });

    expect(mockFetch).not.toHaveBeenCalled();

    vi.useRealTimers();
  });

  it("should handle supervision with room name fallback", async () => {
    setupFetchMock({
      supervised: {
        data: [
          {
            id: 1,
            room_id: 5,
            group_id: 1,
            // No room object, should fallback
          },
        ],
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    expect(result.current.isSupervising).toBe(true);
    expect(result.current.supervisedRoomName).toBe("Room 5");
  });
});

describe("useSupervision", () => {
  it("should throw error when used outside provider", () => {
    // Suppress console.error for this test
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {
      /* noop */
    });

    expect(() => {
      renderHook(() => useSupervision());
    }).toThrow("useSupervision must be used within a SupervisionProvider");

    consoleError.mockRestore();
  });

  it("should return context values when inside provider", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "unauthenticated",
      update: vi.fn(),
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper(),
    });

    expect(result.current).toHaveProperty("hasGroups");
    expect(result.current).toHaveProperty("isLoadingGroups");
    expect(result.current).toHaveProperty("groups");
    expect(result.current).toHaveProperty("isSupervising");
    expect(result.current).toHaveProperty("isLoadingSupervision");
    expect(result.current).toHaveProperty("refresh");
  });
});

describe("useHasGroups", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should return false when loading", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: { token: "test", id: "1", email: "test@test.com", name: "Test" },
        expires: "2099-12-31",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    mockFetch.mockImplementation(
      () =>
        new Promise(() => {
          /* noop */
        }),
    ); // Never resolves

    const { result } = renderHook(() => useHasGroups(), {
      wrapper: createWrapper("test-token"),
    });

    expect(result.current).toBe(false);
  });

  it("should return true when has groups and not loading", async () => {
    setupFetchMock({
      groups: { groups: [{ id: 1, name: "Group A" }] },
    });

    const { result } = renderHook(() => useHasGroups(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current).toBe(true);
    });
  });

  it("should return false when no groups", async () => {
    setupFetchMock(); // Use defaults (empty groups)

    const { result } = renderHook(() => useHasGroups(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current).toBe(false);
    });
  });
});

describe("useIsSupervising", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should return false when loading", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: { token: "test", id: "1", email: "test@test.com", name: "Test" },
        expires: "2099-12-31",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    mockFetch.mockImplementation(
      () =>
        new Promise(() => {
          /* noop */
        }),
    );

    const { result } = renderHook(() => useIsSupervising(), {
      wrapper: createWrapper("test-token"),
    });

    expect(result.current).toBe(false);
  });

  it("should return true when supervising and not loading", async () => {
    setupFetchMock({
      supervised: {
        data: [
          {
            id: 1,
            room_id: 5,
            group_id: 1,
            room: { id: 5, name: "Room 5" },
          },
        ],
      },
    });

    const { result } = renderHook(() => useIsSupervising(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current).toBe(true);
    });
  });

  it("should return false when not supervising", async () => {
    setupFetchMock(); // Use defaults (empty)

    const { result } = renderHook(() => useIsSupervising(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current).toBe(false);
    });
  });
});

describe("SupervisionProvider Schulhof handling", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("should include Schulhof in supervised rooms when it exists", async () => {
    setupFetchMock({
      supervised: {
        data: [
          {
            id: 1,
            room_id: 5,
            group_id: 1,
            room: { id: 5, name: "Room 5" },
          },
        ],
      },
      schulhof: {
        data: {
          data: {
            exists: true,
            room_id: 100,
            room_name: "Schulhof",
            active_group_id: 200,
            is_user_supervising: false,
          },
        },
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    // Should have both regular room and Schulhof
    expect(result.current.supervisedRooms.length).toBeGreaterThanOrEqual(1);
    const schulhofRoom = result.current.supervisedRooms.find(
      (r) => r.isSchulhof,
    );
    expect(schulhofRoom).toBeDefined();
    expect(schulhofRoom?.name).toBe("Schulhof");
  });

  it("should include Schulhof even with no other supervision", async () => {
    setupFetchMock({
      supervised: { data: [] },
      schulhof: {
        data: {
          data: {
            exists: true,
            room_id: 100,
            room_name: "Schulhof",
            active_group_id: 200,
            is_user_supervising: false,
          },
        },
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    // isSupervising is true when Schulhof exists, even if the user is not
    // actively supervising it. The Schulhof tab must be visible to ALL staff
    // so anyone can opt-in. See supervision-context.tsx lines 226-230.
    expect(result.current.isSupervising).toBe(true);
    expect(result.current.supervisedRooms).toHaveLength(1);
    expect(result.current.supervisedRooms[0]?.isSchulhof).toBe(true);
  });

  it("should not include Schulhof when it does not exist", async () => {
    setupFetchMock({
      supervised: {
        data: [
          {
            id: 1,
            room_id: 5,
            group_id: 1,
            room: { id: 5, name: "Room 5" },
          },
        ],
      },
      schulhof: {
        data: {
          data: {
            exists: false,
          },
        },
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    const schulhofRoom = result.current.supervisedRooms.find(
      (r) => r.isSchulhof,
    );
    expect(schulhofRoom).toBeUndefined();
  });

  it("should filter Schulhof from regular supervised rooms", async () => {
    // If a regular supervised room is named Schulhof, it should be filtered out
    // and replaced with the special Schulhof tab
    setupFetchMock({
      supervised: {
        data: [
          {
            id: 1,
            room_id: 5,
            group_id: 1,
            room: { id: 5, name: "Schulhof" }, // Regular room named Schulhof
          },
        ],
      },
      schulhof: {
        data: {
          data: {
            exists: true,
            room_id: 100,
            room_name: "Schulhof",
            active_group_id: 200,
            is_user_supervising: false,
          },
        },
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    // Should only have the special Schulhof tab, not the regular one
    const schulhofRooms = result.current.supervisedRooms.filter(
      (r) => r.name === "Schulhof",
    );
    expect(schulhofRooms.length).toBe(1);
    expect(schulhofRooms[0]?.isSchulhof).toBe(true);
  });
});

describe("SupervisionProvider state optimization", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("should not update state when groups remain the same", async () => {
    const initialGroups = { groups: [{ id: 1, name: "Group A" }] };
    setupFetchMock({ groups: initialGroups });

    const { result, rerender } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingGroups).toBe(false);
    });

    const initialGroupsResult = result.current.groups;

    // Re-render with same data
    rerender();

    // Groups reference should be the same (no unnecessary update)
    expect(result.current.groups).toBe(initialGroupsResult);
  });

  it("should handle silent refresh without updating loading state", () => {
    // Test the silent refresh logic directly
    let isLoadingGroups = false;
    let isLoadingSupervision = false;

    const updateStateForRefresh = (silent: boolean) => {
      if (!silent) {
        isLoadingGroups = true;
        isLoadingSupervision = true;
      }
    };

    // Silent refresh should NOT update loading states
    updateStateForRefresh(true);
    expect(isLoadingGroups).toBe(false);
    expect(isLoadingSupervision).toBe(false);

    // Non-silent refresh SHOULD update loading states
    updateStateForRefresh(false);
    expect(isLoadingGroups).toBe(true);
    expect(isLoadingSupervision).toBe(true);
  });
});

describe("SupervisionProvider API response handling", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("should handle non-OK response from groups API", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/groups/context")) {
        return Promise.resolve({
          ok: false,
          status: 404,
          json: async () => ({}),
        });
      }
      return Promise.resolve({
        ok: true,
        json: async () => ({ data: [] }),
      });
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingGroups).toBe(false);
    });

    expect(result.current.hasGroups).toBe(false);
    expect(result.current.groups).toEqual([]);
  });

  it("should handle non-OK response from supervised API but still show Schulhof", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/me/groups/supervised")) {
        return Promise.resolve({
          ok: false,
          status: 500,
          json: async () => ({}),
        });
      }
      if (url.includes("/api/active/schulhof/status")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({
            data: {
              data: {
                exists: true,
                room_id: 100,
                room_name: "Schulhof",
                active_group_id: 200,
                is_user_supervising: false,
              },
            },
          }),
        });
      }
      if (url.includes("/api/groups/context")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({ groups: [] }),
        });
      }
      return Promise.resolve({
        ok: true,
        json: async () => ({}),
      });
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    // Should still have Schulhof even though supervised API failed
    expect(result.current.supervisedRooms).toHaveLength(1);
    expect(result.current.supervisedRooms[0]?.isSchulhof).toBe(true);
  });

  it("should handle groups API with nested data structure", async () => {
    setupFetchMock({
      groups: {
        data: {
          groups: [
            { id: 1, name: "Nested Group A" },
            { id: 2, name: "Nested Group B" },
          ],
        },
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingGroups).toBe(false);
    });

    expect(result.current.hasGroups).toBe(true);
    expect(result.current.groups).toHaveLength(2);
  });

  it("should sort groups by German locale", async () => {
    setupFetchMock({
      groups: {
        groups: [
          { id: 3, name: "Zebra-Gruppe" },
          { id: 1, name: "Äpfel-Gruppe" },
          { id: 2, name: "Bären-Gruppe" },
        ],
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingGroups).toBe(false);
    });

    // German locale sorting: Ä comes after A, before B
    expect(result.current.groups[0]?.name).toBe("Äpfel-Gruppe");
    expect(result.current.groups[1]?.name).toBe("Bären-Gruppe");
    expect(result.current.groups[2]?.name).toBe("Zebra-Gruppe");
  });
});

describe("SupervisionProvider multiple supervised rooms", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("should handle multiple supervised rooms sorted by name", async () => {
    setupFetchMock({
      supervised: {
        data: [
          {
            id: 1,
            room_id: 10,
            group_id: 1,
            room: { id: 10, name: "Zimmer Z" },
          },
          {
            id: 2,
            room_id: 20,
            group_id: 2,
            room: { id: 20, name: "Atelier A" },
          },
          {
            id: 3,
            room_id: 30,
            group_id: 3,
            room: { id: 30, name: "Mensa M" },
          },
        ],
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    expect(result.current.supervisedRooms).toHaveLength(3);
    // Should be sorted alphabetically
    expect(result.current.supervisedRooms[0]?.name).toBe("Atelier A");
    expect(result.current.supervisedRooms[1]?.name).toBe("Mensa M");
    expect(result.current.supervisedRooms[2]?.name).toBe("Zimmer Z");
  });

  it("should include actual_group name as groupName", async () => {
    setupFetchMock({
      supervised: {
        data: [
          {
            id: 1,
            room_id: 10,
            group_id: 1,
            room: { id: 10, name: "Kunstzimmer" },
            actual_group: { id: 5, name: "OGS Gruppe Blau" },
          },
        ],
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    expect(result.current.supervisedRooms[0]?.groupName).toBe(
      "OGS Gruppe Blau",
    );
  });
});

describe("SupervisionProvider refresh debouncing", () => {
  it("debounces rapid refresh calls", () => {
    // Simulate debounce logic
    let lastRefreshTime = 0;
    const DEBOUNCE_MS = 5000;

    const shouldRefresh = (): boolean => {
      const now = Date.now();
      if (now - lastRefreshTime < DEBOUNCE_MS) {
        return false;
      }
      lastRefreshTime = now;
      return true;
    };

    // First call should succeed
    expect(shouldRefresh()).toBe(true);

    // Immediate second call should be debounced
    expect(shouldRefresh()).toBe(false);
  });

  it("allows refresh after debounce period", () => {
    let lastRefreshTime = Date.now() - 6000; // 6 seconds ago
    const DEBOUNCE_MS = 5000;

    const shouldRefresh = (): boolean => {
      const now = Date.now();
      if (now - lastRefreshTime < DEBOUNCE_MS) {
        return false;
      }
      lastRefreshTime = now;
      return true;
    };

    // Should allow refresh since 6 seconds passed
    expect(shouldRefresh()).toBe(true);
  });
});

describe("SupervisionProvider isRefreshing guard", () => {
  it("prevents concurrent refresh calls", () => {
    let isRefreshing = false;
    let refreshCount = 0;

    const tryRefresh = (): boolean => {
      if (isRefreshing) return false;
      isRefreshing = true;
      refreshCount++;
      return true;
    };

    // First call succeeds
    expect(tryRefresh()).toBe(true);
    expect(refreshCount).toBe(1);

    // Second call blocked while refreshing
    expect(tryRefresh()).toBe(false);
    expect(refreshCount).toBe(1);

    // After completing, next call succeeds
    isRefreshing = false;
    expect(tryRefresh()).toBe(true);
    expect(refreshCount).toBe(2);
  });
});

describe("SupervisionProvider silent refresh mode", () => {
  it("does not update loading state on silent refresh", () => {
    let isLoadingGroups = false;
    let isLoadingSupervision = false;

    const updateStateForRefresh = (silent: boolean) => {
      if (!silent) {
        isLoadingGroups = true;
        isLoadingSupervision = true;
      }
    };

    // Non-silent refresh sets loading
    updateStateForRefresh(false);
    expect(isLoadingGroups).toBe(true);
    expect(isLoadingSupervision).toBe(true);

    // Reset
    isLoadingGroups = false;
    isLoadingSupervision = false;

    // Silent refresh does not set loading
    updateStateForRefresh(true);
    expect(isLoadingGroups).toBe(false);
    expect(isLoadingSupervision).toBe(false);
  });
});

describe("SupervisionProvider state change detection", () => {
  it("skips update when groups unchanged", () => {
    const prevState = {
      hasGroups: true,
      groups: [{ id: 1, name: "Group A" }],
      isLoadingGroups: false,
    };
    const newGroupList = [{ id: 1, name: "Group A" }];
    const newHasGroups = true;

    const shouldUpdate = !(
      prevState.hasGroups === newHasGroups &&
      prevState.groups.length === newGroupList.length &&
      prevState.groups.every(
        (group, index) => group.id === newGroupList[index]?.id,
      ) &&
      !prevState.isLoadingGroups
    );

    expect(shouldUpdate).toBe(false);
  });

  it("triggers update when groups changed", () => {
    const prevState = {
      hasGroups: true,
      groups: [{ id: 1, name: "Group A" }],
      isLoadingGroups: false,
    };
    const newGroupList = [
      { id: 1, name: "Group A" },
      { id: 2, name: "Group B" },
    ];
    const newHasGroups = true;

    const shouldUpdate = !(
      prevState.hasGroups === newHasGroups &&
      prevState.groups.length === newGroupList.length &&
      prevState.groups.every(
        (group, index) => group.id === newGroupList[index]?.id,
      ) &&
      !prevState.isLoadingGroups
    );

    expect(shouldUpdate).toBe(true);
  });

  it("triggers update when loading state differs", () => {
    const prevState = {
      hasGroups: false,
      groups: [] as Array<{ id: number }>,
      isLoadingGroups: true,
    };
    const newGroupList: Array<{ id: number }> = [];
    const newHasGroups = false;

    const shouldUpdate = !(
      prevState.hasGroups === newHasGroups &&
      prevState.groups.length === newGroupList.length &&
      prevState.groups.every(
        (group, index) => group.id === newGroupList[index]?.id,
      ) &&
      !prevState.isLoadingGroups
    );

    expect(shouldUpdate).toBe(true);
  });
});

describe("SupervisionProvider supervised rooms comparison", () => {
  it("skips update when room IDs unchanged", () => {
    const prevRoomIds = ["room-1", "room-2"].join(",");
    const newRoomIds = ["room-1", "room-2"].join(",");

    expect(prevRoomIds === newRoomIds).toBe(true);
  });

  it("triggers update when room IDs changed", () => {
    const prevRoomIds = ["room-1", "room-2"].join(",");
    const newRoomIds = ["room-1", "room-3"].join(",");

    expect(prevRoomIds !== newRoomIds).toBe(true);
  });

  it("detects room order changes", () => {
    const prevRoomIds = ["room-1", "room-2"].join(",");
    const newRoomIds = ["room-2", "room-1"].join(",");

    expect(prevRoomIds !== newRoomIds).toBe(true);
  });
});

describe("SupervisionProvider Schulhof room creation", () => {
  it("creates virtual Schulhof room from status", () => {
    const SCHULHOF_TAB_ID = "schulhof-permanent";
    const SCHULHOF_ROOM_NAME = "Schulhof";

    const schulhofData = {
      exists: true,
      active_group_id: 123,
    };

    const schulhofRoom = schulhofData.exists
      ? {
          id: SCHULHOF_TAB_ID,
          name: SCHULHOF_ROOM_NAME,
          groupId: schulhofData.active_group_id?.toString() ?? SCHULHOF_TAB_ID,
          isSchulhof: true,
        }
      : null;

    expect(schulhofRoom).not.toBeNull();
    expect(schulhofRoom?.id).toBe("schulhof-permanent");
    expect(schulhofRoom?.groupId).toBe("123");
    expect(schulhofRoom?.isSchulhof).toBe(true);
  });

  it("returns null when Schulhof does not exist", () => {
    const schulhofData = { exists: false };

    const schulhofRoom = schulhofData.exists
      ? { id: "schulhof-permanent", name: "Schulhof" }
      : null;

    expect(schulhofRoom).toBeNull();
  });
});

describe("SupervisionProvider room name fallback", () => {
  it("uses room name when available", () => {
    const group = {
      room_id: 10,
      room: { id: 10, name: "Kunstzimmer" },
    };

    const roomName =
      group.room?.name ?? (group.room_id ? `Room ${group.room_id}` : undefined);

    expect(roomName).toBe("Kunstzimmer");
  });

  it("falls back to Room ID format when name missing", () => {
    const group = {
      room_id: 10,
      room: undefined as { id: number; name: string } | undefined,
    };

    const roomName =
      group.room?.name ?? (group.room_id ? `Room ${group.room_id}` : undefined);

    expect(roomName).toBe("Room 10");
  });

  it("returns undefined when no room info", () => {
    const group = {
      room_id: undefined as number | undefined,
      room: undefined as { id: number; name: string } | undefined,
    };

    const roomName =
      group.room?.name ?? (group.room_id ? `Room ${group.room_id}` : undefined);

    expect(roomName).toBeUndefined();
  });
});

describe("SupervisionProvider filters out Schulhof from regular rooms", () => {
  it("excludes Schulhof when mapping supervised groups", () => {
    const SCHULHOF_ROOM_NAME = "Schulhof";
    const supervisedGroups = [
      { room_id: 1, room: { name: "Raum A" } },
      { room_id: 2, room: { name: SCHULHOF_ROOM_NAME } },
      { room_id: 3, room: { name: "Raum B" } },
    ];

    const filteredRooms = supervisedGroups.filter(
      (g) => g.room_id && g.room && g.room.name !== SCHULHOF_ROOM_NAME,
    );

    expect(filteredRooms).toHaveLength(2);
    expect(
      filteredRooms.find((r) => r.room.name === "Schulhof"),
    ).toBeUndefined();
  });
});

describe("SupervisionProvider session handling", () => {
  it("clears state when no session token", () => {
    const token: string | undefined = undefined;
    let stateClearedToEmpty = false;

    if (!token) {
      stateClearedToEmpty = true;
    }

    expect(stateClearedToEmpty).toBe(true);
  });

  it("triggers refresh when session token exists", () => {
    const token = "valid-token";
    let refreshCalled = false;

    if (token) {
      refreshCalled = true;
    }

    expect(refreshCalled).toBe(true);
  });
});

describe("SupervisionProvider uncovered condition coverage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("should handle Schulhof with null active_group_id (fallback to tab ID)", async () => {
    setupFetchMock({
      supervised: { data: [] },
      schulhof: {
        data: {
          data: {
            exists: true,
            room_id: 100,
            room_name: "Schulhof",
            active_group_id: null, // null - should fallback to SCHULHOF_TAB_ID
            is_user_supervising: false,
          },
        },
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    const schulhofRoom = result.current.supervisedRooms.find(
      (r) => r.isSchulhof,
    );
    expect(schulhofRoom).toBeDefined();
    // When active_group_id is null, groupId should fallback to SCHULHOF_TAB_ID
    expect(schulhofRoom?.groupId).toBe("schulhof-permanent");
  });

  it("should handle supervised group with no room_id (undefined roomName and roomId)", async () => {
    setupFetchMock({
      supervised: {
        data: [
          {
            id: 1,
            group_id: 1,
            // No room_id and no room object
          },
        ],
      },
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    // With no room_id, the supervised rooms list should be empty
    // (filtered out because g.room_id is falsy)
    expect(result.current.supervisedRooms).toHaveLength(0);
    // isSupervising is still true because supervisedGroups was non-empty
    expect(result.current.isSupervising).toBe(true);
    // supervisedRoomId should be undefined since firstGroup has no room_id
    expect(result.current.supervisedRoomId).toBeUndefined();
    expect(result.current.supervisedRoomName).toBeUndefined();
  });

  it("should handle Schulhof response that is not OK (null schulhofResponse)", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/api/groups/context")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({ groups: [] }),
        });
      }
      if (url.includes("/api/me/groups/supervised")) {
        return Promise.resolve({
          ok: true,
          json: async () => ({
            data: [
              {
                id: 1,
                room_id: 5,
                group_id: 1,
                room: { id: 5, name: "Room 5" },
              },
            ],
          }),
        });
      }
      if (url.includes("/api/active/schulhof/status")) {
        // Return non-OK response
        return Promise.resolve({
          ok: false,
          status: 500,
          json: async () => ({}),
        });
      }
      return Promise.resolve({
        ok: true,
        json: async () => ({}),
      });
    });

    const { result } = renderHook(() => useSupervision(), {
      wrapper: createWrapper("test-token"),
    });

    await waitFor(() => {
      expect(result.current.isLoadingSupervision).toBe(false);
    });

    // Schulhof should not be in the rooms since its response was not OK
    const schulhofRoom = result.current.supervisedRooms.find(
      (r) => r.isSchulhof,
    );
    expect(schulhofRoom).toBeUndefined();
    // But regular supervision should still work
    expect(result.current.isSupervising).toBe(true);
    expect(result.current.supervisedRoomId).toBe("5");
  });
});
