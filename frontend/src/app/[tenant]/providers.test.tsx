import type React from "react";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { TenantInfo } from "~/lib/tenant-api";
import { TenantProviders } from "./providers";

const { mockTenantProvider } = vi.hoisted(() => ({
  mockTenantProvider: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  SessionProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/components/notifications/service-worker-registrar", () => ({
  PushSubscriptionSync: () => null,
}));

vi.mock("~/components/auth/tenant-auth-wrapper", () => ({
  TenantAuthWrapper: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="tenant-auth-wrapper">{children}</div>
  ),
}));

vi.mock("~/lib/profile-context", () => ({
  ProfileProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/lib/supervision-context", () => ({
  SupervisionProvider: ({ children }: { children: React.ReactNode }) =>
    children,
}));

vi.mock("~/lib/tenant-context", () => ({
  TenantProvider: (props: {
    children: React.ReactNode;
    tenantSlug: string;
    tenant: TenantInfo;
    routingMode: "path" | "subdomain";
  }) => {
    mockTenantProvider(props);
    return <div data-testid="tenant-provider">{props.children}</div>;
  },
}));

describe("TenantProviders", () => {
  it("renders analytics hooks inside the tenant routing context", () => {
    const tenant = { subdomain: "school-a" } as TenantInfo;

    render(
      <TenantProviders tenantSlug="school-a" tenant={tenant} routingMode="path">
        <div>Tenant content</div>
      </TenantProviders>,
    );

    expect(screen.getByTestId("tenant-auth-wrapper").parentElement).toBe(
      screen.getByTestId("tenant-provider"),
    );
    expect(mockTenantProvider).toHaveBeenCalledWith(
      expect.objectContaining({
        tenantSlug: "school-a",
        tenant,
        routingMode: "path",
      }),
    );
  });
});
