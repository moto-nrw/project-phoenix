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

  // Verhaltensänderung, ausdrücklich freigegeben. Info-Displays sind NICHT
  // operator-vorbehalten, deshalb nennt der Satz die Einstellungen.
  it('zeigt den Zustand „nicht eingeschaltet" statt einer 404, wenn Info-Displays aus sind', () => {
    render(
      <TenantProvider tenantSlug="demo" tenant={makeTenant(false)}>
        <DisplayModeGuard>
          <div>display-content</div>
        </DisplayModeGuard>
      </TenantProvider>,
    );

    expect(screen.queryByText("display-content")).not.toBeInTheDocument();
    expect(
      screen.getByText("Diese Funktion ist nicht eingeschaltet"),
    ).toBeInTheDocument();
    expect(screen.getByText(/in den Einstellungen/)).toBeInTheDocument();
  });
});
