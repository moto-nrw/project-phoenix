import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  decideAdminChild: vi.fn(),
  exportCareUsageReport: vi.fn(),
  exportPhaseClassRoster: vi.fn(),
  exportPhaseRegistrations: vi.fn(),
  getCareUsageReport: vi.fn(),
  listAdminRequests: vi.fn(),
  listPhases: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
  useCareOfferingsEnabled: vi.fn(),
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/lib/enrollment-admin-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    decideAdminChild: mocks.decideAdminChild,
    listAdminRequests: mocks.listAdminRequests,
  };
});

vi.mock("~/lib/enrollment-phase-api", () => ({
  listPhases: mocks.listPhases,
}));

vi.mock("~/lib/enrollment-export-api", () => ({
  exportPhaseRegistrations: mocks.exportPhaseRegistrations,
}));

vi.mock("~/lib/enrollment-report-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    exportCareUsageReport: mocks.exportCareUsageReport,
    exportPhaseClassRoster: mocks.exportPhaseClassRoster,
    getCareUsageReport: mocks.getCareUsageReport,
  };
});

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({
    data: ["1a", "2b"],
    error: undefined,
    isLoading: false,
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useCareOfferingsEnabled: mocks.useCareOfferingsEnabled,
  useTenantRoutingModeSafe: () => "path",
  useTenantSlugSafe: () => "demo",
}));

vi.mock("~/components/ui/mobile-back-button", () => ({
  MobileBackButton: () => null,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    error: mocks.toastError,
    success: mocks.toastSuccess,
  }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: mocks.useSetBreadcrumb,
}));

vi.mock("~/lib/enrollment-public-url", () => ({
  useEnrollmentPublicUrl: () => "http://demo.localhost:3000/enroll/1",
}));

vi.mock("~/lib/api", () => ({
  studentService: {
    getStudents: vi.fn(),
  },
}));

import { AdminEnrollmentPhaseDetail } from "./admin-enrollment-phase-detail";

