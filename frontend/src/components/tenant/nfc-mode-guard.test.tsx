import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

// Opt out of the global tenant-provider mock so the guard sees the real
// TenantContext wired up by its wrapping TenantProvider.
vi.unmock("~/lib/tenant-context");
vi.unmock("~/components/tenant/tenant-provider");

import { TenantProvider } from "./tenant-provider";
import { NfcModeGuard } from "./nfc-mode-guard";
import type { TenantInfo } from "~/lib/tenant-api";

vi.mock("next/navigation", () => ({
  notFound: () => {
    throw new Error("NEXT_NOT_FOUND");
  },
}));

function makeTenant(nfcEnabled: boolean): TenantInfo {
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
    nfcEnabled,
  };
}

describe("NfcModeGuard", () => {
  it("renders children when NFC is enabled", () => {
    render(
      <TenantProvider tenantSlug="demo" tenant={makeTenant(true)}>
        <NfcModeGuard>
          <div>nfc-content</div>
        </NfcModeGuard>
      </TenantProvider>,
    );
    expect(screen.getByText("nfc-content")).toBeInTheDocument();
  });

  it("triggers notFound() when NFC is disabled", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(() =>
      render(
        <TenantProvider tenantSlug="demo" tenant={makeTenant(false)}>
          <NfcModeGuard>
            <div>nfc-content</div>
          </NfcModeGuard>
        </TenantProvider>,
      ),
    ).toThrow("NEXT_NOT_FOUND");

    errorSpy.mockRestore();
  });
});
