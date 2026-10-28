import type { ReactNode } from "react";
import { act, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import type { ShellBootstrap } from "~/lib/shell-seed";
import type { TenantInfo } from "~/lib/tenant-api";
import { mockSessionData } from "~/test/mocks/next-auth";

// Exercise the real fallback, cacheData, seeder and boundary together.
vi.unmock("swr");
const { mockUseSession, mockAnnounce } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockAnnounce: vi.fn(),
}));
vi.mock("next-auth/react", () => ({
  SessionProvider: ({ children }: { children: ReactNode }) => children,
  useSession: mockUseSession,
}));
vi.mock("~/lib/session-cache", () => ({
  primeSessionCache: vi.fn(),
  announceSessionChange: mockAnnounce,
}));
vi.mock("~/components/auth/personal-storage-guard", () => ({
  PersonalStorageGuard: () => null,
}));
vi.mock("~/components/auth/sentry-session-context", () => ({
  SentrySessionContext: () => null,
}));
vi.mock("~/components/notifications/service-worker-registrar", () => ({
  PushSubscriptionSync: () => null,
}));
vi.mock("~/components/auth/tenant-auth-wrapper", () => ({
  TenantAuthWrapper: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("~/lib/profile-context", () => ({
  ProfileProvider: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("~/lib/supervision-context", () => ({
  SupervisionProvider: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("~/lib/tenant-context", () => ({
  TenantProvider: ({ children }: { children: ReactNode }) => children,
  useTenantSlugSafe: () => "school-a",
}));

import useSWR from "swr";
import { resetSWRScope, resumeSWRSession } from "~/lib/swr/scope";
import { TenantProviders } from "~/app/[tenant]/providers";

const session = mockSessionData({ user: { tenantId: 1 } });
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
    schoolSwitches: null,
  },
  reminders: { enabled: true, reminders: [], count: 7 },
  staffNoticesToday: [],
  counts: {},
};

function ShellReader() {
  // Keep the old keys deliberately, so disabling wrapper keys alone cannot
  // hide a fallback that should have been withdrawn.
  const context = useSWR("school-a:1:user-context", null);
  const reminders = useSWR("school-a:1:reminders", null);
  const notices = useSWR("school-a:1:staff-notices-today:1", null);
  const count = [context.data, reminders.data, notices.data].filter(
    (data) => data !== undefined,
  ).length;
  return <p data-testid="shell">{count} seeds</p>;
}

function Page() {
  return (
    <TenantProviders
      tenantSlug="school-a"
      tenant={{ tenantId: 1 } as TenantInfo}
      routingMode="subdomain"
      session={session}
      shell={shell}
    >
      <ShellReader />
    </TenantProviders>
  );
}

afterEach(() => {
  resumeSWRSession();
  resetSWRScope({ revalidate: false });
});

it.each([null, "1:2"])(
  "withdraws the real shell fallback immediately on owner change to %s",
  async (owner) => {
    mockUseSession.mockReturnValue({ data: session, status: "authenticated" });
    const view = render(<Page />);
    // Anker: alle drei echten Shell-Seeds sind sichtbar.
    expect(screen.getByTestId("shell")).toHaveTextContent("3 seeds");
    await act(async () => {
      globalThis.dispatchEvent(
        new StorageEvent("storage", {
          key: "moto:personal-storage-owner",
          newValue: owner,
        }),
      );
    });
    expect(screen.getByTestId("shell")).toHaveTextContent("0 seeds");
    expect(mockAnnounce).toHaveBeenCalled();
    view.rerender(<Page />);
    expect(screen.getByTestId("shell")).toHaveTextContent("0 seeds");

    // Returning to the server generation must not revive its withdrawn seed.
    mockUseSession.mockReturnValue({ data: null, status: "unauthenticated" });
    view.rerender(<Page />);
    mockUseSession.mockReturnValue({
      data: { ...session },
      status: "authenticated",
    });
    view.rerender(<Page />);
    expect(screen.getByTestId("shell")).toHaveTextContent("0 seeds");
  },
);
