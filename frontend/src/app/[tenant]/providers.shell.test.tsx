import type React from "react";
import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TenantInfo } from "~/lib/tenant-api";
import type { ShellBootstrap } from "~/lib/shell-seed";
import { mockSessionData } from "~/test/mocks/next-auth";

/**
 * TenantProviders with a server snapshot (#2973): the session primes the
 * getSession() cache before any child effect, the SWR entries land under the
 * tenant-scoped keys, and the providers receive their initial values.
 */

const {
  mockSessionProvider,
  mockUseSession,
  mockSWRConfig,
  mockProfileProvider,
  mockSupervisionProvider,
  mockPrimeSessionCache,
  childEffectOrder,
} = vi.hoisted(() => ({
  mockSessionProvider: vi.fn(),
  mockUseSession: vi.fn(),
  mockSWRConfig: vi.fn(),
  mockProfileProvider: vi.fn(),
  mockSupervisionProvider: vi.fn(),
  mockPrimeSessionCache: vi.fn(),
  childEffectOrder: [] as string[],
}));

vi.mock("next-auth/react", () => ({
  SessionProvider: (props: { children: React.ReactNode }) => {
    mockSessionProvider(props);
    return props.children;
  },
  useSession: mockUseSession,
}));

vi.mock("swr", () => ({
  SWRConfig: (props: { children: React.ReactNode; value: unknown }) => {
    mockSWRConfig(props.value);
    return props.children;
  },
}));

vi.mock("~/lib/session-cache", () => ({
  primeSessionCache: (...args: unknown[]) => {
    childEffectOrder.push("prime");
    mockPrimeSessionCache(...args);
  },
}));

vi.mock("~/components/notifications/service-worker-registrar", () => ({
  PushSubscriptionSync: () => null,
}));

vi.mock("~/components/auth/tenant-auth-wrapper", () => ({
  TenantAuthWrapper: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/lib/profile-context", () => ({
  ProfileProvider: (props: { children: React.ReactNode }) => {
    mockProfileProvider(props);
    return props.children;
  },
}));

vi.mock("~/lib/supervision-context", () => ({
  SupervisionProvider: (props: { children: React.ReactNode }) => {
    mockSupervisionProvider(props);
    return props.children;
  },
}));

vi.mock("~/lib/tenant-context", () => ({
  TenantProvider: ({ children }: { children: React.ReactNode }) => children,
}));

const { TenantProviders } = await import("./providers");
const { useLayoutEffect } = await import("react");

function ChildWithLayoutEffect() {
  useLayoutEffect(() => {
    childEffectOrder.push("child");
  }, []);
  return null;
}

const tenant = { tenantId: 1, subdomain: "school-a" } as TenantInfo;
const session = mockSessionData({ user: { token: "server-token" } });

const shell: ShellBootstrap = {
  accountId: "1",
  userContext: {
    educationalGroups: [],
    supervisedGroups: [],
    currentStaff: null,
    educationalGroupIds: [],
    educationalGroupRoomNames: [],
    supervisedRoomNames: [],
    incomplete: false,
    unavailableSections: [],
  },
  settingsSchema: { tabs: [] },
  profile: {
    id: "1",
    firstName: "Ada",
    lastName: "L",
    email: "a@b",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  reminders: { enabled: true, reminders: [], count: 0 },
  announcements: [],
  supervision: {
    groups: [],
    supervised: [],
    schulhof: null,
    overviewOk: false,
  },
  counts: { messagesUnread: 2 },
};

afterEach(() => {
  vi.clearAllMocks();
  childEffectOrder.length = 0;
});

beforeEach(() => {
  mockUseSession.mockReturnValue({ data: session, status: "authenticated" });
});

describe("TenantProviders with a server snapshot", () => {
  it("hydrates SessionProvider with the read-only server session", () => {
    render(
      <TenantProviders
        tenantSlug="school-a"
        tenant={tenant}
        routingMode="subdomain"
        session={session}
        shell={shell}
      >
        <div />
      </TenantProviders>,
    );

    expect(mockSessionProvider).toHaveBeenCalledWith(
      expect.objectContaining({ session }),
    );
  });

  it("leaves a missing server snapshot for Auth.js to resolve", () => {
    render(
      <TenantProviders
        tenantSlug="school-a"
        tenant={tenant}
        routingMode="subdomain"
      >
        <div />
      </TenantProviders>,
    );

    expect(mockSessionProvider).toHaveBeenCalledWith(
      expect.objectContaining({ session: undefined }),
    );
  });

  it("primes the session cache before any child layout effect runs", () => {
    render(
      <TenantProviders
        tenantSlug="school-a"
        tenant={tenant}
        routingMode="subdomain"
        session={session}
        shell={shell}
      >
        <ChildWithLayoutEffect />
      </TenantProviders>,
    );

    expect(mockPrimeSessionCache).toHaveBeenCalledWith(session);
    expect(childEffectOrder).toEqual(["prime", "child"]);
  });

  it("seeds SWR under the tenant-scoped keys and the providers with their slices", () => {
    render(
      <TenantProviders
        tenantSlug="school-a"
        tenant={tenant}
        routingMode="subdomain"
        session={session}
        shell={shell}
      >
        <div />
      </TenantProviders>,
    );

    const entries = {
      "school-a:user-context": shell.userContext,
      "school-a:settings-schema": shell.settingsSchema,
      "school-a:reminders": shell.reminders,
      // Platform-scoped: no tenant prefix.
      "user-announcements-unread": shell.announcements,
    };
    expect(mockSWRConfig).toHaveBeenCalledWith({
      fallback: entries,
      cacheData: entries,
    });
    expect(mockProfileProvider).toHaveBeenCalledWith(
      expect.objectContaining({ initialProfile: shell.profile }),
    );
    expect(mockSupervisionProvider).toHaveBeenCalledWith(
      expect.objectContaining({ initial: shell.supervision }),
    );
  });

  it("renders without SWRConfig and with empty seeds when there is no snapshot", () => {
    render(
      <TenantProviders
        tenantSlug="school-a"
        tenant={tenant}
        routingMode="subdomain"
        session={session}
      >
        <div />
      </TenantProviders>,
    );

    expect(mockSWRConfig).not.toHaveBeenCalled();
    expect(mockPrimeSessionCache).toHaveBeenCalledWith(session);
    expect(mockProfileProvider).toHaveBeenCalledWith(
      expect.objectContaining({ initialProfile: null }),
    );
    expect(mockSupervisionProvider).toHaveBeenCalledWith(
      expect.objectContaining({ initial: null }),
    );
  });
});
