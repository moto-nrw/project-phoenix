import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { OperatorAuthProvider, useOperatorAuth } from "./auth-context";
import type { ReactNode } from "react";

// Mock useRouter
const mockPush = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    refresh: vi.fn(),
    back: vi.fn(),
  }),
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
        "Failed to check operator auth:",
        expect.any(Error),
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

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "Logout error:",
        expect.any(Error),
      );

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
});
