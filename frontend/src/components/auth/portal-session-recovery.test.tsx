import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { reportSessionTenantMismatch } from "~/lib/api-error";
import { primeSessionCache } from "~/lib/session-cache";
import { boundSessionAccount } from "~/lib/session-account";
import { TenantProviders } from "~/app/[tenant]/providers";
import { OperatorProviders } from "~/app/operator/providers";
import { ParentProviders } from "~/app/parents/providers";
import { SchoolProviders } from "~/app/school/providers";
import { CatererProviders } from "~/app/caterer/providers";
import { TraegerProviders } from "~/app/traeger/providers";

const { mockSession, passthrough, empty } = vi.hoisted(() => ({
  mockSession: vi.fn(),
  passthrough: ({ children }: { children?: ReactNode }) => children,
  empty: () => null,
}));
vi.mock("next-auth/react", () => ({
  SessionProvider: passthrough,
  useSession: mockSession,
  getSession: vi.fn(),
}));
vi.mock("~/components/auth/personal-storage-guard", () => ({
  PersonalStorageGuard: empty,
}));
vi.mock("~/components/auth/sentry-session-context", () => ({
  SentrySessionContext: empty,
}));
vi.mock("~/lib/swr/scope-boundary", () => ({ SWRScopeBoundary: empty }));
vi.mock("~/components/notifications/service-worker-registrar", () => ({
  PushSubscriptionSync: empty,
}));
vi.mock("~/components/analytics/parent-portal-analytics-session", () => ({
  ParentPortalAnalyticsSession: empty,
}));
vi.mock("~/components/analytics/portal-analytics-session", () => ({
  PortalAnalyticsSession: empty,
}));
vi.mock("~/components/analytics/traeger-analytics-session", () => ({
  TraegerAnalyticsSession: empty,
}));
vi.mock("~/lib/module-portal-context", () => ({
  ModulePortalProvider: passthrough,
}));
vi.mock("~/lib/parent-locale-context", () => ({
  ParentLocaleProvider: passthrough,
}));
vi.mock("~/lib/profile-context", () => ({ ProfileProvider: passthrough }));
vi.mock("~/lib/supervision-context", () => ({
  SupervisionProvider: passthrough,
}));
vi.mock("~/lib/tenant-context", () => ({ TenantProvider: passthrough }));
vi.mock("~/components/auth/tenant-auth-wrapper", () => ({
  TenantAuthWrapper: passthrough,
}));

const messages: unknown[] = [];
class RecordingChannel {
  constructor(readonly name: string) {}
  postMessage(value: unknown) {
    if (this.name === "next-auth") messages.push(value);
  }
  close() {}
}
function signedIn(id: string) {
  return {
    status: "authenticated",
    data: { user: { id, token: "access" }, expires: "2099-01-01" },
  };
}
beforeEach(() => {
  messages.length = 0;
  primeSessionCache(null);
  vi.stubGlobal("BroadcastChannel", RecordingChannel);
  mockSession.mockReturnValue(signedIn("A"));
});

afterEach(() => vi.unstubAllGlobals());

const providers = [
  [
    "tenant",
    (children: ReactNode) => (
      <TenantProviders
        tenantSlug="school-a"
        tenant={{
          messagingEnabled: false,
          staffMessagingEnabled: false,
          displayEnabled: false,
          gradeLevelMax: 4,
          tenantId: 7,
          slug: "school-a",
          settings: {},
          presenceMode: "detailed",
          studentPhotosEnabled: false,
          bookedModules: [],
          nfcEnabled: false,
          subdomain: "school-a",
          name: "School A",
          organizationId: 1,
          organizationName: "Org",
        }}
        routingMode="subdomain"
      >
        {children}
      </TenantProviders>
    ),
  ],
  [
    "operator",
    (children: ReactNode) => <OperatorProviders>{children}</OperatorProviders>,
  ],
  [
    "parent",
    (children: ReactNode) => <ParentProviders>{children}</ParentProviders>,
  ],
  [
    "school",
    (children: ReactNode) => <SchoolProviders>{children}</SchoolProviders>,
  ],
  [
    "caterer",
    (children: ReactNode) => <CatererProviders>{children}</CatererProviders>,
  ],
  [
    "traeger",
    (children: ReactNode) => <TraegerProviders>{children}</TraegerProviders>,
  ],
] as const;

describe("every portal re-arms account refusal reports", () => {
  it.each(providers)(
    "%s handles A → B → A without a reload",
    (_portal, provider) => {
      const view = render(provider(<p>Portal content</p>));
      expect(screen.getByText("Portal content")).toBeInTheDocument();
      expect(boundSessionAccount()).toBe("A");
      reportSessionTenantMismatch("identity.session_account_mismatch");
      reportSessionTenantMismatch("identity.session_account_mismatch");
      expect(messages).toHaveLength(1);

      mockSession.mockReturnValue(signedIn("B"));
      view.rerender(provider(<p>Portal content</p>));
      expect(boundSessionAccount()).toBe("B");
      reportSessionTenantMismatch("identity.session_account_mismatch");
      expect(messages).toHaveLength(2);

      mockSession.mockReturnValue(signedIn("A"));
      view.rerender(provider(<p>Portal content</p>));
      expect(boundSessionAccount()).toBe("A");
      reportSessionTenantMismatch("identity.session_account_mismatch");
      expect(messages).toHaveLength(3);
    },
  );
});
