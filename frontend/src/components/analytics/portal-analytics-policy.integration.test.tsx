import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TenantInfo } from "~/lib/tenant-api";

vi.unmock("~/lib/tenant-context");

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  init: vi.fn(),
  optIn: vi.fn(),
  capture: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));
vi.mock("~/env.client", () => ({
  clientEnv: {
    NEXT_PUBLIC_POSTHOG_KEY: "example-test-placeholder",
    NEXT_PUBLIC_TENANT_DOMAIN: "localhost",
    NEXT_PUBLIC_OPERATOR_HOSTNAME: "operator.localhost:3000",
    NEXT_PUBLIC_PARENTS_HOSTNAME: "parents.localhost:3000",
    NEXT_PUBLIC_SCHOOL_HOSTNAME: "schule.localhost:3000",
  },
}));
vi.mock("posthog-js", () => ({
  default: {
    init: mocks.init,
    opt_in_capturing: mocks.optIn,
    capture: mocks.capture,
    opt_out_capturing: vi.fn(),
    stopSessionRecording: vi.fn(),
    set_config: vi.fn(),
    get_distinct_id: () => "anonymous-test-visit",
    register: vi.fn(),
    unregister: vi.fn(),
    reset: vi.fn(),
  },
}));

const tenant: TenantInfo = {
  tenantId: 42,
  slug: "school-a",
  subdomain: "school-a",
  name: "School A",
  organizationId: 10,
  organizationName: "Test organization",
  settings: {},
  presenceMode: "detailed",
  studentPhotosEnabled: false,
  nfcEnabled: false,
  messagingEnabled: false,
  staffMessagingEnabled: false,
  displayEnabled: false,
  gradeLevelMax: 4,
  analyticsEnabled: true,
};

describe("embedded enrollment analytics policy", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    vi.stubGlobal("requestIdleCallback", vi.fn());
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: string) => {
        if (input.startsWith("/api/tenant/resolve")) {
          return Response.json({
            data: {
              tenant_id: 42,
              slug: "school-a",
              subdomain: "school-a",
              analytics_enabled: true,
            },
          });
        }
        return Response.json({ data: { enabled: false } });
      }),
    );
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it.each([
    { host: "parents.localhost:3000", surface: "parents", scope: "parent" },
    { host: "schule.localhost:3000", surface: "school", scope: "school" },
  ] as const)(
    "keeps the denied $surface session policy when school A allows analytics",
    async ({ host, surface, scope }) => {
      vi.stubGlobal("location", { host, hostname: host.split(":")[0] });
      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user: { scope, tenantId: scope === "school" ? 43 : null } },
      });
      const { PortalAnalyticsSession } =
        await import("./portal-analytics-session");
      const { TenantProvider, useTenant } =
        await import("~/lib/tenant-context");
      const { initializePostHog, capturePostHog } =
        await import("~/lib/posthog-client");
      function Enrollment() {
        return <div>{useTenant().tenant?.name}</div>;
      }
      const view = render(<PortalAnalyticsSession surface={surface} />);
      // Let the real portal module consume the denied policy before enrollment mounts.
      await act(async () => {});
      expect(fetch).toHaveBeenCalledWith(`/api/${scope}/analytics-policy`, {
        cache: "no-store",
      });
      view.rerender(
        <>
          <PortalAnalyticsSession surface={surface} />
          <TenantProvider tenantSlug="school-a" tenant={tenant}>
            <Enrollment />
          </TenantProvider>
        </>,
      );
      expect(screen.getByText("School A")).toBeInTheDocument();
      await act(async () => {
        await initializePostHog();
      });
      expect(mocks.init).not.toHaveBeenCalled();

      // The edit/adjust pages refresh this local provider after settings signals.
      await act(async () => {
        window.dispatchEvent(new Event("phoenix:tenant-settings-stale"));
      });
      expect(fetch).toHaveBeenCalledWith("/api/tenant/resolve?slug=school-a");
      await initializePostHog();
      capturePostHog("login_failed");
      expect(mocks.init).not.toHaveBeenCalled();
      expect(mocks.optIn).not.toHaveBeenCalled();
      expect(mocks.capture).not.toHaveBeenCalled();
    },
  );

  it.each([
    "parents.localhost:3000",
    "schule.localhost:3000",
    "localhost:3000",
    "operator.localhost:3000",
  ])("keeps anonymous enrollment on %s off", async (host) => {
    vi.stubGlobal("location", { host, hostname: host.split(":")[0] });
    const { TenantProvider } = await import("~/lib/tenant-context");
    const { initializePostHog } = await import("~/lib/posthog-client");
    render(
      <TenantProvider tenantSlug="school-a" tenant={tenant}>
        <div>Enrollment form</div>
      </TenantProvider>,
    );
    expect(screen.getByText("Enrollment form")).toBeInTheDocument();
    await initializePostHog();
    await act(async () => {
      document.dispatchEvent(new Event("visibilitychange"));
      window.dispatchEvent(new Event("phoenix:tenant-settings-stale"));
    });
    await initializePostHog();
    expect(mocks.init).not.toHaveBeenCalled();
  });

  it("still initializes on the OGS host when its school allows analytics", async () => {
    vi.stubGlobal("location", {
      host: "school-a.localhost:3000",
      hostname: "school-a.localhost",
    });
    const { TenantProvider } = await import("~/lib/tenant-context");
    const { initializePostHog } = await import("~/lib/posthog-client");
    render(
      <TenantProvider tenantSlug="school-a" tenant={tenant}>
        <div>Enrollment form</div>
      </TenantProvider>,
    );
    expect(screen.getByText("Enrollment form")).toBeInTheDocument();
    await initializePostHog();
    expect(mocks.init).toHaveBeenCalledOnce();
  });

  it("preserves an allowed parent policy through embedded-school refresh and unmount", async () => {
    vi.stubGlobal("location", {
      host: "parents.localhost:3000",
      hostname: "parents.localhost",
    });
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { scope: "parent" } },
    });
    vi.mocked(fetch).mockResolvedValue(
      Response.json({ data: { enabled: true } }),
    );
    const { PortalAnalyticsSession } =
      await import("./portal-analytics-session");
    const { TenantProvider } = await import("~/lib/tenant-context");
    const { initializePostHog, capturePostHog } =
      await import("~/lib/posthog-client");
    const view = render(
      <>
        <PortalAnalyticsSession surface="parents" />
        <TenantProvider
          tenantSlug="school-a"
          tenant={{ ...tenant, analyticsEnabled: false }}
        >
          <div>Enrollment form</div>
        </TenantProvider>
      </>,
    );
    expect(screen.getByText("Enrollment form")).toBeInTheDocument();
    await act(async () => {});
    await initializePostHog();
    expect(mocks.init).toHaveBeenCalledOnce();
    await act(async () => {
      window.dispatchEvent(new Event("phoenix:tenant-settings-stale"));
    });
    view.rerender(<PortalAnalyticsSession surface="parents" />);
    capturePostHog("login_failed");
    expect(mocks.capture).toHaveBeenCalledWith("login_failed", undefined);
  });
});
