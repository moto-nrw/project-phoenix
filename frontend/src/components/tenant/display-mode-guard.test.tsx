import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

// Opt out of the global tenant-provider mock so the guard sees the real
// TenantContext wired up by its wrapping TenantProvider.
vi.unmock("~/lib/tenant-context");

import { TenantProvider } from "~/lib/tenant-context";
import { DisplayModeGuard } from "./display-mode-guard";
import type { TenantInfo } from "~/lib/tenant-api";

vi.mock("next/navigation", () => ({
  notFound: () => {
    throw new Error("NEXT_NOT_FOUND");
  },
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

  it("triggers notFound() when the Info-Point Dashboard is disabled", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(() =>
      render(
        <TenantProvider tenantSlug="demo" tenant={makeTenant(false)}>
          <DisplayModeGuard>
            <div>display-content</div>
          </DisplayModeGuard>
        </TenantProvider>,
      ),
    ).toThrow("NEXT_NOT_FOUND");

    errorSpy.mockRestore();
  });
});
