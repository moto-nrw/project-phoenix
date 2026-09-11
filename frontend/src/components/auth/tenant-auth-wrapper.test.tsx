import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { TenantAuthWrapper } from "./tenant-auth-wrapper";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  useTenant: vi.fn(),
  setPostHogContext: vi.fn(),
  clearPostHogContext: vi.fn(),
  trackPageView: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));
vi.mock("next/navigation", () => ({
  usePathname: () => "/school-b/dashboard",
}));
vi.mock("~/lib/posthog-client", () => ({
  setPostHogContext: mocks.setPostHogContext,
  clearPostHogContext: mocks.clearPostHogContext,
}));
vi.mock("~/env.client", () => ({
  clientEnv: { NEXT_PUBLIC_TENANT_DOMAIN: "moto-app.de" },
}));
vi.mock("~/lib/hooks/use-user-context", () => ({
  useUserContext: () => ({ isReady: true }),
}));
vi.mock("~/lib/hooks/use-global-sse", () => ({
  useGlobalSSE: () => ({ status: "connected" }),
}));
vi.mock("~/lib/analytics", () => ({
  trackPageView: mocks.trackPageView,
}));
vi.mock("~/lib/tenant-context", () => ({ useTenant: mocks.useTenant }));

function renderWrapper() {
  return render(
    <TenantAuthWrapper>
      <div>Tenant content</div>
    </TenantAuthWrapper>,
  );
}

describe("TenantAuthWrapper analytics", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.useTenant.mockReturnValue({
      tenantSlug: "school-b",
      routingMode: "path",
      tenant: { tenantId: 2 },
    });
  });

  it("captures after the URL tenant matches the session tenant", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { tenantId: 2 } },
    });

    renderWrapper();

    expect(mocks.setPostHogContext).toHaveBeenCalledWith(
      {
        school_id: "2",
        $groups: { school: "2" },
        deployment: "moto-app.de",
      },
      false,
    );
    expect(mocks.trackPageView).toHaveBeenCalledWith("/dashboard", "2");
  });

  it("clears context and skips capture while TenantGuard switches tenants", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { tenantId: 1 } },
    });

    const view = renderWrapper();

    expect(mocks.setPostHogContext).not.toHaveBeenCalled();
    expect(mocks.trackPageView).not.toHaveBeenCalled();
    expect(mocks.clearPostHogContext).toHaveBeenCalledOnce();

    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { tenantId: 2 } },
    });
    view.rerender(
      <TenantAuthWrapper>
        <div>Tenant content</div>
      </TenantAuthWrapper>,
    );

    expect(mocks.setPostHogContext).toHaveBeenCalledWith(
      {
        school_id: "2",
        $groups: { school: "2" },
        deployment: "moto-app.de",
      },
      false,
    );
    expect(mocks.trackPageView).toHaveBeenCalledWith("/dashboard", "2");
  });

  it("resets identity before a direct authenticated tenant change", () => {
    mocks.useTenant.mockReturnValue({
      tenantSlug: "school-a",
      routingMode: "path",
      tenant: { tenantId: 1 },
    });
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { tenantId: 1 } },
    });

    const view = renderWrapper();

    expect(mocks.setPostHogContext).toHaveBeenCalledWith(
      {
        school_id: "1",
        $groups: { school: "1" },
        deployment: "moto-app.de",
      },
      false,
    );

    mocks.useTenant.mockReturnValue({
      tenantSlug: "school-b",
      routingMode: "path",
      tenant: { tenantId: 2 },
    });
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { tenantId: 2 } },
    });
    view.rerender(
      <TenantAuthWrapper>
        <div>Tenant content</div>
      </TenantAuthWrapper>,
    );

    expect(mocks.setPostHogContext).toHaveBeenNthCalledWith(
      2,
      {
        school_id: "2",
        $groups: { school: "2" },
        deployment: "moto-app.de",
      },
      true,
    );
  });
});
