import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { OperatorAuthProvider, useOperatorAuth } from "./auth-context";
import type { ReactNode } from "react";

// Mock next/navigation
const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    refresh: vi.fn(),
    back: vi.fn(),
  }),
  usePathname: () => "/operator/suggestions",
}));

// Mock global fetch
const mockFetch = vi.fn();
global.fetch = mockFetch as typeof fetch;

describe("OperatorAuthProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch.mockClear();
  });

  const wrapper = ({ children }: { children: ReactNode }) => (
    <OperatorAuthProvider>{children}</OperatorAuthProvider>
  );

  describe("initial auth check", () => {
    it("checks authentication status on mount", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          id: "1",
          displayName: "Admin User",
          email: "admin@example.com",
        }),
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      expect(result.current.isLoading).toBe(true);

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(global.fetch).toHaveBeenCalledWith("/api/operator/me");
      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.operator).toEqual({
        id: "1",
        displayName: "Admin User",
        email: "admin@example.com",
      });
    });

    it("sets operator to null when auth check fails", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.isAuthenticated).toBe(false);
      expect(result.current.operator).toBeNull();
    });

    it("recovers session via refresh when access token is expired on mount", async () => {
      // /me returns 401 (access token expired)
      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });
      // refresh succeeds
      mockFetch.mockResolvedValueOnce({ ok: true });
      // retry /me succeeds with fresh access token
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          id: "1",
          displayName: "Recovered User",
          email: "recovered@example.com",
        }),
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.operator?.displayName).toBe("Recovered User");
      expect(mockFetch).toHaveBeenCalledWith("/api/operator/refresh", {
        method: "POST",
      });
    });

    it("clears operator when both access token and refresh token are expired on mount", async () => {
      // /me returns 401
      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });
      // refresh also fails
      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.isAuthenticated).toBe(false);
      expect(result.current.operator).toBeNull();
    });

    it("clears operator when refresh succeeds but retry /me still fails", async () => {
      // /me returns 401
      mockFetch.mockResolvedValueOnce({ ok: false, status: 401 });
      // refresh succeeds
      mockFetch.mockResolvedValueOnce({ ok: true });
      // retry /me still fails (e.g. backend issue)
      mockFetch.mockResolvedValueOnce({ ok: false, status: 500 });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.isAuthenticated).toBe(false);
      expect(result.current.operator).toBeNull();
    });

    it("handles network errors during auth check", async () => {
      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {
          // noop
        });
      mockFetch.mockRejectedValueOnce(new Error("Network error"));

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "operator_auth_check_failed",
        { error: "Network error" },
      );
      expect(result.current.isAuthenticated).toBe(false);

      consoleErrorSpy.mockRestore();
    });
  });

  describe("login", () => {
    it("logs in successfully and redirects", async () => {
      mockFetch
        .mockResolvedValueOnce({ ok: false }) // Initial auth check
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            success: true,
            operator: {
              id: "2",
              displayName: "Test Operator",
              email: "test@example.com",
            },
          }),
        });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      await result.current.login("test@example.com", "password123");

      expect(global.fetch).toHaveBeenCalledWith("/api/operator/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: "test@example.com",
          password: "password123",
        }),
      });

      await waitFor(() => {
        expect(result.current.operator).toEqual({
          id: "2",
          displayName: "Test Operator",
          email: "test@example.com",
        });
      });

      expect(mockPush).toHaveBeenCalledWith("/operator/suggestions");
    });

    it("throws error when login fails", async () => {
      mockFetch.mockResolvedValueOnce({ ok: false }).mockResolvedValueOnce({
        ok: false,
        json: async () => ({ error: "Invalid credentials" }),
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      await expect(
        result.current.login("wrong@example.com", "wrongpass"),
      ).rejects.toThrow("Invalid credentials");
    });

    it("uses fallback error message when no error provided", async () => {
      mockFetch.mockResolvedValueOnce({ ok: false }).mockResolvedValueOnce({
        ok: false,
        json: async () => ({}),
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      await expect(
        result.current.login("test@example.com", "password"),
      ).rejects.toThrow("Login failed");
    });
  });

  describe("logout", () => {
    it("logs out successfully and redirects", async () => {
      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            id: "1",
            displayName: "User",
            email: "user@example.com",
          }),
        })
        .mockResolvedValueOnce({ ok: true }); // logout call

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.isAuthenticated).toBe(true);
      });

      await result.current.logout();

      expect(global.fetch).toHaveBeenCalledWith("/api/operator/logout", {
        method: "POST",
      });

      await waitFor(() => {
        expect(result.current.operator).toBeNull();
      });

      expect(mockPush).toHaveBeenCalledWith("/operator/login");
    });

    it("clears operator state even if logout request fails", async () => {
      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {
          // noop
        });

      mockFetch
        .mockResolvedValueOnce({
          ok: true,
          json: async () => ({
            id: "1",
            displayName: "User",
            email: "user@example.com",
          }),
        })
        .mockRejectedValueOnce(new Error("Network error"));

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.isAuthenticated).toBe(true);
      });

      await result.current.logout();

      expect(consoleErrorSpy).toHaveBeenCalledWith("operator_logout_error", {
        error: "Network error",
      });

      await waitFor(() => {
        expect(result.current.operator).toBeNull();
      });

      expect(mockPush).toHaveBeenCalledWith("/operator/login");

      consoleErrorSpy.mockRestore();
    });
  });

  describe("updateOperator", () => {
    it("updates operator display name", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          id: "1",
          displayName: "Original Name",
          email: "user@example.com",
        }),
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.operator?.displayName).toBe("Original Name");
      });

      result.current.updateOperator({ displayName: "Updated Name" });

      await waitFor(() => {
        expect(result.current.operator?.displayName).toBe("Updated Name");
      });

      expect(result.current.operator?.id).toBe("1");
      expect(result.current.operator?.email).toBe("user@example.com");
    });

    it("updates operator email", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          id: "1",
          displayName: "User",
          email: "old@example.com",
        }),
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.operator?.email).toBe("old@example.com");
      });

      result.current.updateOperator({ email: "new@example.com" });

      await waitFor(() => {
        expect(result.current.operator?.email).toBe("new@example.com");
      });
    });

    it("does not update when operator is null", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await waitFor(() => {
        expect(result.current.operator).toBeNull();
      });

      result.current.updateOperator({ displayName: "Should not apply" });

      expect(result.current.operator).toBeNull();
    });
  });

  describe("useOperatorAuth hook", () => {
    it("throws error when used outside provider", () => {
      expect(() => {
        renderHook(() => useOperatorAuth());
      }).toThrow("useOperatorAuth must be used within an OperatorAuthProvider");
    });
  });

  describe("token refresh polling", () => {
    beforeEach(() => {
      vi.useFakeTimers({ shouldAdvanceTime: true });
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it("calls refresh endpoint every 4 minutes when authenticated", async () => {
      // Initial auth check succeeds
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          id: "1",
          displayName: "Test",
          email: "test@example.com",
        }),
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      // Flush initial auth check (avoid runAllTimersAsync with setInterval)
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });
      expect(result.current.isAuthenticated).toBe(true);

      // Mock refresh response
      mockFetch.mockResolvedValue({ ok: true });

      // Advance 4 minutes
      await act(async () => {
        await vi.advanceTimersByTimeAsync(4 * 60 * 1000);
      });

      // Should have called refresh
      expect(mockFetch).toHaveBeenCalledWith("/api/operator/refresh", {
        method: "POST",
      });
    });

    it("does not poll when not authenticated", async () => {
      mockFetch.mockResolvedValueOnce({ ok: false });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await vi.runAllTimersAsync();
      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      mockFetch.mockClear();

      await vi.advanceTimersByTimeAsync(4 * 60 * 1000);

      // Only the redirect may happen, but no refresh call
      const refreshCalls = mockFetch.mock.calls.filter(
        (call) => call[0] === "/api/operator/refresh",
      );
      expect(refreshCalls).toHaveLength(0);
    });

    it("clears operator on 401 refresh response", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          id: "1",
          displayName: "Test",
          email: "test@example.com",
        }),
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      await waitFor(() => {
        expect(result.current.isAuthenticated).toBe(true);
      });

      // Mock refresh returning 401
      mockFetch.mockResolvedValue({ ok: false, status: 401 });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(4 * 60 * 1000);
      });

      await waitFor(() => {
        expect(result.current.operator).toBeNull();
      });
    });

    it("does not force logout on 5xx refresh response", async () => {
      const consoleWarnSpy = vi
        .spyOn(console, "warn")
        .mockImplementation(() => {
          // noop
        });

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          id: "1",
          displayName: "Test",
          email: "test@example.com",
        }),
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      await waitFor(() => {
        expect(result.current.isAuthenticated).toBe(true);
      });

      // Mock refresh returning 500 (transient server error)
      mockFetch.mockResolvedValue({ ok: false, status: 500 });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(4 * 60 * 1000);
      });

      // Operator should still be authenticated — 5xx is transient
      expect(result.current.isAuthenticated).toBe(true);
      expect(result.current.operator?.displayName).toBe("Test");

      consoleWarnSpy.mockRestore();
    });

    it("forces logout on 403 refresh response (operator deactivated)", async () => {
      const consoleWarnSpy = vi
        .spyOn(console, "warn")
        .mockImplementation(() => {
          // noop
        });

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          id: "1",
          displayName: "Test",
          email: "test@example.com",
        }),
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      await waitFor(() => {
        expect(result.current.isAuthenticated).toBe(true);
      });

      // Mock refresh returning 403 (operator deactivated)
      mockFetch.mockResolvedValue({ ok: false, status: 403 });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(4 * 60 * 1000);
      });

      await waitFor(() => {
        expect(result.current.operator).toBeNull();
      });

      consoleWarnSpy.mockRestore();
    });

    it("logs error on refresh network failure", async () => {
      const consoleErrorSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => {
          // noop
        });

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          id: "1",
          displayName: "Test",
          email: "test@example.com",
        }),
      });

      const { result } = renderHook(() => useOperatorAuth(), { wrapper });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      await waitFor(() => {
        expect(result.current.isAuthenticated).toBe(true);
      });

      // Mock refresh throwing
      mockFetch.mockRejectedValue(new Error("Network failure"));

      await act(async () => {
        await vi.advanceTimersByTimeAsync(4 * 60 * 1000);
      });

      await waitFor(() => {
        expect(consoleErrorSpy).toHaveBeenCalledWith(
          "operator_token_refresh_error",
          { error: "Network failure" },
        );
      });

      consoleErrorSpy.mockRestore();
    });

    it("cleans up interval on unmount", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({
          id: "1",
          displayName: "Test",
          email: "test@example.com",
        }),
      });

      const { result, unmount } = renderHook(() => useOperatorAuth(), {
        wrapper,
      });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      await waitFor(() => {
        expect(result.current.isAuthenticated).toBe(true);
      });

      mockFetch.mockClear();
      unmount();

      await vi.advanceTimersByTimeAsync(4 * 60 * 1000);

      const refreshCalls = mockFetch.mock.calls.filter(
        (call) => call[0] === "/api/operator/refresh",
      );
      expect(refreshCalls).toHaveLength(0);
    });
  });
});
