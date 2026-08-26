import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { renderHook } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { TenantInfo } from "~/lib/tenant-api";

// Override the global mock from test/setup.ts to use the REAL implementation
vi.unmock("~/lib/tenant-context");

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
  useAttendanceLogEnabled,
  useAttendanceWebEnabled,
  useCareOfferingsEnabled,
  useDisplayEnabled,
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
  useShowTimetableCounts,
  useTenant,
  useTenantRoutingModeSafe,
  useTenantSafe,
  useTenantSlugSafe,
  useWaitlistEnabled,
} from "~/lib/tenant-context";

// ============================================================================
// Test Data
// ============================================================================

const mockTenant: TenantInfo = {
  tenantId: 1,
  slug: "demo-school",
  name: "Demo School",
  // The provider's tenantSlug prop is the URL segment, which is the
  // school's SUBDOMAIN — keep the fixture consistent with that (#1975).
  subdomain: "demo-school",
  organizationId: 10,
  organizationName: "Org A",
  settings: {},
  presenceMode: "detailed",
  studentPhotosEnabled: false,
  nfcEnabled: false,
  messagingEnabled: false,
  staffMessagingEnabled: false,
  displayEnabled: false,
  gradeLevelMax: 4,
};

function TenantWrapper({
  children,
  tenant = mockTenant,
}: Readonly<{
  children: React.ReactNode;
  tenant?: TenantInfo | null;
}>) {
  return (
    <TenantProvider tenantSlug="demo-school" tenant={tenant}>
      {children}
    </TenantProvider>
  );
}

function useTenantSettingValues() {
  return {
    careOfferingsEnabled: useCareOfferingsEnabled(),
    attendanceWebEnabled: useAttendanceWebEnabled(),
    openCareGroupMode: useOpenCareGroupMode(),
    showTimetableCounts: useShowTimetableCounts(),
    waitlistEnabled: useWaitlistEnabled(),
  };
}

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

