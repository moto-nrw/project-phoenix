import "@testing-library/jest-dom/vitest";
import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { StudentCareTimesSection } from "./care-times-section";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { CareTimeDay, StudentCareTimes } from "~/lib/care-times";
import { catalogText } from "~/test/error-catalog-text";

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

const mockFetch = vi.fn();
vi.mock("~/lib/care-times", async () => {
  const actual =
    await vi.importActual<typeof import("~/lib/care-times")>(
      "~/lib/care-times",
    );
  return {
    ...actual,
    fetchStudentCareTimes: (...args: unknown[]) =>
      mockFetch(...args) as Promise<StudentCareTimes>,
  };
});

function day(overrides: Partial<CareTimeDay>): CareTimeDay {
  return {
    date: "2026-09-07",
    kind: "within_plan",
    overrun: false,
    planned_start: "08:00",
    planned_end: "15:00",
    actual_start: "08:00",
    actual_end: "15:00",
    planned_minutes: 420,
    actual_minutes: 420,
    deviation_minutes: 0,
    early_minutes: 0,
    late_minutes: 0,
    ...overrides,
  };
}

function result(days: CareTimeDay[]): StudentCareTimes {
  return {
    student_id: "7",
    from: "2026-09-01",
    to: "2026-09-09",
    data_from: "2026-09-01",
    tolerance_minutes: 15,
    summary: {
      overrun_count: 2,
      overrun_days: 1,
      unbooked_days: 1,
      absent_days: 0,
      extra_minutes: 150,
      planned_minutes: 840,
      actual_minutes: 990,
      deviation_minutes: 150,
    },
    days,
  };
}