const phase = {
  id: "1",
  name: "Demo Anmeldung 2026/2027",
  kind: "school_year",
  service_start_date: "2026-08-18",
  service_end_date: "2027-07-18",
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

const requests = [
  {
    id: "10",
    phase_id: "1",
    phase_name: phase.name,
    guardian_first_name: "Eva",
    guardian_last_name: "Muster",
    guardian_email: "eva@example.test",
    submitted_at: "2026-06-18T11:15:00Z",
    children: [
      {
        id: "20",
        first_name: "Lina",
        last_name: "Muster",
        date_of_birth: "2019-01-01",
        target_grade_level: 1,
        status: "submitted",
        activation_mode: "manual",
      },
      {
        id: "21",
        first_name: "Tom",
        last_name: "Muster",
        date_of_birth: "2018-01-01",
        target_grade_level: 2,
        status: "approved",
        activation_mode: "manual",
      },
    ],
  },
];

function report(overrides: Record<string, unknown> = {}) {
  return {
    phase: { id: "1", name: phase.name },
    filters: { phase_id: "1", status: "all", care_offering_ids: ["1", "2"] },
    totals: {
      children: 2,
      by_day_count: { "1": 1, "2": 0, "3": 1, "4": 0, "5": 0 },
    },
    by_offering: [],
    filter_options: {
      offerings: [
        { id: "1", name: "Kurzbetreuung", counts_as_care: true },
        { id: "2", name: "OGS Ganztag", counts_as_care: true },
        { id: "3", name: "Randstunde", counts_as_care: false },
      ],
      grade_levels: [1, 2],
    },
    rows: [
      {
        request_id: "10",
        child_id: "20",
        child_first_name: "Lina",
        child_last_name: "Muster",
        date_of_birth: "2019-01-01",
        target_grade_level: 1,
        status: "submitted",
        offerings: [
          {
            id: "2",
            name: "OGS Ganztag",
            days: ["mon", "wed", "fri"],
            days_source: "selected",
            days_of_week_mode: "parent_choice",
          },
          {
            id: "3",
            name: "Randstunde",
            days: ["mon", "tue", "wed", "thu", "fri"],
            days_source: "selected",
            days_of_week_mode: "parent_choice",
            manual_selected_days: ["fri"],
            automatic_selected_days: ["mon", "tue", "wed", "thu"],
          },
        ],
        effective_days: ["mon", "wed", "fri"],
        day_count: 3,
        guardian_first_name: "Eva",
        guardian_last_name: "Muster",
        guardian_email: "eva@example.test",
        submitted_at: "2026-06-18T11:15:00Z",
      },
      {
        request_id: "10",
        child_id: "21",
        child_first_name: "Tom",
        child_last_name: "Muster",
        date_of_birth: "2018-01-01",
        target_grade_level: 2,
        status: "approved",
        offerings: [
          {
            id: "1",
            name: "Kurzbetreuung",
            days: ["tue"],
            days_source: "selected",
            days_of_week_mode: "parent_choice",
          },
        ],
        effective_days: ["tue"],
        day_count: 1,
        guardian_first_name: "Eva",
        guardian_last_name: "Muster",
        guardian_email: "eva@example.test",
        submitted_at: "2026-06-18T11:15:00Z",
      },
    ],
    ...overrides,
  };
}

async function renderPhase() {
  render(<AdminEnrollmentPhaseDetail phaseId="1" />);
  // Die Filter sitzen seit der UI-Vereinheitlichung in der Kopfkarte des
  // Seitengerüsts; der Angebotsfilter steht, sobald die Phase geladen ist.
  await waitFor(() => {
    expect(
      screen.getByRole("button", { name: /Berücksichtigte Angebote/ }),
    ).toBeVisible();
  });
}

beforeEach(() => {
  for (const mock of Object.values(mocks)) {
    mock.mockReset();
  }
  mocks.listPhases.mockResolvedValue([phase]);
  mocks.listAdminRequests.mockResolvedValue(requests);
  mocks.getCareUsageReport.mockImplementation(async (filters) =>
    report({ filters }),
  );
  mocks.exportCareUsageReport.mockResolvedValue(undefined);
  mocks.exportPhaseClassRoster.mockResolvedValue(undefined);
  mocks.exportPhaseRegistrations.mockResolvedValue(undefined);
  mocks.decideAdminChild.mockResolvedValue({ id: "20", status: "approved" });
  mocks.useCareOfferingsEnabled.mockReturnValue(true);
});

describe("AdminEnrollmentPhaseDetail", () => {
  it("keeps the tenant in the parent enrollment link in path routing", async () => {
    await renderPhase();

    expect(
      screen.getByRole("link", { name: "Elternansicht öffnen" }),
    ).toHaveAttribute("href", "/demo/enroll/1");
  });

  it("renders care offerings and starts with all statuses", async () => {
    await renderPhase();

    expect(mocks.getCareUsageReport).toHaveBeenCalledWith(
      expect.objectContaining({ phase_id: "1", status: "all" }),
    );
    expect(mocks.getCareUsageReport.mock.calls[0]?.[0]).not.toHaveProperty(
      "care_offering_ids",
    );
    expect(screen.getByRole("button", { name: "Alle Status" })).toBeVisible();
    expect(
      screen.getByRole("button", { name: /Berücksichtigte Angebote/ }),
    ).toHaveTextContent("2 Angebote");
    expect(screen.getAllByText("OGS Ganztag").length).toBeGreaterThan(0);
    expect(screen.getByText("Mo, Mi, Fr")).toBeVisible();
    expect(screen.getAllByText("Randstunde").length).toBeGreaterThan(0);
    expect(
      screen.getByText(
        "Mo, Di, Mi, Do automatisch mitgebucht; Fr von Eltern gewählt",
      ),
    ).toBeVisible();
    fireEvent.click(
      screen.getByRole("button", { name: /Berücksichtigte Angebote/ }),
    );
    const offeringMenu = screen.getByRole("dialog", {
      name: /Berücksichtigte Angebote/,
    });
    expect(
      within(offeringMenu).getByRole("checkbox", { name: /Kurzbetreuung/ }),
    ).toBeChecked();
    expect(
      within(offeringMenu).getByRole("checkbox", { name: /OGS Ganztag/ }),
    ).toBeChecked();
    expect(
      within(offeringMenu).getByRole("checkbox", { name: /Randstunde/ }),
    ).not.toBeChecked();
    expect(
      within(offeringMenu).getByText("Zählt nicht als Betreuungstag"),
    ).toBeVisible();
    expect(screen.getByText("3 Betreuungstage")).toBeVisible();
    expect(screen.getByText("1 Tag")).toBeVisible();
  });

  it("reloads and exports the table with the active offering filter", async () => {
    await renderPhase();

    fireEvent.click(
      screen.getByRole("button", { name: /Berücksichtigte Angebote/ }),
    );
    fireEvent.click(screen.getByRole("checkbox", { name: /OGS Ganztag/ }));

    await waitFor(() => {
      expect(mocks.getCareUsageReport).toHaveBeenLastCalledWith(
        expect.objectContaining({ care_offering_ids: ["1"] }),
      );
    });

    expect(
      screen.getByRole("button", { name: "Anmeldungen exportieren" }),
    ).toBeVisible();

    fireEvent.click(
      screen.getByRole("button", { name: "Auswertung exportieren" }),
    );
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Als Excel-Datei exportieren" }),
    );

    await waitFor(() => {
      expect(mocks.exportCareUsageReport).toHaveBeenCalledWith(
        expect.objectContaining({ care_offering_ids: ["1"], status: "all" }),
        "xlsx",
      );
    });
  });

  it("allows clearing the final care offering and exports the empty selection", async () => {
    await renderPhase();

    fireEvent.click(
      screen.getByRole("button", { name: /Berücksichtigte Angebote/ }),
    );
    fireEvent.click(screen.getByRole("checkbox", { name: /OGS Ganztag/ }));

    await waitFor(() => {
      expect(mocks.getCareUsageReport).toHaveBeenLastCalledWith(
        expect.objectContaining({ care_offering_ids: ["1"] }),
      );
    });

    // Das Menü bleibt nach einer Auswahl offen; ein zweiter Klick auf den
    // Auslöser würde es schließen.
    fireEvent.click(screen.getByRole("checkbox", { name: /Kurzbetreuung/ }));

    await waitFor(() => {
      expect(mocks.getCareUsageReport).toHaveBeenLastCalledWith(
        expect.objectContaining({ care_offering_ids: [] }),
      );
    });
    expect(
      screen.getByRole("button", { name: /Berücksichtigte Angebote/ }),
    ).toHaveTextContent("Keine Angebote");

    fireEvent.click(
      screen.getByRole("button", { name: "Auswertung exportieren" }),
    );
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Als Excel-Datei exportieren" }),
    );

    await waitFor(() => {
      expect(mocks.exportCareUsageReport).toHaveBeenCalledWith(
        expect.objectContaining({ care_offering_ids: [] }),
        "xlsx",
      );
    });
  });

  it("exports the report as Word from the stats card", async () => {
    await renderPhase();

    fireEvent.click(
      screen.getByRole("button", { name: "Auswertung exportieren" }),
    );
    fireEvent.click(
      screen.getByRole("menuitem", {
        name: "Als Word-Dokument exportieren (ausführlich)",
      }),
    );

    await waitFor(() => {
      expect(mocks.exportCareUsageReport).toHaveBeenCalledWith(
        expect.objectContaining({ status: "all" }),
        "docx",
      );
    });
  });

  it("exports the report as a compact table and forwards the active filters", async () => {
    await renderPhase();

    fireEvent.click(
      screen.getByRole("button", { name: "Auswertung exportieren" }),
    );
    expect(
      screen.getByRole("menuitem", {
        name: "Als Word-Dokument exportieren (kompakte Tabelle)",
      }),
    ).toBeVisible();
    fireEvent.click(
      screen.getByRole("menuitem", {
        name: "Als PDF exportieren (kompakte Tabelle)",
      }),
    );

    await waitFor(() => {
      expect(mocks.exportCareUsageReport).toHaveBeenCalledWith(
        expect.objectContaining({ status: "all" }),
        "pdf",
        "compact",
      );
    });
  });

  it("exports a phase-aware class roster for the selected class", async () => {
    await renderPhase();

    fireEvent.click(
      screen.getByRole("combobox", { name: "Klasse für Klassenliste" }),
    );
    fireEvent.click(screen.getByRole("option", { name: "1a" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Klassenliste exportieren" }),
    );
    const rosterMenu = screen.getByRole("menu", {
      name: "Klassenliste exportieren",
    });
    fireEvent.click(
      within(rosterMenu).getByRole("menuitem", {
        name: "Als PDF exportieren",
      }),
    );

    await waitFor(() => {
      expect(mocks.exportPhaseClassRoster).toHaveBeenCalledWith(
        "1",
        "1a",
        "pdf",
      );
    });
  });

  it("ignores infeasible day-count buckets and preserves a zero-day filter", async () => {
    mocks.getCareUsageReport.mockImplementation(async (filters) =>
      report({
        filters,
        totals: {
          children: 1,
          by_day_count: { "0": 1, "6": 2, "7": 3 },
        },
        rows: [
          {
            request_id: "10",
            child_id: "20",
            child_first_name: "Lina",
            child_last_name: "Muster",
            date_of_birth: "2019-01-01",
            target_grade_level: 1,
            status: "submitted",
            offerings: [
              {
                id: "2",
                name: "OGS Ganztag",
                days: [],
                days_source: "selected",
                days_of_week_mode: "parent_choice",
              },
            ],
            effective_days: [],
            day_count: 0,
            guardian_first_name: "Eva",
            guardian_last_name: "Muster",
            guardian_email: "eva@example.test",
            submitted_at: "2026-06-18T11:15:00Z",
          },
        ],
      }),
    );

    await renderPhase();

    expect(screen.getByText("0 Betreuungstage")).toBeVisible();
    expect(screen.queryByText("6 Tage")).not.toBeInTheDocument();
    expect(screen.queryByText("7 Tage")).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Alle Betreuungstage" }),
    );
    expect(
      screen.queryByRole("button", { name: "6 Tage" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "7 Tage" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "0 Tage" }));

    await waitFor(() => {
      expect(mocks.getCareUsageReport).toHaveBeenLastCalledWith(
        expect.objectContaining({ day_count: 0 }),
      );
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Auswertung exportieren" }),
    );
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Als Excel-Datei exportieren" }),
    );

    await waitFor(() => {
      expect(mocks.exportCareUsageReport).toHaveBeenCalledWith(
        expect.objectContaining({ day_count: 0 }),
        "xlsx",
      );
    });
  });

  it("does not leave stale report rows visible after a failed filtered reload", async () => {
    await renderPhase();
    expect(screen.getByText("Lina Muster")).toBeVisible();

    mocks.getCareUsageReport.mockRejectedValueOnce(
      new Error("Auswertung konnte nicht geladen werden"),
    );

    fireEvent.click(
      screen.getByRole("button", { name: /Berücksichtigte Angebote/ }),
    );
    fireEvent.click(screen.getByRole("checkbox", { name: /OGS Ganztag/ }));

    await waitFor(() => {
      expect(
        screen.getByText("Auswertung konnte nicht geladen werden"),
      ).toBeVisible();
    });
    expect(screen.queryByText("Lina Muster")).not.toBeInTheDocument();
    expect(screen.queryByText("Tom Muster")).not.toBeInTheDocument();
  });

  it("keeps quick decisions on non-terminal rows", async () => {
    await renderPhase();

    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() => {
      expect(mocks.decideAdminChild).toHaveBeenCalledWith(
        "10",
        "20",
        "approved",
      );
    });
    expect(mocks.toastSuccess).toHaveBeenCalledWith(
      "Entscheidung gespeichert: Bestätigt",
    );
  });

  it("warns before a quick approval in an optional phase without an offering", async () => {
    mocks.getCareUsageReport.mockResolvedValue(
      report({
        rows: [
          {
            ...report().rows[0],
            offerings: [],
            effective_days: [],
            day_count: 0,
          },
        ],
      }),
    );
    await renderPhase();

    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    expect(mocks.decideAdminChild).not.toHaveBeenCalled();
    expect(
      screen.getByText(
        "Für dieses Kind ist kein Betreuungsangebot gebucht. Das Kind wird trotzdem in die OGS aufgenommen.",
      ),
    ).toBeVisible();

    fireEvent.click(
      screen.getByRole("button", { name: "Trotzdem bestätigen" }),
    );
    await waitFor(() => {
      expect(mocks.decideAdminChild).toHaveBeenCalledWith(
        "10",
        "20",
        "approved",
      );
    });
  });

  it("approves directly when care offerings are disabled", async () => {
    mocks.useCareOfferingsEnabled.mockReturnValue(false);
    mocks.getCareUsageReport.mockResolvedValue(
      report({
        rows: [
          {
            ...report().rows[0],
            offerings: [],
            effective_days: [],
            day_count: 0,
          },
        ],
      }),
    );
    await renderPhase();

    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() => {
      expect(mocks.decideAdminChild).toHaveBeenCalledWith(
        "10",
        "20",
        "approved",
      );
    });
    expect(
      screen.queryByRole("button", { name: "Trotzdem bestätigen" }),
    ).not.toBeInTheDocument();
  });
});
