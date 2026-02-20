import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { renderHook } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { TenantInfo } from "~/lib/tenant-api";

// Override the global mock from test/setup.ts to use the REAL implementation
vi.unmock("~/components/tenant/tenant-provider");

import {
  TenantProvider,
  useTenant,
  useTenantSlugSafe,
} from "./tenant-provider";

// ============================================================================
// Test Data
// ============================================================================

const mockTenant: TenantInfo = {
  tenantId: 1,
  slug: "demo-school",
  name: "Demo School",
  subdomain: "demo",
  organizationId: 10,
  organizationName: "Org A",
  settings: {},
};

// ============================================================================
// Tests
// ============================================================================

describe("TenantProvider", () => {
  it("renders children", () => {
    render(
      <TenantProvider tenantSlug="demo-school" tenant={mockTenant}>
        <div data-testid="child">Hello</div>
      </TenantProvider>,
    );

    expect(screen.getByTestId("child")).toHaveTextContent("Hello");
  });
});

describe("useTenant", () => {
  it("returns tenant context when inside provider", () => {
    function Wrapper({ children }: { children: React.ReactNode }) {
      return (
        <TenantProvider tenantSlug="demo-school" tenant={mockTenant}>
          {children}
        </TenantProvider>
      );
    }

    const { result } = renderHook(() => useTenant(), { wrapper: Wrapper });

    expect(result.current.tenantSlug).toBe("demo-school");
    expect(result.current.tenant).toEqual(mockTenant);
  });

  it("throws when used outside provider", () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(vi.fn());

    expect(() => {
      renderHook(() => useTenant());
    }).toThrow(
      "useTenant must be used within a TenantProvider (under [tenant] route)",
    );

    consoleError.mockRestore();
  });
});

describe("useTenantSlugSafe", () => {
  it("returns tenant slug when inside provider", () => {
    function Wrapper({ children }: { children: React.ReactNode }) {
      return (
        <TenantProvider tenantSlug="demo-school" tenant={mockTenant}>
          {children}
        </TenantProvider>
      );
    }

    const { result } = renderHook(() => useTenantSlugSafe(), {
      wrapper: Wrapper,
    });

    expect(result.current).toBe("demo-school");
  });

  it("returns null when outside provider", () => {
    const { result } = renderHook(() => useTenantSlugSafe());

    expect(result.current).toBeNull();
  });
});
