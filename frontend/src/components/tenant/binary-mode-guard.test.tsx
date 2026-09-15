import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

// Opt out of the global tenant-provider mock so the guard sees the real
// TenantContext wired up by its wrapping TenantProvider.
vi.unmock("~/lib/tenant-context");

import { TenantProvider } from "~/lib/tenant-context";
import { BinaryModeGuard } from "./binary-mode-guard";
import type { TenantInfo } from "~/lib/tenant-api";

// `notFound()` throws a Next.js-internal NEXT_NOT_FOUND error synchronously.
// We mock the module so the test can assert the throw without a Next runtime.
vi.mock("next/navigation", () => ({
  notFound: () => {
    throw new Error("NEXT_NOT_FOUND");
  },
}));

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
    staffMessagingEnabled: false,
    displayEnabled: false,
    gradeLevelMax: 4,
  };
}

describe("BinaryModeGuard", () => {
  it("renders children in detailed mode", () => {
    render(
      <TenantProvider tenantSlug="demo" tenant={makeTenant("detailed")}>
        <BinaryModeGuard>
          <div>protected-content</div>
        </BinaryModeGuard>
      </TenantProvider>,
    );
    expect(screen.getByText("protected-content")).toBeInTheDocument();
  });

  // Verhaltensänderung, ausdrücklich freigegeben: eine nicht eingeschaltete
  // Funktion ist ein Zustand, kein „nicht gefunden".
  it('zeigt den Zustand „nicht eingeschaltet" statt einer 404 im Binärmodus', () => {
    render(
      <TenantProvider tenantSlug="demo" tenant={makeTenant("binary")}>
        <BinaryModeGuard>
          <div>protected-content</div>
        </BinaryModeGuard>
      </TenantProvider>,
    );

    expect(screen.queryByText("protected-content")).not.toBeInTheDocument();
    expect(
      screen.getByText("Diese Funktion ist nicht eingeschaltet"),
    ).toBeInTheDocument();
  });

  it("renders children when no tenant context (safe default is detailed)", () => {
    // Outside a TenantProvider the hook returns "detailed" — first paint
    // on cold load should NOT accidentally 404 protected pages.
    render(
      <BinaryModeGuard>
        <div>protected-content</div>
      </BinaryModeGuard>,
    );
    expect(screen.getByText("protected-content")).toBeInTheDocument();
  });
});
