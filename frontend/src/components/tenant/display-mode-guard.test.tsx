import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

// Opt out of the global tenant-provider mock so the guard sees the real
// TenantContext wired up by its wrapping TenantProvider.
vi.unmock("~/lib/tenant-context");

import { TenantProvider } from "~/lib/tenant-context";
import { DisplayModeGuard } from "./display-mode-guard";
import type { TenantInfo } from "~/lib/tenant-api";

// The FeatureDisabledPage inside the guard navigates via useTenantRouter,
// which needs next/navigation's useRouter under jsdom.
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}));

function makeTenant(displayEnabled: boolean): TenantInfo {
  return {
    tenantId: 1,
    slug: "demo",
    name: "Demo",
    subdomain: "demo",
    organizationId: 10,
    organizationName: "Org",
    settings: {},
    presenceMode: "detailed",
    studentPhotosEnabled: false,
    nfcEnabled: false,
    messagingEnabled: false,
    staffMessagingEnabled: false,
    displayEnabled,
    gradeLevelMax: 4,
  };
}

describe("DisplayModeGuard", () => {
  it("renders children when the Info-Point Dashboard is enabled", () => {
    render(
      <TenantProvider tenantSlug="demo" tenant={makeTenant(true)}>
        <DisplayModeGuard>
          <div>display-content</div>
        </DisplayModeGuard>
      </TenantProvider>,
    );
    expect(screen.getByText("display-content")).toBeInTheDocument();
  });

  it("shows the feature-disabled page when the Info-Point Dashboard is disabled (#2624)", () => {
    render(
      <TenantProvider tenantSlug="demo" tenant={makeTenant(false)}>
        <DisplayModeGuard>
          <div>display-content</div>
        </DisplayModeGuard>
      </TenantProvider>,
    );

    expect(
      screen.getByText("Diese Funktion ist ausgeschaltet"),
    ).toBeInTheDocument();
    expect(screen.queryByText("display-content")).not.toBeInTheDocument();
  });
});
