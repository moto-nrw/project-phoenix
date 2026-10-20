import { Suspense } from "react";
import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({
    tenantSlug: "demo",
    tenant: { name: "Demo School", gradeLevelMax: 4 },
  }),
  useTenantSlugSafe: () => "demo",
  useTenantRoutingModeSafe: () => "path",
}));

vi.mock("~/lib/enrollment-form-schema-api", () => ({
  fetchEnrollmentPreviewBootstrap: vi.fn(),
  schemaToPublicFormSchema: vi.fn(),
}));

vi.mock("~/components/enrollment/enrollment-form", () => ({
  EnrollmentForm: () => null,
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

import EnrollmentPreviewPage from "./page";

describe("EnrollmentPreviewPage", () => {
  // Der Rückweg zeigte auf `/enrollment-form` ohne Schule und landete im
  // Pfad-Modus außerhalb des Tenant-Portals (#4364).
  it("links back to the enrollment forms of the same school", async () => {
    await act(async () => {
      render(
        <Suspense fallback={null}>
          <EnrollmentPreviewPage />
        </Suspense>,
      );
    });

    expect(
      await screen.findByRole("link", {
        name: "Zurück zu den Anmeldeformularen",
      }),
    ).toHaveAttribute("href", "/demo/enrollment-form");
  });
});
