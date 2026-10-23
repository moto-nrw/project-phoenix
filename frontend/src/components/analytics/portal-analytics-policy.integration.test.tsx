import { Suspense } from "react";
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { TenantInfo } from "~/lib/tenant-api";

vi.unmock("~/lib/tenant-context");

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  init: vi.fn(),
  optIn: vi.fn(),
  capture: vi.fn(),
  register: vi.fn(),
  reset: vi.fn(),
  fetchBootstrap: vi.fn(),
  fetchEditBootstrap: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
  usePathname: () => "/parents/anmeldung/school-c/5",
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
    register: mocks.register,
    unregister: vi.fn(),
    reset: mocks.reset,
  },
}));

vi.mock("~/lib/enrollment-submission-api", () => ({
  fetchParentEnrollmentBootstrap: mocks.fetchBootstrap,
  fetchEnrollmentEditBootstrap: mocks.fetchEditBootstrap,
}));
vi.mock("~/lib/parent-api", () => ({
  fetchParentEnrollmentProfile: async () => null,
  submitParentEnrollment: vi.fn(),
}));
vi.mock("~/components/enrollment/enrollment-form", () => ({
  EnrollmentForm: () => <div>Loaded enrollment form</div>,
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
    vi.restoreAllMocks();
  });

  it.each([false, true, "failure"] as const)(
    "revokes the previous account policy until the new account resolves (%s)",
    async (enabled) => {
      vi.stubGlobal("location", {
        host: "parents.localhost:3000",
        hostname: "parents.localhost",
      });
      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user: { id: "7", scope: "parent" } },
      });
      vi.mocked(fetch).mockResolvedValueOnce(
        Response.json({ data: { enabled: true } }),
      );
      const { PortalAnalyticsSession } =
        await import("./portal-analytics-session");
      const { initializePostHog, capturePostHog } =
        await import("~/lib/posthog-client");
      const view = render(<PortalAnalyticsSession surface="parents" />);
      await act(async () => {});
      await initializePostHog();
      capturePostHog("login_failed");
      expect(mocks.capture).toHaveBeenCalledWith("login_failed", undefined);
      mocks.capture.mockClear();

      let resolvePolicy!: (response: Response) => void;
      let rejectPolicy!: (error: Error) => void;
      vi.mocked(fetch).mockImplementationOnce(
        () =>
          new Promise<Response>((resolve, reject) => {
            resolvePolicy = resolve;
            rejectPolicy = reject;
          }),
      );
      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user: { id: "8", scope: "parent" } },
      });
      view.rerender(<PortalAnalyticsSession surface="parents" />);
      expect(fetch).toHaveBeenCalledTimes(2);
      capturePostHog("login_failed");
      expect(mocks.capture).not.toHaveBeenCalled();

      await act(async () => {
        if (enabled === "failure")
          rejectPolicy(new Error("Policy unavailable"));
        else resolvePolicy(Response.json({ data: { enabled } }));
      });
      capturePostHog("login_failed");
      if (enabled === true) {
        expect(mocks.capture).toHaveBeenCalledWith("login_failed", undefined);
      } else {
        expect(mocks.capture).not.toHaveBeenCalled();
      }
    },
  );

  it.each([
    { surface: "parents", scope: "parent", host: "parents.localhost" },
    { surface: "school", scope: "school", host: "schule.localhost" },
  ] as const)(
    "preserves a blocked $surface account switch reset across five-minute refreshes",
    async ({ surface, scope, host }) => {
      vi.stubGlobal("location", { host: `${host}:3000`, hostname: host });
      vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user: { id: "7", scope, tenantId: 42 } },
      });
      vi.mocked(fetch).mockResolvedValueOnce(
        Response.json({ data: { enabled: true } }),
      );
      const { PortalAnalyticsSession } =
        await import("./portal-analytics-session");
      const { initializePostHog, capturePostHog } =
        await import("~/lib/posthog-client");
      vi.useFakeTimers();
      const view = render(<PortalAnalyticsSession surface={surface} />);
      await act(async () => {});
      await initializePostHog();
      capturePostHog("login_failed");
      expect(mocks.capture).toHaveBeenCalledWith("login_failed", undefined);
      mocks.capture.mockClear();
      mocks.register.mockClear();

      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user: { id: "8", scope, tenantId: 43 } },
      });
      await act(async () =>
        view.rerender(<PortalAnalyticsSession surface={surface} />),
      );
      await act(async () => vi.advanceTimersByTimeAsync(5 * 60 * 1_000));
      expect(fetch).toHaveBeenCalledTimes(3);
      capturePostHog("login_failed");
      expect(mocks.capture).not.toHaveBeenCalled();
      expect(mocks.reset).not.toHaveBeenCalled();
      expect(mocks.register).not.toHaveBeenCalled();

      vi.mocked(fetch).mockResolvedValueOnce(
        Response.json({ data: { enabled: true } }),
      );
      await act(async () => vi.advanceTimersByTimeAsync(5 * 60 * 1_000));
      expect(mocks.reset).toHaveBeenCalledOnce();
      expect(mocks.register).toHaveBeenCalledExactlyOnceWith({
        school_id: "43",
        role: surface === "parents" ? "guardian" : "lehrkraft",
      });
      expect(mocks.reset.mock.invocationCallOrder[0]).toBeLessThan(
        mocks.register.mock.invocationCallOrder[0]!,
      );
      capturePostHog("login_failed");
      expect(mocks.capture).toHaveBeenCalledWith("login_failed", undefined);
      await act(async () => vi.advanceTimersByTimeAsync(5 * 60 * 1_000));
      expect(mocks.reset).toHaveBeenCalledOnce();
    },
  );

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

  it.each([
    { surface: "parents", scope: "parent", host: "parents.localhost" },
    { surface: "school", scope: "school", host: "schule.localhost" },
  ] as const)(
    "revokes an open $surface tab before rereading policy and ignores stale permission",
    async ({ surface, scope, host }) => {
      vi.stubGlobal("location", { host: `${host}:3000`, hostname: host });
      vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user: { id: "7", scope, tenantId: 42 } },
      });
      vi.mocked(fetch).mockResolvedValueOnce(
        Response.json({ data: { enabled: true } }),
      );
      const { PortalAnalyticsSession } =
        await import("./portal-analytics-session");
      const { initializePostHog, capturePostHog } =
        await import("~/lib/posthog-client");
      render(<PortalAnalyticsSession surface={surface} />);
      await act(async () => {});
      await initializePostHog();
      capturePostHog("login_failed");
      expect(mocks.capture).toHaveBeenCalledWith("login_failed", undefined);
      mocks.capture.mockClear();
      let resolveStale!: (response: Response) => void;
      vi.mocked(fetch).mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            resolveStale = resolve;
          }),
      );
      act(() => document.dispatchEvent(new Event("visibilitychange")));
      expect(fetch).toHaveBeenCalledTimes(2);
      capturePostHog("login_failed");
      expect(mocks.capture).not.toHaveBeenCalled();
      vi.mocked(fetch).mockResolvedValueOnce(
        Response.json({ data: { enabled: false } }),
      );
      await act(async () =>
        document.dispatchEvent(new Event("visibilitychange")),
      );
      expect(fetch).toHaveBeenCalledTimes(3);
      await act(async () =>
        resolveStale(Response.json({ data: { enabled: true } })),
      );
      capturePostHog("login_failed");
      expect(mocks.capture).not.toHaveBeenCalled();
    },
  );

  it.each([
    { hostEnabled: false, foreignEnabled: true },
    { hostEnabled: true, foreignEnabled: false },
    { hostEnabled: true, foreignEnabled: true },
  ])(
    "keeps the OGS host decision when a foreign school resolves last (host=$hostEnabled, embedded=$foreignEnabled)",
    async ({ hostEnabled, foreignEnabled }) => {
      vi.stubGlobal("location", {
        host: "school-a.localhost:3000",
        hostname: "school-a.localhost",
      });
      vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
      const { TenantProvider } = await import("~/lib/tenant-context");
      const { initializePostHog } = await import("~/lib/posthog-client");
      const foreign = {
        ...tenant,
        subdomain: "school-b",
        slug: "school-b",
        analyticsEnabled: foreignEnabled,
      };
      render(
        <TenantProvider
          tenantSlug="school-a"
          tenant={{ ...tenant, analyticsEnabled: hostEnabled }}
        >
          <TenantProvider tenantSlug="school-b" tenant={foreign}>
            <div>Foreign enrollment form</div>
          </TenantProvider>
        </TenantProvider>,
      );
      // Anker: both school providers render the embedded enrollment content.
      expect(screen.getByText("Foreign enrollment form")).toBeInTheDocument();
      let resolveHost!: (response: Response) => void;
      let resolveForeign!: (response: Response) => void;
      vi.mocked(fetch).mockImplementation(
        (input) =>
          new Promise<Response>((resolve) => {
            if (String(input).endsWith("school-a")) resolveHost = resolve;
            else resolveForeign = resolve;
          }),
      );
      act(() => document.dispatchEvent(new Event("visibilitychange")));
      expect(fetch).toHaveBeenCalledWith("/api/tenant/resolve?slug=school-a");
      expect(fetch).toHaveBeenCalledWith("/api/tenant/resolve?slug=school-b");
      await act(async () =>
        resolveHost(
          Response.json({
            data: {
              tenant_id: 42,
              slug: "school-a",
              subdomain: "school-a",
              analytics_enabled: hostEnabled,
            },
          }),
        ),
      );
      await act(async () =>
        resolveForeign(
          Response.json({
            data: {
              tenant_id: 43,
              slug: "school-b",
              subdomain: "school-b",
              analytics_enabled: foreignEnabled,
            },
          }),
        ),
      );
      await initializePostHog();
      expect(mocks.init).toHaveBeenCalledTimes(
        hostEnabled && foreignEnabled ? 1 : 0,
      );
    },
  );

  it.each([false, true])(
    "keeps an OGS token edit off until the foreign target is known (%s)",
    async (enabled) => {
      vi.stubGlobal("location", {
        host: "school-a.localhost:3000",
        hostname: "school-a.localhost",
      });
      vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
      let resolveBootstrap!: (value: unknown) => void;
      mocks.fetchEditBootstrap.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolveBootstrap = resolve;
          }),
      );
      vi.mocked(fetch).mockImplementation(async (input) => {
        const slug = String(input).endsWith("school-b")
          ? "school-b"
          : "school-a";
        return Response.json({
          data: {
            tenant_id: slug === "school-a" ? 42 : 43,
            slug,
            subdomain: slug,
            analytics_enabled: slug === "school-a" || enabled,
          },
        });
      });
      const { TenantProvider } = await import("~/lib/tenant-context");
      const { EnrollmentEditPage } =
        await import("~/components/enrollment/enrollment-edit-page");
      const { ToastProvider } = await import("~/contexts/ToastContext");
      const { initializePostHog, capturePostHog } =
        await import("~/lib/posthog-client");
      const view = render(
        <TenantProvider tenantSlug="school-a" tenant={tenant}>
          <div>Host school content</div>
        </TenantProvider>,
      );
      await initializePostHog();
      capturePostHog("login_failed");
      expect(mocks.capture).toHaveBeenCalledWith("login_failed", undefined);
      mocks.capture.mockClear();
      await act(async () =>
        view.rerender(
          <TenantProvider tenantSlug="school-a" tenant={tenant}>
            <ToastProvider>
              <Suspense fallback={<div>Resolving edit route</div>}>
                <EnrollmentEditPage
                  params={Promise.resolve({ token: "example-test-token" })}
                />
              </Suspense>
            </ToastProvider>
          </TenantProvider>,
        ),
      );
      expect(mocks.fetchEditBootstrap).toHaveBeenCalled();
      capturePostHog("login_failed");
      expect(mocks.capture).not.toHaveBeenCalled();
      await act(async () =>
        resolveBootstrap({
          draft: { tenant_subdomain: "school-b", phase_id: "5" },
          grade_level_max: 4,
          phase: {},
        }),
      );
      // Anker: the token edit loaded its actual foreign-school form.
      expect(screen.getByText("Loaded enrollment form")).toBeInTheDocument();
      // Initial resolution must not depend on a settings or visibility event.
      expect(fetch).toHaveBeenCalledWith("/api/tenant/resolve?slug=school-b");
      capturePostHog("login_failed");
      expect(mocks.capture).toHaveBeenCalledTimes(enabled ? 1 : 0);
    },
  );

  it.each([false, true])(
    "gates the actual parent enrollment page while target metadata loads (%s)",
    async (enabled) => {
      vi.stubGlobal("location", {
        host: "parents.localhost:3000",
        hostname: "parents.localhost",
      });
      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user: { id: "7", scope: "parent" } },
      });
      mocks.fetchBootstrap.mockResolvedValue({ phase: {} });
      let resolveTarget!: (response: Response) => void;
      vi.mocked(fetch).mockImplementation(async (input) => {
        if (String(input).startsWith("/api/tenant/resolve")) {
          return new Promise<Response>((resolve) => {
            resolveTarget = resolve;
          });
        }
        return Response.json({ data: { enabled: true } });
      });
      const { PortalAnalyticsSession } =
        await import("./portal-analytics-session");
      const { default: ParentEnrollFormPage } =
        await import("~/app/parents/anmeldung/[tenantSlug]/[phaseId]/page");
      const { initializePostHog, capturePostHog } =
        await import("~/lib/posthog-client");
      const view = render(<PortalAnalyticsSession surface="parents" />);
      await act(async () => {});
      await initializePostHog();
      capturePostHog("login_failed");
      expect(mocks.capture).toHaveBeenCalledWith("login_failed", undefined);
      mocks.capture.mockClear();
      await act(async () =>
        view.rerender(
          <>
            <PortalAnalyticsSession surface="parents" />
            <Suspense fallback={<div>Resolving route</div>}>
              <ParentEnrollFormPage
                params={Promise.resolve({
                  tenantSlug: "school-c",
                  phaseId: "5",
                })}
              />
            </Suspense>
          </>,
        ),
      );
      expect(fetch).toHaveBeenCalledWith("/api/tenant/resolve?slug=school-c");
      capturePostHog("login_failed");
      expect(mocks.capture).not.toHaveBeenCalled();
      await act(async () =>
        resolveTarget(
          Response.json({
            data: {
              tenant_id: 44,
              slug: "school-c",
              subdomain: "school-c",
              analytics_enabled: enabled,
              grade_level_max: 4,
            },
          }),
        ),
      );
      // Anker: the actual page finished loading its target school's form.
      expect(screen.getByText("Loaded enrollment form")).toBeInTheDocument();
      capturePostHog("login_failed");
      expect(mocks.capture).toHaveBeenCalledTimes(enabled ? 1 : 0);
    },
  );

  it.each([false, true])(
    "requires the enrollment target in addition to allowed parent memberships (%s)",
    async (targetEnabled) => {
      vi.stubGlobal("location", {
        host: "parents.localhost:3000",
        hostname: "parents.localhost",
      });
      mocks.useSession.mockReturnValue({
        status: "authenticated",
        data: { user: { scope: "parent" } },
      });
      vi.mocked(fetch).mockImplementation(async (input) => {
        if (String(input).startsWith("/api/tenant/resolve")) {
          return Response.json({
            data: {
              tenant_id: 42,
              slug: "school-a",
              subdomain: "school-a",
              analytics_enabled: targetEnabled,
            },
          });
        }
        return Response.json({ data: { enabled: true } });
      });
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
            tenant={{ ...tenant, analyticsEnabled: targetEnabled }}
          >
            <div>Enrollment form</div>
          </TenantProvider>
        </>,
      );
      // Anker: the target-school page is mounted alongside the parent session.
      expect(screen.getByText("Enrollment form")).toBeInTheDocument();
      await act(async () => {});
      await initializePostHog();
      expect(mocks.init).toHaveBeenCalledTimes(targetEnabled ? 1 : 0);
      // A portal refresh must not override the target school's veto.
      await act(async () =>
        document.dispatchEvent(new Event("visibilitychange")),
      );
      await initializePostHog();
      expect(mocks.init).toHaveBeenCalledTimes(targetEnabled ? 1 : 0);
      view.rerender(
        <>
          <PortalAnalyticsSession surface="parents" />
        </>,
      );
      await act(async () => {});
      await initializePostHog();
      capturePostHog("login_failed");
      expect(mocks.capture).toHaveBeenCalledWith("login_failed", undefined);
    },
  );
});
