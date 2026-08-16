import { Suspense } from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  fetchBootstrap: vi.fn(),
  fetchProfile: vi.fn(),
  resolveTenant: vi.fn(),
  submitEnrollment: vi.fn(),
  enrollmentForm: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/lib/tenant-context", () => ({
  TenantProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/lib/tenant-api", () => ({
  resolveTenant: mocks.resolveTenant,
}));

vi.mock("~/lib/enrollment-submission-api", () => ({
  fetchParentEnrollmentBootstrap: mocks.fetchBootstrap,
}));

vi.mock("~/lib/parent-api", () => ({
  fetchParentEnrollmentProfile: mocks.fetchProfile,
  submitParentEnrollment: mocks.submitEnrollment,
}));

vi.mock("~/components/enrollment/enrollment-form", () => ({
  EnrollmentForm: (props: {
    gradeLevelMax: number;
    prefetchedData: { profile: unknown };
  }) => {
    mocks.enrollmentForm(props);
    return (
      <div
        data-testid="parent-enrollment-form"
        data-grade-level-max={props.gradeLevelMax}
        data-has-profile={String(props.prefetchedData.profile !== null)}
      />
    );
  },
}));

import ParentEnrollFormPage from "./page";

const tenant = {
  tenantId: 1,
  slug: "demo",
  name: "Demo School",
  subdomain: "demo",
  organizationId: 2,
  organizationName: "Demo Org",
  settings: {},
  presenceMode: "detailed",
  studentPhotosEnabled: false,
  nfcEnabled: false,
  messagingEnabled: false,
  displayEnabled: false,
  gradeLevelMax: 13,
};

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
  collect_grade_level: false,
  care_offerings_enabled: false,
  school_class: {
    collect: false,
    available_classes: [],
    require: false,
  },
  captcha_config: { enabled: true, site_key: "public-captcha" },
  legal_texts: { blocks: [] },
  // A tenant-session profile must never win on the parent portal.
  profile: {
    guardian: {
      first_name: "Wrong",
      last_name: "Session",
      email: "wrong@example.test",
    },
    children: [],
  },
};

const parentProfile = {
  guardian: {
    first_name: "Parent",
    last_name: "Portal",
    email: "parent@example.test",
  },
  children: [],
};

describe("ParentEnrollFormPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.resolveTenant.mockResolvedValue(tenant);
    mocks.fetchBootstrap.mockResolvedValue(bootstrap);
    mocks.fetchProfile.mockResolvedValue(parentProfile);
  });

  it("uses the resolved tenant cap and keeps autofill on the parent API", async () => {
    await act(async () => {
      render(
        <Suspense fallback={null}>
          <ParentEnrollFormPage
            params={Promise.resolve({ tenantSlug: "demo", phaseId: "5" })}
          />
        </Suspense>,
      );
    });

    const form = await screen.findByTestId("parent-enrollment-form");
    expect(
      screen.getByText("Anmeldung für die OGS-Betreuung"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zurück zum Elternportal" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Andere Anmeldung wählen" }),
    ).toHaveAttribute("href", "/parents/enroll");
    expect(form.closest(".w-full")).toBeInTheDocument();
    expect(form.closest(".max-w-4xl")).not.toBeInTheDocument();
    expect(form).toHaveAttribute("data-grade-level-max", "13");
    expect(form).toHaveAttribute("data-has-profile", "true");
    expect(mocks.resolveTenant).toHaveBeenCalledWith("demo");
    // #1663: the parents portal loads form metadata through the
    // parent-authenticated bootstrap, not the anonymous public path.
    expect(mocks.fetchBootstrap).toHaveBeenCalledWith("demo", "5", {
      lateInviteToken: undefined,
    });
    expect(mocks.fetchProfile).toHaveBeenCalledWith("demo");
    expect(mocks.enrollmentForm).toHaveBeenCalledWith(
      expect.objectContaining({
        gradeLevelMax: 13,
        prefetchedData: expect.objectContaining({
          collectGradeLevel: false,
          careOfferingsEnabled: false,
          captchaConfig: null,
          profile: parentProfile,
        }),
      }),
    );
  });

  it("shows an error instead of falling back when tenant metadata is unavailable", async () => {
    mocks.resolveTenant.mockResolvedValueOnce(null);

    await act(async () => {
      render(
        <Suspense fallback={null}>
          <ParentEnrollFormPage
            params={Promise.resolve({ tenantSlug: "demo", phaseId: "5" })}
          />
        </Suspense>,
      );
    });

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    await waitFor(() => {
      expect(
        screen.queryByTestId("parent-enrollment-form"),
      ).not.toBeInTheDocument();
    });
  });
});
