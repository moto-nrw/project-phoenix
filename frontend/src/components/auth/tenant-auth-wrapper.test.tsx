import { render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { TenantAuthWrapper } from "./tenant-auth-wrapper";

const mocks = vi.hoisted(() => ({
  useSession: vi.fn(),
  useTenant: vi.fn(),
  registerPortalSession: vi.fn(),
  clearPortalSession: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));
vi.mock("~/lib/hooks/use-user-context", () => ({
  useUserContext: () => ({ isReady: true }),
}));
vi.mock("~/lib/hooks/use-global-sse", () => ({
  useGlobalSSE: () => ({ status: "connected" }),
}));
vi.mock("~/lib/analytics", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/analytics")>()),
  registerPortalSession: mocks.registerPortalSession,
  clearPortalSession: mocks.clearPortalSession,
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

  it("registers school and role after the URL tenant matches the session tenant", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { tenantId: 2 } },
    });

    renderWrapper();

    expect(mocks.registerPortalSession).toHaveBeenCalledWith(
      { surface: "ogs", schoolId: "2", role: "staff" },
      false,
    );
  });

  it("sends the admin flag as role, never a school's role name", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { tenantId: 2, isAdmin: true, roles: ["OGS-Leitung"] } },
    });

    renderWrapper();

    expect(mocks.registerPortalSession).toHaveBeenCalledWith(
      { surface: "ogs", schoolId: "2", role: "admin" },
      false,
    );
  });

  it("clears context while TenantGuard switches tenants", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { tenantId: 1 } },
    });

    const view = renderWrapper();

    expect(mocks.registerPortalSession).not.toHaveBeenCalled();
    expect(mocks.clearPortalSession).toHaveBeenCalledOnce();

    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { tenantId: 2 } },
    });
    view.rerender(
      <TenantAuthWrapper>
        <div>Tenant content</div>
      </TenantAuthWrapper>,
    );

    expect(mocks.registerPortalSession).toHaveBeenCalledWith(
      { surface: "ogs", schoolId: "2", role: "staff" },
      false,
    );
  });

  // #3603: with the school's Analyse-Freigabe the account joins the context
  // as its pseudonymous ID (the same hash the backend computes).
  it("registers the Analyse-Freigabe with the pseudonymous person", async () => {
    mocks.useTenant.mockReturnValue({
      tenantSlug: "school-b",
      routingMode: "path",
      tenant: {
        tenantId: 42,
        analyticsFreigabe: true,
        analyticsRecordingSamplePercent: 30,
      },
    });
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "7", tenantId: 42 } },
    });

    renderWrapper();

    await waitFor(() =>
      expect(mocks.registerPortalSession).toHaveBeenLastCalledWith(
        {
          surface: "ogs",
          schoolId: "42",
          role: "staff",
          analyseFreigabe: true,
          recordingSamplePercent: 30,
          person: "pseudo_4b9630678fc1afce76ce690721aaf949",
        },
        false,
      ),
    );
  });

  it("names no person in the read-only staff preview", async () => {
    mocks.useTenant.mockReturnValue({
      tenantSlug: "school-b",
      routingMode: "path",
      tenant: {
        tenantId: 42,
        analyticsFreigabe: true,
        analyticsRecordingSamplePercent: 30,
      },
    });
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "7", tenantId: 42, isPreview: true } },
    });

    renderWrapper();

    await waitFor(() =>
      expect(mocks.registerPortalSession).toHaveBeenCalledWith(
        expect.objectContaining({ analyseFreigabe: true, person: null }),
        false,
      ),
    );
    expect(mocks.registerPortalSession).not.toHaveBeenCalledWith(
      expect.objectContaining({
        person: expect.stringMatching(/^pseudo_/) as unknown,
      }),
      expect.anything(),
    );
  });

  it("clears context on logout", () => {
    mocks.useSession.mockReturnValue({ status: "unauthenticated", data: null });

    renderWrapper();

    expect(mocks.registerPortalSession).not.toHaveBeenCalled();
    expect(mocks.clearPortalSession).toHaveBeenCalledOnce();
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

    expect(mocks.registerPortalSession).toHaveBeenCalledWith(
      { surface: "ogs", schoolId: "1", role: "staff" },
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

    expect(mocks.registerPortalSession).toHaveBeenNthCalledWith(
      2,
      { surface: "ogs", schoolId: "2", role: "staff" },
      true,
    );
  });
});