describe("Soll und Ist je Kind (#4774)", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("lädt den laufenden Monat bis heute, sobald der Reiter offen ist", async () => {
    mockFetch.mockResolvedValue(result([]));
    const { rerender } = render(
      <StudentCareTimesSection studentId="7" active={false} />,
    );
    expect(screen.getByText("Soll und Ist")).toBeInTheDocument();
    expect(mockFetch).not.toHaveBeenCalled();

    rerender(<StudentCareTimesSection studentId="7" active />);

    await waitFor(() =>
      expect(mockFetch).toHaveBeenCalledWith("7", "2026-09-01", "2026-09-09"),
    );
  });

  it("lädt neu, wenn sich die Buchung ändert, und behält den Zeitraum (#5994)", async () => {
    mockFetch.mockResolvedValue(result([]));
    const range = { from: "2026-09-07", to: "2026-09-09" };
    const { rerender } = render(
      <StudentCareTimesSection
        studentId="7"
        active
        initialRange={range}
        refreshKey="08:00-13:00"
      />,
    );
    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(1));

    rerender(
      <StudentCareTimesSection
        studentId="7"
        active
        initialRange={range}
        refreshKey="08:00-16:00"
      />,
    );

    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(2));
    expect(mockFetch).toHaveBeenLastCalledWith("7", "2026-09-07", "2026-09-09");
  });

  it("nennt die Quelle der gebuchten Zeit: Ferienbuchung, Vertrag oder keine (#5994)", async () => {
    mockFetch.mockResolvedValue(
      result([
        day({
          date: "2026-09-07",
          planned_source: "holiday_booking",
          planned_end: "16:00",
        }),
        day({ date: "2026-09-08", planned_source: "contract" }),
        day({ date: "2026-09-09", planned_source: "care_plan" }),
      ]),
    );
    render(<StudentCareTimesSection studentId="7" active />);

    const table = within(await screen.findByRole("table"));
    const rows = await table.findAllByRole("row");
    const rowOf = (date: string) =>
      rows.find((row) => row.textContent?.includes(date));
    expect(
      within(rowOf("07.09.")!).getByText("laut Ferienbuchung"),
    ).toBeInTheDocument();
    expect(
      within(rowOf("08.09.")!).getByText("laut Vertrag"),
    ).toBeInTheDocument();
    const planRow = within(rowOf("09.09.")!);
    // Gebucht und Tatsächlich zeigen dieselbe Spanne.
    expect(planRow.getAllByText("08:00–15:00")).toHaveLength(2);
    expect(planRow.queryByText(/^laut /)).toBeNull();
  });

  it("markiert Überschreitungen mit Farbe und Text und nennt den Grund", async () => {
    mockFetch.mockResolvedValue(
      result([
        day({}),
        day({
          date: "2026-09-08",
          kind: "overrun",
          overrun: true,
          actual_start: "07:30",
          actual_end: "15:20",
          actual_minutes: 470,
          deviation_minutes: 50,
          early_minutes: 30,
          late_minutes: 20,
        }),
        day({
          date: "2026-09-05",
          kind: "unbooked",
          overrun: true,
          planned_start: null,
          planned_end: null,
          planned_minutes: 0,
          actual_start: "10:00",
          actual_end: "12:00",
          actual_minutes: 120,
          deviation_minutes: 120,
        }),
        day({
          date: "2026-09-09",
          kind: "sick",
          actual_start: null,
          actual_end: null,
          actual_minutes: null,
          deviation_minutes: null,
        }),
      ]),
    );
    render(<StudentCareTimesSection studentId="7" active />);

    // Der Telefonstapel wiederholt die Zeilen; geprüft wird die Tabelle.
    const table = within(await screen.findByRole("table"));
    expect(await table.findByText("Außerhalb der Buchung")).toBeInTheDocument();
    expect(
      table.getByText("30 Min. früher gekommen, 20 Min. später gegangen"),
    ).toBeInTheDocument();
    expect(table.getByText("Da ohne Buchung")).toBeInTheDocument();
    expect(
      table.getByText("An diesem Tag ist keine Betreuung gebucht."),
    ).toBeInTheDocument();
    expect(table.getByText("Wie gebucht")).toBeInTheDocument();
    expect(table.getByText("Krank")).toBeInTheDocument();
    expect(table.getByText("07:30–15:20")).toBeInTheDocument();
    expect(table.getByText("+50 Min.")).toBeInTheDocument();

    const overruns = screen.getByText("Überschreitungen").closest("div");
    expect(overruns).not.toBeNull();
    expect(within(overruns as HTMLElement).getByText("2")).toBeInTheDocument();
  });

  it("nennt die Zeit im Unterricht als Hinweis mit Uhrzeit", async () => {
    mockFetch.mockResolvedValue(
      result([
        day({
          date: "2026-09-07",
          actual_end: "08:30",
          deviation_minutes: -450,
          class_times: [{ start: "08:30", end: null }],
        }),
        day({
          date: "2026-09-08",
          class_times: [{ start: "08:30", end: "13:00" }],
        }),
      ]),
    );
    render(<StudentCareTimesSection studentId="7" active />);

    const table = within(await screen.findByRole("table"));
    // Ohne Rückkehr liest sich -450 als Unterricht, nicht als Heimgang.
    expect(await table.findByText("-450 Min.")).toBeInTheDocument();
    expect(table.getByText("Zum Unterricht: ab 08:30")).toBeInTheDocument();
    expect(table.getByText("Zum Unterricht: 08:30–13:00")).toBeInTheDocument();
  });

  it("zeigt einen Ladefehler mit Katalogtext und lädt auf Wiederholen neu", async () => {
    mockFetch
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue(result([]));
    render(<StudentCareTimesSection studentId="7" active />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "Soll und Ist"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(2));
  });

  it("blendet alte Ergebnisse beim Laden eines anderen Kindes aus", async () => {
    mockFetch
      .mockResolvedValueOnce(
        result([
          day({
            actual_start: "07:30",
            actual_end: "15:20",
            actual_minutes: 470,
            deviation_minutes: 50,
          }),
        ]),
      )
      .mockReturnValueOnce(new Promise<StudentCareTimes>(() => undefined));
    const { rerender } = render(
      <StudentCareTimesSection studentId="7" active />,
    );

    expect((await screen.findAllByText("07:30–15:20")).length).toBeGreaterThan(
      0,
    );
    rerender(<StudentCareTimesSection studentId="8" active />);

    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(2));
    // Anker: Der neue Request zeigt den Ladezustand.
    expect(screen.getByText("Wird geladen…")).toBeInTheDocument();
    expect(screen.queryAllByText("07:30–15:20")).toHaveLength(0);
  });
});
