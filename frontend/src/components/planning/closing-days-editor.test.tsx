import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ClosingDay } from "~/lib/closing-day-helpers";

const {
  mockList,
  mockDelete,
  mockInvalidate,
  mockToastSuccess,
  mockToastError,
  mockUseSession,
  mockBulkCancel,
  mockRefreshPlan,
} = vi.hoisted(() => ({
  mockList: vi.fn(),
  mockDelete: vi.fn(),
  mockInvalidate: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockUseSession: vi.fn(),
  mockBulkCancel: vi.fn(),
  mockRefreshPlan: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { bulkCancel: mockBulkCancel },
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutateMatching: () => mockRefreshPlan,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: mockToastError }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

vi.mock("~/lib/closing-day-api", () => ({
  closingDayService: {
    list: mockList,
    delete: mockDelete,
  },
}));

vi.mock("~/lib/hooks/use-closing-days", () => ({
  useInvalidateClosingDays: () => mockInvalidate,
}));

vi.mock("~/components/planning/closing-day-modal", () => ({
  ClosingDayModal: ({
    isOpen,
    initial,
    onSaved,
    onOfferCancel,
  }: {
    isOpen: boolean;
    initial?: ClosingDay | null;
    onSaved: () => void;
    onOfferCancel?: (range: { startDate: string; endDate: string }) => void;
  }) =>
    isOpen ? (
      <div
        data-testid="closing-day-modal"
        data-initial-reason={initial?.reason ?? ""}
      >
        <button type="button" onClick={onSaved}>
          Mock speichern
        </button>
        {onOfferCancel && (
          <button
            type="button"
            onClick={() =>
              onOfferCancel({ startDate: "2026-10-12", endDate: "2026-10-25" })
            }
          >
            Mock Termine übrig
          </button>
        )}
      </div>
    ) : null,
}));

import { ClosingDaysEditor } from "./closing-days-editor";

function makeClosingDay(overrides: Partial<ClosingDay> = {}): ClosingDay {
  return {
    id: "3",
    startDate: "2026-12-24",
    endDate: "2026-12-31",
    reason: "Weihnachtswoche",
    ...overrides,
  };
}

describe("ClosingDaysEditor", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockInvalidate.mockResolvedValue(undefined);
    mockRefreshPlan.mockResolvedValue(undefined);
    mockUseSession.mockReturnValue({
      data: { user: { permissions: ["schedules:manage"] } },
      status: "authenticated",
    });
  });

  it("sagt die Termine eines Schließtags nach Bestätigung ab (#3594)", async () => {
    mockList.mockResolvedValue([
      makeClosingDay({
        startDate: "2026-10-12",
        endDate: "2026-10-25",
        reason: "Herbstferien",
      }),
    ]);
    mockBulkCancel.mockImplementation((from: string, to: string, dryRun) =>
      Promise.resolve({
        from,
        to,
        dryRun,
        count: 83,
        days: [{ date: "2026-10-12", count: 83 }],
        kept: 0,
      }),
    );

    const onAppointmentsCancelled = vi.fn();
    render(
      <ClosingDaysEditor onAppointmentsCancelled={onAppointmentsCancelled} />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: /^Aktionen für/ }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Termine absagen" }));

    expect(
      await screen.findByText(
        "Im Zeitraum 12.10.2026 – 25.10.2026 werden 83 Termine abgesagt. Eltern bekommen keine Nachricht.",
      ),
    ).toBeInTheDocument();
    // Der Zeitraum ist mit dem Schließtag vorbelegt, lässt sich aber ändern.
    expect(
      within(screen.getByRole("group", { name: "Zeitraum" })).getByRole(
        "button",
        { name: "12.–25. Okt. 2026" },
      ),
    ).toBeInTheDocument();
    expect(mockBulkCancel).toHaveBeenCalledWith(
      "2026-10-12",
      "2026-10-25",
      true,
      false,
    );

    fireEvent.click(screen.getByRole("button", { name: "Termine absagen" }));
    fireEvent.click(screen.getByRole("button", { name: "Endgültig absagen" }));

    await waitFor(() =>
      expect(mockBulkCancel).toHaveBeenCalledWith(
        "2026-10-12",
        "2026-10-25",
        false,
        false,
      ),
    );
    await waitFor(() =>
      expect(mockToastSuccess).toHaveBeenCalledWith("83 Termine abgesagt"),
    );
    expect(mockRefreshPlan).toHaveBeenCalled();
    // The periods page reloads its "Verwendung" column after a cancellation.
    expect(onAppointmentsCancelled).toHaveBeenCalledOnce();
    // Nothing stays behind, so the holiday-care note is not shown.
    expect(screen.queryByText(/Sie bleiben im Plan/)).not.toBeInTheDocument();
  });

  it("sperrt das Absagen, wenn im Zeitraum nichts mehr geplant ist", async () => {
    mockList.mockResolvedValue([makeClosingDay()]);
    mockBulkCancel.mockResolvedValue({
      from: "2026-12-24",
      to: "2026-12-31",
      dryRun: true,
      count: 0,
      days: [],
    });

    render(<ClosingDaysEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: /^Aktionen für/ }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Termine absagen" }));

    expect(
      await screen.findByText(
        "Im Zeitraum 24.12.2026 – 31.12.2026 sind keine Termine mehr geplant.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Termine absagen" }),
    ).toBeDisabled();
  });

  it("bietet das Absagen nach dem Speichern an, wenn Termine übrig sind", async () => {
    mockList.mockResolvedValue([]);
    mockBulkCancel.mockResolvedValue({
      from: "2026-10-12",
      to: "2026-10-25",
      dryRun: true,
      count: 5,
      days: [],
      kept: 3,
      keptSeries: [{ name: "Ferienbetreuung", count: 3 }],
    });

    render(<ClosingDaysEditor />);
    const createButtons = await screen.findAllByRole("button", {
      name: /Schließtag anlegen/,
    });
    fireEvent.click(createButtons[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Mock Termine übrig" }));

    expect(
      await screen.findByText(/werden 5 Termine abgesagt/),
    ).toBeInTheDocument();
    // The offer says the save worked, and title and actions do not repeat
    // each other.
    expect(
      screen.getByText(/Der Schließtag ist gespeichert\./),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("dialog", { name: "Termine im Zeitraum absagen" }),
    ).toBeInTheDocument();
    // Holiday care in the range is named only when there is some.
    expect(
      screen.getByText(
        "Serien, die auch an Schließtagen stattfinden, bleiben: Ferienbetreuung (3 Termine).",
      ),
    ).toBeInTheDocument();
    // Das Angebot nach dem Speichern lässt den Zeitraum ebenfalls ändern.
    expect(screen.getByRole("group", { name: "Zeitraum" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Termine behalten" }));
    await waitFor(() =>
      expect(
        screen.queryByText(/Der Schließtag ist gespeichert\./),
      ).not.toBeInTheDocument(),
    );
    expect(mockBulkCancel).not.toHaveBeenCalledWith(
      "2026-10-12",
      "2026-10-25",
      false,
      false,
    );
  });

  it("zeigt ohne Planungsrecht kein Absagen", async () => {
    mockUseSession.mockReturnValue({
      data: { user: { permissions: ["schedules:read"] } },
      status: "authenticated",
    });
    mockList.mockResolvedValue([makeClosingDay()]);

    render(<ClosingDaysEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: /^Aktionen für/ }),
    );
    expect(
      screen.queryByRole("menuitem", { name: "Termine absagen" }),
    ).not.toBeInTheDocument();
    expect(mockBulkCancel).not.toHaveBeenCalled();
  });

  it("zeigt die Schließtage mit Zeitraum und Grund", async () => {
    mockList.mockResolvedValue([makeClosingDay()]);

    render(<ClosingDaysEditor />);

    expect(await screen.findByText("Weihnachtswoche")).toBeInTheDocument();
    // Der Zeitraum steht doppelt im Markup: eigene Spalte ab sm, darunter die
    // Unterzeile für schmale Screens (#2033). Im Browser ist immer genau eine
    // Variante sichtbar, jsdom wertet die CSS-Sichtbarkeit aber nicht aus.
    const mobileRange = (
      await screen.findAllByText("24.12.2026 – 31.12.2026")
    ).find((element) => element.tagName === "P");

    expect(mobileRange).toHaveClass("break-words");
    expect(mobileRange).not.toHaveClass("truncate");
  });

  it("lässt den Grund umbrechen und hält die Aktionsspalte schmal", async () => {
    mockList.mockResolvedValue([
      makeClosingDay({ reason: "Weihnachtsschließung" }),
    ]);

    render(<ClosingDaysEditor />);

    // Ein langes Wort darf die Grund-Spalte nicht über die Tabellenbreite
    // hinaus aufziehen — sonst scrollt die Tabelle auf 320px seitwärts.
    const reason = await screen.findByText("Weihnachtsschließung");
    expect(reason).toHaveClass("wrap-anywhere");
    expect(reason).not.toHaveClass("truncate");
    expect(reason.parentElement?.className).not.toMatch(/max-w-\[/);

    // Die Aktionsspalte bekommt nur ihre Mindestbreite, den Rest der
    // Tabellenbreite behält der Grund.
    const actionCell = screen
      .getByRole("button", { name: "Aktionen für Weihnachtsschließung" })
      .closest("td");
    expect(actionCell).toHaveClass("w-px");
  });

  it("zeigt für einen Eintages-Schließtag nur ein Datum", async () => {
    mockList.mockResolvedValue([
      makeClosingDay({
        startDate: "2027-02-08",
        endDate: "2027-02-08",
        reason: "Rosenmontag",
      }),
    ]);

    render(<ClosingDaysEditor />);

    expect((await screen.findAllByText("08.02.2027"))[0]).toBeInTheDocument();
    expect(
      screen.queryByText("08.02.2027 – 08.02.2027"),
    ).not.toBeInTheDocument();
  });

  it("öffnet das Modal ohne Vorbelegung beim Anlegen", async () => {
    mockList.mockResolvedValue([]);

    render(<ClosingDaysEditor />);

    const createButtons = await screen.findAllByRole("button", {
      name: /Schließtag anlegen/,
    });
    fireEvent.click(createButtons[0]!);

    expect(screen.getByTestId("closing-day-modal")).toHaveAttribute(
      "data-initial-reason",
      "",
    );
  });

  it("öffnet das Modal mit Vorbelegung beim Bearbeiten", async () => {
    mockList.mockResolvedValue([makeClosingDay()]);

    render(<ClosingDaysEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: /^Aktionen für/ }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Bearbeiten" }));

    expect(screen.getByTestId("closing-day-modal")).toHaveAttribute(
      "data-initial-reason",
      "Weihnachtswoche",
    );
  });

  it("invalidiert den Planungs-Cache nach dem Speichern", async () => {
    mockList.mockResolvedValue([]);

    render(<ClosingDaysEditor />);
    const createButtons = await screen.findAllByRole("button", {
      name: /Schließtag anlegen/,
    });
    fireEvent.click(createButtons[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Mock speichern" }));

    await waitFor(() => expect(mockInvalidate).toHaveBeenCalledOnce());
    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(2));
  });

  it("löscht nach Bestätigung und lädt die Liste neu", async () => {
    mockList
      .mockResolvedValueOnce([makeClosingDay()])
      .mockResolvedValueOnce([]);
    mockDelete.mockResolvedValue(undefined);

    render(<ClosingDaysEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: /^Aktionen für/ }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));
    // Zweistufige Löschbestätigung (ConfirmDeleteModal, #3110).
    fireEvent.click(screen.getByRole("button", { name: "Ja, löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));

    await waitFor(() => expect(mockDelete).toHaveBeenCalledWith("3"));
    await waitFor(() => expect(mockInvalidate).toHaveBeenCalledOnce());
    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(2));
    expect(mockToastSuccess).toHaveBeenCalled();
  });

  it("zeigt den Leerzustand ohne Schließtage", async () => {
    mockList.mockResolvedValue([]);

    render(<ClosingDaysEditor />);

    expect(
      await screen.findByText("Noch keine Schließtage"),
    ).toBeInTheDocument();
  });
});
