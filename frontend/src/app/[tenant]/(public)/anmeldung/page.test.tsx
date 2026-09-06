import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  fetchPublicPhases: vi.fn(),
  tenant: {
    tenantSlug: "demo",
    tenant: { name: "Demo School" },
  },
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => mocks.tenant,
  useTenantSlugSafe: () => mocks.tenant.tenantSlug,
  useTenantRoutingModeSafe: () => "path",
}));

vi.mock("~/lib/enrollment-submission-api", () => ({
  fetchPublicPhases: mocks.fetchPublicPhases,
}));

vi.mock("~/components/enrollment/public-enrollment-shell", () => ({
  PublicEnrollmentBrand: () => null,
  PublicEnrollmentLocaleSwitcher: () => null,
  PublicEnrollmentPageShell: ({ children }: { children: React.ReactNode }) => (
    <main>{children}</main>
  ),
  PublicEnrollmentSteps: () => null,
  PublicInfoCard: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}));

import EnrollPhasePickerPage from "./page";

describe("EnrollPhasePickerPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.fetchPublicPhases.mockResolvedValue([
      {
        id: "phase/1",
        name: "Schuljahr 2026/27",
        kind: "school_year",
        service_start_date: "2026-08-01",
        service_end_date: "2027-07-31",
        show_status_reason_to_parent: false,
        care_offering_selection_mode: "optional",
      },
    ]);
  });

  it("keeps the tenant in phase links in path mode", async () => {
    render(<EnrollPhasePickerPage />);

    expect(
      await screen.findByRole("link", { name: /Schuljahr 2026\/27/ }),
    ).toHaveAttribute("href", "/demo/anmeldung/phase%2F1");
  });
});