describe("useTenantRoutingModeSafe", () => {
  it("returns routing mode from provider", () => {
    function Wrapper({ children }: { children: React.ReactNode }) {
      return (
        <TenantProvider
          tenantSlug="demo-school"
          tenant={mockTenant}
          routingMode="subdomain"
        >
          {children}
        </TenantProvider>
      );
    }

    const { result } = renderHook(() => useTenantRoutingModeSafe(), {
      wrapper: Wrapper,
    });

    expect(result.current).toBe("subdomain");
  });

  it("defaults to path mode outside provider", () => {
    const { result } = renderHook(() => useTenantRoutingModeSafe());

    expect(result.current).toBe("path");
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

  it("applies refetched values when the slug column differs from the subdomain (#1975)", async () => {
    // slug and subdomain are independent columns (prod example: slug
    // "ogs-burbach", subdomain "burbach"). The URL segment is the
    // subdomain, so the freshness guard must compare subdomain — a
    // slug compare silently dropped every settings refresh for such
    // schools.
    const divergentTenant: TenantInfo = {
      ...mockTenant,
      slug: "ogs-demo-school",
      subdomain: "demo-school",
    };
    mockResolveTenant.mockResolvedValue({
      ...divergentTenant,
      studentPhotosEnabled: true,
    });

    const observed: { value: TenantInfo | null } = { value: null };
    function Probe() {
      observed.value = useTenantSafe()?.tenant ?? null;
      return null;
    }

    render(
      <TenantProvider tenantSlug="demo-school" tenant={divergentTenant}>
        <Probe />
      </TenantProvider>,
    );

    act(() => {
      for (const handler of broadcastHandlers) handler();
    });

    await waitFor(() => {
      expect(observed.value?.studentPhotosEnabled).toBe(true);
    });
  });

  it("ignores refetched values for a different subdomain", async () => {
    mockResolveTenant.mockResolvedValue({
      ...mockTenant,
      subdomain: "other-school",
      studentPhotosEnabled: true,
    });

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

    await act(async () => {
      for (const handler of broadcastHandlers) handler();
      await Promise.resolve();
    });

    expect(mockResolveTenant).toHaveBeenCalledWith("demo-school");
    expect(observed.value).toEqual(mockTenant);
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

  it("drops in-flight responses from a previous tenantSlug after navigation", async () => {
    // Real-world scenario: a user toggles a setting on School A, then
    // immediately switches to School B before School A's resolveTenant
    // returns. Without the slug guard + monotonic counter, A's response
    // would land last and overwrite B's context, leaving the new tab
    // showing School A's metadata under School B's URL.
    const tenantA: TenantInfo = {
      ...mockTenant,
      slug: "school-a",
      subdomain: "school-a",
      name: "A",
    };
    const tenantB: TenantInfo = {
      ...mockTenant,
      slug: "school-b",
      subdomain: "school-b",
      name: "B",
    };

    let resolveForA: (v: TenantInfo) => void = () => {};
    let resolveForB: (v: TenantInfo) => void = () => {};
    const aPromise = new Promise<TenantInfo>((r) => (resolveForA = r));
    const bPromise = new Promise<TenantInfo>((r) => (resolveForB = r));

    mockResolveTenant.mockImplementation((slug: string) => {
      if (slug === "school-a") return aPromise;
      if (slug === "school-b") return bPromise;
      return Promise.resolve(null);
    });

    const observed: { value: TenantInfo | null } = { value: null };
    function Probe() {
      observed.value = useTenantSafe()?.tenant ?? null;
      return null;
    }

    const { rerender } = render(
      <TenantProvider tenantSlug="school-a" tenant={tenantA}>
        <Probe />
      </TenantProvider>,
    );

    // Kick off A's refetch.
    act(() => {
      for (const handler of broadcastHandlers) handler();
    });

    // Navigate to School B — new slug, new server-provided tenant.
    rerender(
      <TenantProvider tenantSlug="school-b" tenant={tenantB}>
        <Probe />
      </TenantProvider>,
    );

    // Kick off B's refetch.
    act(() => {
      for (const handler of broadcastHandlers) handler();
    });

    // School A's in-flight resolve finally lands — must be ignored.
    await act(async () => {
      resolveForA({ ...tenantA, name: "A (stale update)" });
      await Promise.resolve();
    });

    // Context still shows B's server-provided tenant; A's response was
    // dropped by either the counter or the slug guard.
    expect(observed.value?.slug).toBe("school-b");

    // B's resolve lands with a fresh value — that one DOES apply.
    await act(async () => {
      resolveForB({ ...tenantB, name: "B (fresh)" });
      await Promise.resolve();
    });

    await waitFor(() => {
      expect(observed.value?.name).toBe("B (fresh)");
    });
    expect(observed.value?.slug).toBe("school-b");
  });

  it("drops A's in-flight response even when B never fires its own refetch", async () => {
    // Tighter variant of the test above: the reviewer flagged that the
    // slug guard alone is not enough. If A's refetch is in flight,
    // and the user navigates to B BEFORE a B-refetch bumps the
    // sequence counter, then at A's resolve time:
    //   token === counter (both still 1)
    //   fresh.slug === requestedSlug (both "school-a", captured at refetch time)
    // → setTenant(A) wins and overwrites B's server-provided context.
    // The cleanup-counter-bump prevents this.
    const tenantA: TenantInfo = {
      ...mockTenant,
      slug: "school-a",
      subdomain: "school-a",
      name: "A",
    };
    const tenantB: TenantInfo = {
      ...mockTenant,
      slug: "school-b",
      subdomain: "school-b",
      name: "B",
    };

    let resolveForA: (v: TenantInfo) => void = () => {};
    const aPromise = new Promise<TenantInfo>((r) => (resolveForA = r));

    mockResolveTenant.mockImplementation((slug: string) => {
      if (slug === "school-a") return aPromise;
      // school-b never resolves in this test — no B-refetch is fired.
      return new Promise<TenantInfo>(() => {
        /* never */
      });
    });

    const observed: { value: TenantInfo | null } = { value: null };
    function Probe() {
      observed.value = useTenantSafe()?.tenant ?? null;
      return null;
    }

    const { rerender } = render(
      <TenantProvider tenantSlug="school-a" tenant={tenantA}>
        <Probe />
      </TenantProvider>,
    );

    // Kick off A's refetch.
    act(() => {
      for (const handler of broadcastHandlers) handler();
    });

    // Navigate to B without firing a B-refetch.
    rerender(
      <TenantProvider tenantSlug="school-b" tenant={tenantB}>
        <Probe />
      </TenantProvider>,
    );

    expect(observed.value?.slug).toBe("school-b");

    // A's in-flight resolve lands. With the counter-bump on cleanup,
    // its token is now stale (counter advanced when A's effect tore
    // down) and the response must be ignored.
    await act(async () => {
      resolveForA({ ...tenantA, name: "A (stale)" });
      await Promise.resolve();
    });

    expect(observed.value?.slug).toBe("school-b");
    expect(observed.value?.name).toBe("B");
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

describe("useNFCEnabled", () => {
  const nfcTenant: TenantInfo = { ...mockTenant, nfcEnabled: true };

  it("returns true when NFC is enabled for the tenant", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantProvider tenantSlug="demo" tenant={nfcTenant}>
        {children}
      </TenantProvider>
    );
    const { result } = renderHook(() => useNFCEnabled(), { wrapper });
    expect(result.current).toBe(true);
  });

  it("returns false outside any TenantProvider", () => {
    const { result } = renderHook(() => useNFCEnabled());
    expect(result.current).toBe(false);
  });

  it("returns false when tenant hasn't resolved yet", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantProvider tenantSlug="demo" tenant={null}>
        {children}
      </TenantProvider>
    );
    const { result } = renderHook(() => useNFCEnabled(), { wrapper });
    expect(result.current).toBe(false);
  });
});

describe("useDisplayEnabled", () => {
  const displayTenant: TenantInfo = { ...mockTenant, displayEnabled: true };

  it("returns true when the Info-Point Dashboard is enabled for the tenant", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantProvider tenantSlug="demo" tenant={displayTenant}>
        {children}
      </TenantProvider>
    );
    const { result } = renderHook(() => useDisplayEnabled(), { wrapper });
    expect(result.current).toBe(true);
  });

  it("returns false outside any TenantProvider", () => {
    const { result } = renderHook(() => useDisplayEnabled());
    expect(result.current).toBe(false);
  });

  it("returns false when tenant hasn't resolved yet", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantProvider tenantSlug="demo" tenant={null}>
        {children}
      </TenantProvider>
    );
    const { result } = renderHook(() => useDisplayEnabled(), { wrapper });
    expect(result.current).toBe(false);
  });
});

