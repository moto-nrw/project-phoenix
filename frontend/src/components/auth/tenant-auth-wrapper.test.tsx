import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { TenantAuthWrapper } from "./tenant-auth-wrapper";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  useTenant: vi.fn(),
  register: vi.fn(),
  unregister: vi.fn(),
  reset: vi.fn(),
  trackPageView: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));
vi.mock("next/navigation", () => ({
  usePathname: () => "/school-b/dashboard",
}));
vi.mock("posthog-js", () => ({
  default: {
    register: mocks.register,
    unregister: mocks.unregister,
    reset: mocks.reset,
  },
}));
vi.mock("~/env", () => ({
  env: { NEXT_PUBLIC_TENANT_DOMAIN: "moto-app.de" },
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

    expect(mocks.register).toHaveBeenCalledWith({
      school_id: "2",
      $groups: { school: "2" },
      deployment: "moto-app.de",
    });
    expect(mocks.trackPageView).toHaveBeenCalledWith("/dashboard", "2");
  });

  it("clears context and skips capture while TenantGuard switches tenants", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { tenantId: 1 } },
    });

    const view = renderWrapper();

    expect(mocks.register).not.toHaveBeenCalled();
    expect(mocks.trackPageView).not.toHaveBeenCalled();
    expect(mocks.unregister).toHaveBeenCalledWith("school_id");
    expect(mocks.unregister).toHaveBeenCalledWith("$groups");
    expect(mocks.unregister).toHaveBeenCalledWith("deployment");
    expect(mocks.reset).not.toHaveBeenCalled();

    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { tenantId: 2 } },
    });
    view.rerender(
      <TenantAuthWrapper>
        <div>Tenant content</div>
      </TenantAuthWrapper>,
    );

    expect(mocks.register).toHaveBeenCalledWith({
      school_id: "2",
      $groups: { school: "2" },
      deployment: "moto-app.de",
    });
    expect(mocks.trackPageView).toHaveBeenCalledWith("/dashboard", "2");
  });
});
