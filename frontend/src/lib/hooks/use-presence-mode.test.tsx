import { renderHook } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

// Opt out of the global tenant-provider mock from src/test/setup.ts — that
// mock's TenantProvider passes children through without a real context,
// which breaks hooks that subscribe to TenantContext directly.
vi.unmock("~/components/tenant/tenant-provider");

import { TenantProvider } from "~/components/tenant/tenant-provider";
import type { TenantInfo } from "~/lib/tenant-api";
import { usePresenceMode } from "./use-presence-mode";

function makeTenant(
  presenceMode: "detailed" | "binary" = "detailed",
): TenantInfo {
  return {
    tenantId: 1,
    slug: "demo",
    name: "Demo",
    subdomain: "demo",
    organizationId: 10,
    organizationName: "Org",
    settings: {},
    presenceMode,
  };
}

describe("usePresenceMode", () => {
  it("returns the tenant's presenceMode when inside a TenantProvider", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantProvider tenantSlug="demo" tenant={makeTenant("binary")}>
        {children}
      </TenantProvider>
    );
    const { result } = renderHook(() => usePresenceMode(), { wrapper });
    expect(result.current).toBe("binary");
  });

  it("returns 'detailed' outside any TenantProvider (safe default)", () => {
    const { result } = renderHook(() => usePresenceMode());
    expect(result.current).toBe("detailed");
  });

  it("returns 'detailed' when tenant hasn't resolved yet", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantProvider tenantSlug="demo" tenant={null}>
        {children}
      </TenantProvider>
    );
    const { result } = renderHook(() => usePresenceMode(), { wrapper });
    expect(result.current).toBe("detailed");
  });
});
