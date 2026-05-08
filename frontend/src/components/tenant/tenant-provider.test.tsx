import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { renderHook } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { TenantInfo } from "~/lib/tenant-api";

// Override the global mock from test/setup.ts to use the REAL implementation
vi.unmock("~/components/tenant/tenant-provider");

// Hoisted shared state for the cross-tab subscription test below.
const { broadcastHandlers, mockResolveTenant } = vi.hoisted(() => ({
  broadcastHandlers: new Set<() => void>(),
  mockResolveTenant: vi.fn(),
}));

vi.mock("~/lib/settings-broadcast", () => ({
  subscribeSettingsChanged: (handler: () => void) => {
    broadcastHandlers.add(handler);
    return () => broadcastHandlers.delete(handler);
  },
}));

vi.mock("~/lib/tenant-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/tenant-api")>();
  return {
    ...actual,
    resolveTenant: mockResolveTenant,
  };
});

import {
  TenantProvider,
  usePresenceMode,
  useTenant,
  useTenantSafe,
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
  presenceMode: "detailed",
  studentPhotosEnabled: false,
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

// ============================================================================
// Cross-tab settings sync (P3 — single subscription per tab)
// ============================================================================
//
// The provider owns ONE subscription to settings-broadcast. When a save in
// another tab fires, this provider re-resolves the tenant ONCE and updates
// context, so all consumers re-render via React context. Previously each
// `useStudentPhotosEnabled` call site (one per StudentCard, etc.) opened its
// own subscription and triggered its own resolveTenant fetch — a thundering
// herd on pages with many students.

describe("TenantProvider — cross-tab settings sync", () => {
  beforeEach(() => {
    broadcastHandlers.clear();
    mockResolveTenant.mockReset();
  });

  it("registers exactly one broadcast subscription regardless of consumer count", () => {
    function ManyConsumers() {
      // Render 5 hook instances. With the old per-card pattern, each would
      // open its own subscription. The centralised provider must keep the
      // subscriber count at 1.
      return (
        <>
          {Array.from({ length: 5 }, (_, i) => (
            <ConsumerProbe key={i} />
          ))}
        </>
      );
    }
    function ConsumerProbe() {
      useTenantSafe();
      return null;
    }

    render(
      <TenantProvider tenantSlug="demo-school" tenant={mockTenant}>
        <ManyConsumers />
      </TenantProvider>,
    );

    expect(broadcastHandlers.size).toBe(1);
  });

  it("re-resolves the tenant on broadcast and propagates the fresh value", async () => {
    const updatedTenant: TenantInfo = {
      ...mockTenant,
      studentPhotosEnabled: true,
    };
    mockResolveTenant.mockResolvedValue(updatedTenant);

    let observed: TenantInfo | null = null;
    function Probe() {
      observed = useTenantSafe()?.tenant ?? null;
      return null;
    }

    render(
      <TenantProvider tenantSlug="demo-school" tenant={mockTenant}>
        <Probe />
      </TenantProvider>,
    );

    expect(observed).toEqual(mockTenant);

    // Simulate another tab broadcasting a settings change.
    act(() => {
      for (const handler of broadcastHandlers) handler();
    });

    await waitFor(() => {
      expect(observed?.studentPhotosEnabled).toBe(true);
    });
    expect(mockResolveTenant).toHaveBeenCalledTimes(1);
    expect(mockResolveTenant).toHaveBeenCalledWith("demo-school");
  });

  it("ignores stale resolveTenant responses that finish out of order", async () => {
    // Bursting refetches (visibilitychange + SSE + BroadcastChannel
    // landing in the same tick) used to last-writer-wins on whichever
    // promise resolved last. If the OLDER request finished last, it would
    // overwrite the newer studentPhotosEnabled state and leave the tab on
    // a stale flag until the next refresh. The provider now sequences
    // requests with a token; this test pins that contract.
    const oldStateTenant: TenantInfo = {
      ...mockTenant,
      studentPhotosEnabled: false,
    };
    const newStateTenant: TenantInfo = {
      ...mockTenant,
      studentPhotosEnabled: true,
    };

    // Two pending promises we control resolution order on.
    let resolveFirst: (v: TenantInfo) => void = () => {
      /* assigned below */
    };
    let resolveSecond: (v: TenantInfo) => void = () => {
      /* assigned below */
    };
    const firstPromise = new Promise<TenantInfo>((r) => (resolveFirst = r));
    const secondPromise = new Promise<TenantInfo>((r) => (resolveSecond = r));

    mockResolveTenant
      .mockReturnValueOnce(firstPromise)
      .mockReturnValueOnce(secondPromise);

    // Wrap the observed value in an object so TS doesn't flow-narrow it
    // back to `null` after the initial assignment in this scope (closure
    // assignments inside Probe don't widen the let-binding's narrowed
    // type). Reading observed.value at assertion time always sees the
    // latest committed render.
    const observed: { value: TenantInfo | null } = { value: null };
    function Probe() {
      observed.value = useTenantSafe()?.tenant ?? null;
      return null;
    }

    render(
      <TenantProvider tenantSlug="demo-school" tenant={mockTenant}>
        <Probe />
      </TenantProvider>,
    );

    // Two refetches fired in quick succession (e.g. SSE + visibilitychange).
    act(() => {
      for (const handler of broadcastHandlers) handler();
      for (const handler of broadcastHandlers) handler();
    });

    // Resolve them out of order: the SECOND request returns first with
    // the new state, then the FIRST request returns later with old state.
    await act(async () => {
      resolveSecond(newStateTenant);
      // Yield so the second .then() runs.
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(observed.value?.studentPhotosEnabled).toBe(true);
    });

    // Now the older request finishes — it must NOT overwrite the new state.
    await act(async () => {
      resolveFirst(oldStateTenant);
      await Promise.resolve();
    });

    expect(observed.value?.studentPhotosEnabled).toBe(true);
  });
});

describe("usePresenceMode", () => {
  const binaryTenant: TenantInfo = { ...mockTenant, presenceMode: "binary" };

  it("returns the tenant's presenceMode when inside a TenantProvider", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantProvider tenantSlug="demo" tenant={binaryTenant}>
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
