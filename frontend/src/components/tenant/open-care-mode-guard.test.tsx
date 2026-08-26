import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

// Opt out of the global tenant-provider mock so the guard sees the real
// TenantContext wired up by its wrapping TenantProvider.
vi.unmock("~/lib/tenant-context");

import { TenantProvider } from "~/lib/tenant-context";
import { OpenCareModeGuard } from "./open-care-mode-guard";
import type { TenantInfo } from "~/lib/tenant-api";

// `redirect()` throws a Next.js-internal error synchronously. We mock the
// module so the test can assert the throw (incl. target path) without a
// Next runtime.
vi.mock("next/navigation", () => ({
  redirect: (path: string) => {
    throw new Error(`NEXT_REDIRECT:${path}`);
  },
}));

function makeTenant(
  groupMode: "fixed_groups" | "open_care" = "fixed_groups",
): TenantInfo {
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
    displayEnabled: false,
    gradeLevelMax: 4,
    groupMode,
  };
}

describe("OpenCareModeGuard", () => {
  it("renders children in fixed-groups mode", () => {
    render(
      <TenantProvider tenantSlug="demo" tenant={makeTenant("fixed_groups")}>
        <OpenCareModeGuard>
          <div>protected-content</div>
        </OpenCareModeGuard>
      </TenantProvider>,
    );
    expect(screen.getByText("protected-content")).toBeInTheDocument();
  });

  it("redirects to the Kindersuche in open-care mode", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    expect(
      () =>
        render(
          <TenantProvider tenantSlug="demo" tenant={makeTenant("open_care")}>
            <OpenCareModeGuard>
              <div>protected-content</div>
            </OpenCareModeGuard>
          </TenantProvider>,
        ),
      // TenantProvider defaults to path routing in tests, so the target is
      // tenant-prefixed. Subdomain mode would redirect to /students/search.
    ).toThrow("NEXT_REDIRECT:/demo/students/search");

    errorSpy.mockRestore();
  });

  it("renders children when no tenant context (safe default is fixed_groups)", () => {
    // Outside a TenantProvider the hook returns false — first paint on cold
    // load should NOT bounce staff off the group page.
    render(
      <OpenCareModeGuard>
        <div>protected-content</div>
      </OpenCareModeGuard>,
    );
    expect(screen.getByText("protected-content")).toBeInTheDocument();
  });
});
