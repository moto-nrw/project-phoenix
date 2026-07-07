import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";

// ============================================================================
// Mocks
// ============================================================================

const { mockUseTenantSlugSafe } = vi.hoisted(() => ({
  mockUseTenantSlugSafe: vi.fn((): string | null => null),
}));
const { mockUseTenantRoutingModeSafe } = vi.hoisted(() => ({
  mockUseTenantRoutingModeSafe: vi.fn((): "path" | "subdomain" => "path"),
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

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: mockUseTenantSlugSafe,
  useTenantRoutingModeSafe: mockUseTenantRoutingModeSafe,
  useNFCEnabled: vi.fn(() => true),
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
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseTenantRoutingModeSafe.mockReturnValue("path");
  });

  describe("path mode (no subdomain)", () => {
    beforeEach(() => {
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      mockUseTenantRoutingModeSafe.mockReturnValue("path");
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
      mockUseTenantSlugSafe.mockReturnValue("school-a");
      mockUseTenantRoutingModeSafe.mockReturnValue("subdomain");
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
