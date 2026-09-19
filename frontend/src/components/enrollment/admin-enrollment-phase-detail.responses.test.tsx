import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  getCareUsageReport: vi.fn(),
  listAdminRequests: vi.fn(),
  listPhases: vi.fn(),
  responses: undefined as unknown,
  responseError: undefined as Error | undefined,
}));

vi.mock("~/lib/enrollment-admin-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return { ...actual, listAdminRequests: mocks.listAdminRequests };
});

vi.mock("~/lib/enrollment-phase-api", () => ({
  listPhases: mocks.listPhases,
  getPhaseResponseOverview: vi.fn(),
}));

vi.mock("~/lib/enrollment-export-api", () => ({
  exportPhaseRegistrations: vi.fn(),
}));

vi.mock("~/lib/enrollment-report-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return { ...actual, getCareUsageReport: mocks.getCareUsageReport };
});

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) =>
    key?.startsWith("enrollment-phase-responses-")
      ? { data: mocks.responses, error: mocks.responseError, isLoading: false }
      : { data: ["1a"], error: undefined, isLoading: false },
}));

vi.mock("~/lib/tenant-context", () => ({
  useCareOfferingsEnabled: () => true,
  useTenantRoutingModeSafe: () => "path",
  useTenantSlugSafe: () => "demo",
}));

vi.mock("~/components/ui/mobile-back-button", () => ({
  MobileBackButton: () => null,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ error: vi.fn(), success: vi.fn() }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/lib/enrollment-public-url", () => ({
  useEnrollmentPublicUrl: () => "http://demo.localhost:3000/anmeldung/1",
}));

vi.mock("~/lib/api", () => ({
  studentService: { getStudents: vi.fn() },
}));

vi.mock("~/components/enrollment/phase-response-overview", () => ({
  PhaseResponseOverview: ({ search }: { search: string }) => (
    <div data-testid="response-overview">Suche: {search}</div>
  ),
}));

import { AdminEnrollmentPhaseDetail } from "./admin-enrollment-phase-detail";

const phase = {
  id: "1",
  name: "Anmeldung 2027/2028",
  kind: "school_year",
  service_start_date: "2027-08-01",
  service_end_date: "2028-07-31",
  enrollment_open_at: null,
  enrollment_close_at: null,
  form_schema_id: null,
  show_status_reason_to_parent: false,
  care_overflow_mode: "waitlist",
  care_offering_selection_mode: "optional",
  is_active: true,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const emptyReport = {
  phase: { id: "1", name: phase.name },
  filters: { phase_id: "1", status: "all", care_offering_ids: [] },
  totals: {
    children: 0,
    by_day_count: { "1": 0, "2": 0, "3": 0, "4": 0, "5": 0 },
  },
  by_offering: [],
  filter_options: { offerings: [], grade_levels: [] },
  rows: [],
};

beforeEach(() => {
  vi.clearAllMocks();
  mocks.listPhases.mockResolvedValue([phase]);
  mocks.listAdminRequests.mockResolvedValue([]);
  mocks.getCareUsageReport.mockResolvedValue(emptyReport);
  mocks.responseError = undefined;
  mocks.responses = {
    applicable: true,
    expected: 100,
    responded: 80,
    children: [],
    excluded: [],
  };
});

describe("AdminEnrollmentPhaseDetail: Rücklauf (#3379)", () => {
  it("adds the tab with the number of children still missing", async () => {
    render(<AdminEnrollmentPhaseDetail phaseId="1" />);

    const tab = await screen.findByRole("tab", { name: /Rücklauf/ });
    expect(tab).toHaveTextContent("20");
    expect(
      screen.getByRole("tab", { name: /Anmeldungen/ }),
    ).toBeInTheDocument();
    // The list of requests stays the first thing the school sees.
    expect(screen.queryByTestId("response-overview")).not.toBeInTheDocument();
  });

  it("shows the overview on its tab and hands it the page search", async () => {
    render(<AdminEnrollmentPhaseDetail phaseId="1" />);

    fireEvent.click(await screen.findByRole("tab", { name: /Rücklauf/ }));

    expect(await screen.findByTestId("response-overview")).toBeInTheDocument();
    fireEvent.change(
      screen.getAllByPlaceholderText("Nach Kind oder Klasse suchen…")[0]!,
      { target: { value: "arslan" } },
    );
    await waitFor(() =>
      expect(screen.getByTestId("response-overview")).toHaveTextContent(
        "Suche: arslan",
      ),
    );
  });

  it("has no tab for a phase that does not pin existing children", async () => {
    mocks.responses = {
      applicable: false,
      expected: 0,
      responded: 0,
      children: [],
      excluded: [],
    };
    render(<AdminEnrollmentPhaseDetail phaseId="1" />);

    expect(await screen.findByText("Anmeldung 2027/2028")).toBeInTheDocument();
    expect(
      screen.queryByRole("tab", { name: /Rücklauf/ }),
    ).not.toBeInTheDocument();
  });

  it("shows the response error while keeping the registrations available", async () => {
    mocks.responses = undefined;
    mocks.responseError = new Error("Rücklauf konnte nicht geladen werden");
    render(<AdminEnrollmentPhaseDetail phaseId="1" />);

    expect(await screen.findByText("Anmeldung 2027/2028")).toBeInTheDocument();
    expect(screen.getByText("Rücklauf nicht geladen")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Der Rücklauf konnte nicht geladen werden. Die Anmeldungen bleiben verfügbar.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("tab", { name: /Rücklauf/ }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Eingänge")).toBeInTheDocument();
  });
});
