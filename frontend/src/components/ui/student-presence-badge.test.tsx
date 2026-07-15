import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

// Opt out of the global tenant-provider mock so the real TenantContext
// is wired in — the wrapper depends on it to decide between badges.
vi.unmock("~/lib/tenant-context");
vi.unmock("~/lib/tenant-context");

import { TenantProvider } from "~/lib/tenant-context";
import type { TenantInfo } from "~/lib/tenant-api";
import { StudentPresenceBadge } from "./student-presence-badge";

function makeTenant(
  presenceMode: "detailed" | "binary" = "detailed",
): TenantInfo {
  return {
    tenantId: 1,
    slug: "demo",
    name: "Demo",
    subdomain: "demo",
    organizationId: 10,
    organizationName: "Org",
    settings: {},
    presenceMode,
    studentPhotosEnabled: false,
    nfcEnabled: false,
    messagingEnabled: false,
    displayEnabled: false,
    gradeLevelMax: 4,
  };
}

describe("StudentPresenceBadge", () => {
  it("routes to LocationBadge in detailed mode (renders full room detail)", () => {
    render(
      <TenantProvider tenantSlug="demo" tenant={makeTenant("detailed")}>
        <StudentPresenceBadge
          student={{ current_location: "Anwesend - Kunstraum" }}
          displayMode="roomName"
          userGroups={[]}
          groupRooms={[]}
        />
      </TenantProvider>,
    );

    // LocationBadge renders the room name; PresenceBadge would collapse to "Anwesend".
    expect(screen.getByText("Kunstraum")).toBeInTheDocument();
  });

  it("routes to PresenceBadge in binary mode (collapses room detail)", () => {
    render(
      <TenantProvider tenantSlug="demo" tenant={makeTenant("binary")}>
        <StudentPresenceBadge
          student={{ current_location: "Anwesend - Kunstraum" }}
          displayMode="roomName"
          userGroups={[]}
          groupRooms={[]}
        />
      </TenantProvider>,
    );

    // PresenceBadge never leaks room names — that's its defense-in-depth promise.
    expect(screen.queryByText("Kunstraum")).not.toBeInTheDocument();
    expect(screen.getByText("Anwesend")).toBeInTheDocument();
  });

  it("defaults to detailed rendering when rendered outside a TenantProvider", () => {
    // Hook falls back to "detailed" — this is the safe first-paint behavior.
    render(
      <StudentPresenceBadge
        student={{ current_location: "Anwesend - Kunstraum" }}
        displayMode="roomName"
        userGroups={[]}
        groupRooms={[]}
      />,
    );
    expect(screen.getByText("Kunstraum")).toBeInTheDocument();
  });
});