describe("useAttendanceLogEnabled", () => {
  it("returns true when the attendance log is enabled for the tenant", () => {
    const logTenant: TenantInfo = { ...mockTenant, attendanceLogEnabled: true };
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantProvider tenantSlug="demo" tenant={logTenant}>
        {children}
      </TenantProvider>
    );
    const { result } = renderHook(() => useAttendanceLogEnabled(), { wrapper });
    expect(result.current).toBe(true);
  });

  it("returns false outside any TenantProvider", () => {
    const { result } = renderHook(() => useAttendanceLogEnabled());
    expect(result.current).toBe(false);
  });

  it("returns false when tenant hasn't resolved yet", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantProvider tenantSlug="demo" tenant={null}>
        {children}
      </TenantProvider>
    );
    const { result } = renderHook(() => useAttendanceLogEnabled(), { wrapper });
    expect(result.current).toBe(false);
  });
});

describe("tenant setting hooks", () => {
  it("returns enabled values from the resolved tenant", () => {
    const enabledTenant: TenantInfo = {
      ...mockTenant,
      careOfferingsEnabled: true,
      attendanceWebEnabled: true,
      groupMode: "open_care",
      showTimetableCounts: true,
      waitlistEnabled: true,
    };
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantWrapper tenant={enabledTenant}>{children}</TenantWrapper>
    );

    const { result } = renderHook(() => useTenantSettingValues(), { wrapper });

    expect(result.current).toEqual({
      careOfferingsEnabled: true,
      attendanceWebEnabled: true,
      openCareGroupMode: true,
      showTimetableCounts: true,
      waitlistEnabled: true,
    });
  });

  it("returns disabled values from the resolved tenant", () => {
    const disabledTenant: TenantInfo = {
      ...mockTenant,
      careOfferingsEnabled: false,
      attendanceWebEnabled: false,
      groupMode: "fixed_groups",
      showTimetableCounts: false,
      waitlistEnabled: false,
    };
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantWrapper tenant={disabledTenant}>{children}</TenantWrapper>
    );

    const { result } = renderHook(() => useTenantSettingValues(), { wrapper });

    expect(result.current).toEqual({
      careOfferingsEnabled: false,
      attendanceWebEnabled: false,
      openCareGroupMode: false,
      showTimetableCounts: false,
      waitlistEnabled: false,
    });
  });

  it("uses safe metadata defaults when fields are missing", () => {
    const { result } = renderHook(() => useTenantSettingValues(), {
      wrapper: TenantWrapper,
    });

    expect(result.current).toEqual({
      careOfferingsEnabled: true,
      attendanceWebEnabled: false,
      openCareGroupMode: false,
      showTimetableCounts: true,
      waitlistEnabled: true,
    });
  });

  it("uses safe metadata defaults while the tenant is unresolved", () => {
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <TenantWrapper tenant={null}>{children}</TenantWrapper>
    );
    const { result } = renderHook(() => useTenantSettingValues(), { wrapper });

    expect(result.current).toEqual({
      careOfferingsEnabled: true,
      attendanceWebEnabled: false,
      openCareGroupMode: false,
      showTimetableCounts: true,
      waitlistEnabled: true,
    });
  });

  it("uses safe metadata defaults outside a TenantProvider", () => {
    const { result } = renderHook(() => useTenantSettingValues());

    expect(result.current).toEqual({
      careOfferingsEnabled: true,
      attendanceWebEnabled: false,
      openCareGroupMode: false,
      showTimetableCounts: true,
      waitlistEnabled: true,
    });
  });
});
