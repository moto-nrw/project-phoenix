import "@testing-library/jest-dom/vitest";
import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import StatisticsPage from "./page";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { StatisticsReport } from "~/lib/statistics-api";
import { catalogText } from "~/test/error-catalog-text";

// Exportfehler kommen als Toast (#2517).
function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: vi.fn() })),
  usePathname: vi.fn(() => "/statistics"),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));

const mockFetchReport = vi.fn();
vi.mock("~/lib/statistics-api", async () => {
  const actual = await vi.importActual<typeof import("~/lib/statistics-api")>(
    "~/lib/statistics-api",
  );
  return {
    ...actual,
    fetchStatisticsReport: (...args: unknown[]) =>
      mockFetchReport(...args) as Promise<StatisticsReport>,
  };
});

const emptyGroupRow = {
  group_id: "0",
  name: "Gesamt",
  student_count: 0,
  present_days: 0,
  sick_days: 0,
  excused_days: 0,
  unexplained_days: 0,
  attendance_rate: null,
};

function report(overrides: Partial<StatisticsReport> = {}): StatisticsReport {
  return {
    from: "2026-06-01",
    to: "2026-06-30",
    care_days: 22,
    excluded_days: {
      total: 0,
      public_holidays: 0,
      closing_days: 0,
      holiday_periods: 0,
    },
    totals: emptyGroupRow,
    students: [],
    groups: [],
    rooms: [],
    room_data_days: 30,
    room_data_from: "2026-06-01",
    courses: [
      {
        course_id: "1",
        name: "Fußball",
        category_name: "Sport",
        max_participants: 20,
        held_instances: 4,
        cancelled_instances: 1,
        student_count: 5,
        present_days: 18,
        absent_days: 2,
        open_days: 3,
        participation_rate: 90,
        occupancy_percent: 25,
      },
    ],
    course_students: [
      {
        student_id: "7",
        first_name: "Mia",
        last_name: "Bauer",
        school_class: "1b",
        group_name: "Bärengruppe",
        course_id: "1",
        course_name: "Fußball",
        present_days: 3,
        absent_days: 1,
        open_days: 0,
        participation_rate: 75,
      },
    ],
    course_totals: {
      course_id: "0",
      name: "Gesamt",
      category_name: "",
      max_participants: 0,
      held_instances: 4,
      cancelled_instances: 1,
      student_count: 5,
      present_days: 18,
      absent_days: 2,
      open_days: 3,
      participation_rate: 90,
      occupancy_percent: null,
    },
    course_data_days: 365,
    course_data_from: "2025-09-01",
    ...overrides,
  };
}

/** Ein Datum weit nach heute, damit der Zeitraum sicher davor liegt. */
function futureISODate(): string {
  return `${new Date().getFullYear() + 1}-12-31`;
}

async function openCourses() {
  render(<StatisticsPage />);
  await waitFor(() =>
    expect(screen.getByRole("tab", { name: "Kurse" })).toBeInTheDocument(),
  );
  fireEvent.click(screen.getByRole("tab", { name: "Kurse" }));
}

describe("Statistik — Bereich Kurse (#2891)", () => {
  beforeEach(() => {
    mockFetchReport.mockReset();
    mockFetchReport.mockResolvedValue(report());
  });

  it("zeigt bei fehlendem Zugriff weiterhin die bisherige Meldung", async () => {
    mockFetchReport.mockRejectedValueOnce(new ApiError("forbidden", 403));
    render(<StatisticsPage />);

    await waitFor(() =>
      expect(
        screen.getByText(
          "Ihr Konto darf die Statistik nicht sehen. Bitte wenden Sie sich an Ihre Administration.",
        ),
      ).toBeVisible(),
    );
  });

  it("zeigt einen Ladefehler mit Katalogtext und lädt auf Wiederholen neu", async () => {
    mockFetchReport.mockRejectedValueOnce(
      new ApiError("down", 503, { code: "general.unavailable" }),
    );
    render(<StatisticsPage />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Statistik"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mockFetchReport).toHaveBeenCalledTimes(2));
  });

  it("erklärt einen ungültigen Zeitraum mit dem eigenen Text", async () => {
    mockFetchReport.mockRejectedValueOnce(
      new ApiError("bad range", 400, {
        code: "presence.statistics_range_invalid",
      }),
    );
    render(<StatisticsPage />);

    expect(
      await screen.findByText(
        catalogText("presence.statistics_range_invalid", "die Statistik"),
      ),
    ).toBeInTheDocument();
  });

  it("zeigt je Kurs Termine, abgesagte Termine und die Quote", async () => {
    await openCourses();

    await waitFor(() => expect(screen.getByText("Fußball")).toBeVisible());
    expect(screen.getByText("Sport")).toBeVisible();
    // Die Quote steht in der Kachel und in der Zeile.
    expect(screen.getAllByText("90,0 %").length).toBeGreaterThan(0);
    expect(screen.getByText("25,0 %")).toBeVisible();
  });

  it("wechselt zwischen der Kurs- und der Kind-Sicht", async () => {
    await openCourses();
    await waitFor(() => expect(screen.getByText("Fußball")).toBeVisible());

    fireEvent.click(screen.getByRole("button", { name: "Je Kind" }));

    await waitFor(() =>
      expect(screen.getByText("Bauer, Mia")).toBeInTheDocument(),
    );
    expect(screen.getByText("75,0 %")).toBeVisible();
  });

  // Ohne entschiedenen Termin gibt es keine Quote. Die Kachel muss das
  // sagen; ein leeres Feld sieht aus wie ein Fehler.
  it("zeigt einen Gedankenstrich statt einer leeren Quote", async () => {
    mockFetchReport.mockResolvedValue(
      report({
        courses: [],
        course_students: [],
        course_totals: {
          ...report().course_totals,
          participation_rate: null,
          held_instances: 0,
          cancelled_instances: 0,
          student_count: 0,
          open_days: 0,
        },
      }),
    );
    await openCourses();

    await waitFor(() => expect(screen.getByText("Quote gesamt")).toBeVisible());
    expect(screen.getByText("–")).toBeVisible();
  });

  it("nennt im Leerzustand den Grund", async () => {
    mockFetchReport.mockResolvedValue(
      report({ courses: [], course_students: [] }),
    );
    await openCourses();

    await waitFor(() =>
      expect(screen.getByText("Keine Kurse im Zeitraum")).toBeVisible(),
    );
  });

  // Liegt der ganze Zeitraum vor der Aufbewahrungsfrist, kann es dort keine
  // Termine mehr geben. Das muss die Seite sagen statt stumm Nullen zu zeigen.
  it("erklärt einen Zeitraum vor der Aufbewahrungsfrist", async () => {
    mockFetchReport.mockResolvedValue(
      report({
        courses: [],
        course_students: [],
        // Der Standardzeitraum der Seite endet heute, liegt also
        // vollständig vor diesem Datum.
        course_data_from: futureISODate(),
      }),
    );
    await openCourses();

    await waitFor(() =>
      expect(
        screen.getByText("Keine Kurstermine für diesen Zeitraum"),
      ).toBeVisible(),
    );
  });
});
