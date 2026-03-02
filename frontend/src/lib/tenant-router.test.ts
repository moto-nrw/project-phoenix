import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/react";

// ============================================================================
// Mocks
// ============================================================================

const { mockUseTenant } = vi.hoisted(() => ({
  mockUseTenant: vi.fn(),
}));

const {
  mockPush,
  mockReplace,
  mockBack,
  mockForward,
  mockRefresh,
  mockPrefetch,
} = vi.hoisted(() => ({
  mockPush: vi.fn(),
  mockReplace: vi.fn(),
  mockBack: vi.fn(),
  mockForward: vi.fn(),
  mockRefresh: vi.fn(),
  mockPrefetch: vi.fn(),
}));

vi.mock("~/components/tenant/tenant-provider", () => ({
  useTenant: mockUseTenant,
}));

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({
    push: mockPush,
    replace: mockReplace,
    back: mockBack,
    forward: mockForward,
    refresh: mockRefresh,
    prefetch: mockPrefetch,
  })),
}));

import { useTenantRouter } from "./tenant-router";

// ============================================================================
// Tests
// ============================================================================

describe("useTenantRouter", () => {
  let originalWindow: typeof globalThis.window;

  beforeEach(() => {
    vi.clearAllMocks();
    originalWindow = globalThis.window;
  });

  afterEach(() => {
    // Restore window if overridden
    if (globalThis.window !== originalWindow) {
      Object.defineProperty(globalThis, "window", {
        value: originalWindow,
        writable: true,
      });
    }
  });

  describe("path mode (no subdomain)", () => {
    beforeEach(() => {
      mockUseTenant.mockReturnValue({ tenantSlug: "school-a" });
      // Hostname does NOT start with the slug — path mode
      Object.defineProperty(window, "location", {
        value: { ...window.location, hostname: "localhost" },
        writable: true,
      });
    });

    it("prefixes push with tenant slug", () => {
      const { result } = renderHook(() => useTenantRouter());
      result.current.push("/dashboard");
      expect(mockPush).toHaveBeenCalledWith("/school-a/dashboard");
    });

    it("prefixes replace with tenant slug", () => {
      const { result } = renderHook(() => useTenantRouter());
      result.current.replace("/settings");
      expect(mockReplace).toHaveBeenCalledWith("/school-a/settings");
    });

    it("prefixes prefetch with tenant slug", () => {
      const { result } = renderHook(() => useTenantRouter());
      result.current.prefetch("/rooms");
      expect(mockPrefetch).toHaveBeenCalledWith("/school-a/rooms");
    });

    it("delegates back without prefix", () => {
      const { result } = renderHook(() => useTenantRouter());
      result.current.back();
      expect(mockBack).toHaveBeenCalled();
    });

    it("delegates forward without prefix", () => {
      const { result } = renderHook(() => useTenantRouter());
      result.current.forward();
      expect(mockForward).toHaveBeenCalled();
    });

    it("delegates refresh without prefix", () => {
      const { result } = renderHook(() => useTenantRouter());
      result.current.refresh();
      expect(mockRefresh).toHaveBeenCalled();
    });
  });

  describe("subdomain mode", () => {
    beforeEach(() => {
      mockUseTenant.mockReturnValue({ tenantSlug: "school-a" });
      // Hostname starts with the slug — subdomain mode
      Object.defineProperty(window, "location", {
        value: { ...window.location, hostname: "school-a.localhost" },
        writable: true,
      });
    });

    it("push uses bare path without slug prefix", () => {
      const { result } = renderHook(() => useTenantRouter());
      result.current.push("/dashboard");
      expect(mockPush).toHaveBeenCalledWith("/dashboard");
    });

    it("replace uses bare path without slug prefix", () => {
      const { result } = renderHook(() => useTenantRouter());
      result.current.replace("/settings");
      expect(mockReplace).toHaveBeenCalledWith("/settings");
    });

    it("prefetch uses bare path without slug prefix", () => {
      const { result } = renderHook(() => useTenantRouter());
      result.current.prefetch("/rooms");
      expect(mockPrefetch).toHaveBeenCalledWith("/rooms");
    });
  });
});
