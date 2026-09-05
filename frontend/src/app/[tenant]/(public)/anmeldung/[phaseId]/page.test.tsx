import { Suspense } from "react";
import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  fetchBootstrap: vi.fn(),
  enrollmentForm: vi.fn(),
  tenant: {
    tenantSlug: "demo",
    tenant: { name: "Demo School", gradeLevelMax: 12 },
  },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => mocks.tenant,
}));

vi.mock("~/lib/enrollment-submission-api", () => ({
  fetchPublicEnrollmentBootstrap: mocks.fetchBootstrap,
}));

vi.mock("~/components/enrollment/enrollment-form", () => ({
  EnrollmentForm: (props: { gradeLevelMax: number }) => {
    mocks.enrollmentForm(props);
    return (
      <div
        data-testid="public-enrollment-form"
        data-grade-level-max={props.gradeLevelMax}
      />
    );
  },
}));

vi.mock("~/components/enrollment/public-enrollment-shell", () => ({
  PublicEnrollmentBackLink: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
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

import EnrollPhaseFormPage from "./page";

const bootstrap = {
  phase: {
    id: "5",
    name: "2026/27",
    kind: "school_year",
    service_start_date: "2026-08-01",
    service_end_date: "2027-07-31",
    show_status_reason_to_parent: false,
    care_offering_selection_mode: "optional",
  },
  schema: null,
  offerings: [],
  care_offering_selection_mode: "optional",
  care_required: false,
  school_class: {
    collect: false,
    available_classes: [],
    require: false,
  },
  captcha_config: null,
  legal_texts: { blocks: [] },
  profile: null,
};

describe("EnrollPhaseFormPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.fetchBootstrap.mockResolvedValue(bootstrap);
    mocks.tenant.tenant = { name: "Demo School", gradeLevelMax: 12 };
  });

  it("passes the tenant-configured cap to the public form", async () => {
    await act(async () => {
      render(
        <Suspense fallback={null}>
          <EnrollPhaseFormPage
            params={Promise.resolve({ tenant: "demo", phaseId: "5" })}
          />
        </Suspense>,
      );
    });

    expect(await screen.findByTestId("public-enrollment-form")).toHaveAttribute(
      "data-grade-level-max",
      "12",
    );
    expect(mocks.enrollmentForm).toHaveBeenCalledWith(
      expect.objectContaining({ gradeLevelMax: 12 }),
    );
  });
});
