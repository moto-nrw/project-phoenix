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
