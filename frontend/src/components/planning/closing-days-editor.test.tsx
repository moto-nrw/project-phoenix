import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ClosingDay } from "~/lib/closing-day-helpers";

const { mockList, mockDelete, mockToastSuccess, mockToastError } = vi.hoisted(
  () => ({
    mockList: vi.fn(),
    mockDelete: vi.fn(),
    mockToastSuccess: vi.fn(),
    mockToastError: vi.fn(),
  }),
);

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

vi.mock("~/components/planning/closing-day-modal", () => ({
  ClosingDayModal: ({
    isOpen,
    initial,
  }: {
    isOpen: boolean;
    initial?: ClosingDay | null;
  }) =>
    isOpen ? (
      <div
        data-testid="closing-day-modal"
        data-initial-reason={initial?.reason ?? ""}
      />
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
      .getByRole("button", { name: "Bearbeiten" })
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

    fireEvent.click(await screen.findByRole("button", { name: "Bearbeiten" }));

    expect(screen.getByTestId("closing-day-modal")).toHaveAttribute(
      "data-initial-reason",
      "Weihnachtswoche",
    );
  });

  it("löscht nach Bestätigung und lädt die Liste neu", async () => {
    mockList
      .mockResolvedValueOnce([makeClosingDay()])
      .mockResolvedValueOnce([]);
    mockDelete.mockResolvedValue(undefined);

    render(<ClosingDaysEditor />);

    fireEvent.click(await screen.findByRole("button", { name: "Löschen" }));
    // Bestätigungsdialog: der Confirm-Button trägt ebenfalls "Löschen".
    const confirmButtons = screen.getAllByRole("button", { name: "Löschen" });
    fireEvent.click(confirmButtons[confirmButtons.length - 1]!);

    await waitFor(() => expect(mockDelete).toHaveBeenCalledWith("3"));
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
