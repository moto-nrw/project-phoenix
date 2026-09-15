import { Suspense } from "react";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const {
  mockCreateEnrollmentChangeRequest,
  mockFetchEnrollmentEditBootstrap,
  mockUpdateEnrollmentRequest,
} = vi.hoisted(() => ({
  mockCreateEnrollmentChangeRequest: vi.fn(),
  mockFetchEnrollmentEditBootstrap: vi.fn(),
  mockUpdateEnrollmentRequest: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/parents/anmeldung/status/tok/edit",
  useRouter: () => ({ push: vi.fn() }),
}));

vi.mock("next-intl", () => {
  const messages: Record<string, string> = {
    editFull: "Edit enrollment",
    editLoading: "Loading enrollment…",
    editLoadError: "Enrollment cannot be edited",
    editSaveError: "Changes could not be saved",
    editSubmit: "Save changes",
    backToStatus: "Back to status",
    adjustTitle: "Adjust offerings",
    adjustDescription: "Only offerings and weekdays",
    adjustSubmit: "Send adjustment",
  };
  const t = (key: string) => messages[key] ?? key;
  return {
    useTranslations: () => t,
  };
});

vi.mock("~/lib/enrollment-submission-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    createEnrollmentChangeRequest: mockCreateEnrollmentChangeRequest,
    fetchEnrollmentEditBootstrap: mockFetchEnrollmentEditBootstrap,
    updateEnrollmentRequest: mockUpdateEnrollmentRequest,
  };
});

vi.mock("~/lib/tenant-context", () => ({
  TenantProvider: ({ children }: { children: React.ReactNode }) => children,
}));

vi.mock("~/components/enrollment/enrollment-form", () => ({
  EnrollmentForm: ({
    submitLabel,
    gradeLevelMax,
    restrictToOfferings,
    lockChildStructure,
    submitter,
  }: {
    submitLabel: string;
    gradeLevelMax: number;
    restrictToOfferings?: boolean;
    lockChildStructure?: boolean;
    submitter: (payload: unknown) => Promise<unknown>;
  }) => (
    <button
      type="button"
      data-grade-level-max={gradeLevelMax}
      data-restrict-to-offerings={restrictToOfferings ? "true" : "false"}
      data-lock-child-structure={lockChildStructure ? "true" : "false"}
      onClick={() => void submitter({})}
    >
      {submitLabel}
    </button>
  ),
}));

import { EnrollmentEditPage } from "./enrollment-edit-page";

const bootstrap = {
  phase: {
    id: "5",
    name: "2026",
    kind: "school_year",
    service_start_date: "2026-08-01",
    service_end_date: "2027-07-31",
    show_status_reason_to_parent: true,
    care_offering_selection_mode: "optional",
  },
  schema: null,
  offerings: [],
  care_offering_selection_mode: "optional",
  care_required: false,
  grade_level_max: 13,
  legal_texts: { blocks: [] },
  draft: {
    request_id: "99",
    status_token: "tok",
    tenant_id: "1",
    tenant_subdomain: "demo",
    phase_id: "5",
    guardian_first_name: "Mara",
    guardian_last_name: "Muster",
    guardian_email: "mara@example.test",
    consent_flags: {},
    custom_data: {},
    children: [],
  },
};

describe("EnrollmentEditPage", () => {
  beforeEach(() => {
    mockFetchEnrollmentEditBootstrap.mockReset();
    mockCreateEnrollmentChangeRequest.mockReset();
    mockUpdateEnrollmentRequest.mockReset();
  });

  it("uses localized chrome and API fallbacks", async () => {
    let resolveBootstrap: (value: typeof bootstrap) => void = () => {};
    mockFetchEnrollmentEditBootstrap.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveBootstrap = resolve;
      }),
    );

    await act(async () => {
      render(
        <Suspense fallback={null}>
          <EnrollmentEditPage params={Promise.resolve({ token: "tok" })} />
        </Suspense>,
      );
    });

    expect(await screen.findByText("Loading enrollment…")).toBeInTheDocument();
    await act(async () => {
      resolveBootstrap(bootstrap);
    });
    expect(await screen.findByRole("heading")).toHaveTextContent(
      "Edit enrollment",
    );
    expect(screen.getByText("Back to status")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Save changes" }),
    ).toHaveAttribute("data-grade-level-max", "13");
    expect(mockFetchEnrollmentEditBootstrap).toHaveBeenCalledWith(
      "tok",
      "Enrollment cannot be edited",
    );
  });

  it("withholds the form when the bootstrap omits a supported grade cap", async () => {
    mockFetchEnrollmentEditBootstrap.mockResolvedValueOnce({
      ...bootstrap,
      grade_level_max: 0,
    });

    await act(async () => {
      render(
        <Suspense fallback={null}>
          <EnrollmentEditPage params={Promise.resolve({ token: "tok" })} />
        </Suspense>,
      );
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Enrollment cannot be edited",
    );
    expect(
      screen.queryByRole("button", { name: "Save changes" }),
    ).not.toBeInTheDocument();
  });
});

describe("EnrollmentEditPage adjustOnly (#2251)", () => {
  beforeEach(() => {
    mockFetchEnrollmentEditBootstrap.mockReset();
    mockUpdateEnrollmentRequest.mockReset();
  });

  it("renders the reduced heading and passes the restricted mode to the form", async () => {
    mockFetchEnrollmentEditBootstrap.mockResolvedValueOnce({
      ...bootstrap,
      edit_mode: "change_request",
    });

    await act(async () => {
      render(
        <Suspense fallback={null}>
          <EnrollmentEditPage
            params={Promise.resolve({ token: "tok" })}
            adjustOnly
          />
        </Suspense>,
      );
    });

    expect(
      await screen.findByRole("heading", { name: "Adjust offerings" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Only offerings and weekdays")).toBeInTheDocument();
    const form = screen.getByRole("button", { name: "Send adjustment" });
    expect(form.getAttribute("data-restrict-to-offerings")).toBe("true");
    expect(form.getAttribute("data-lock-child-structure")).toBe("true");
  });
});

describe("EnrollmentEditPage change requests", () => {
  it("refreshes request badges after submitting a change request", async () => {
    mockFetchEnrollmentEditBootstrap.mockResolvedValueOnce({
      ...bootstrap,
      edit_mode: "change_request",
    });
    mockCreateEnrollmentChangeRequest.mockResolvedValueOnce({
      request_id: "99",
    });
    const refreshListener = vi.fn();
    window.addEventListener("change-requests-refresh", refreshListener);

    await act(async () => {
      render(
        <Suspense fallback={null}>
          <EnrollmentEditPage params={Promise.resolve({ token: "tok" })} />
        </Suspense>,
      );
    });
    fireEvent.click(
      await screen.findByRole("button", { name: "changeRequestSubmit" }),
    );

    await waitFor(() => expect(refreshListener).toHaveBeenCalledTimes(1));
    window.removeEventListener("change-requests-refresh", refreshListener);
  });
});
