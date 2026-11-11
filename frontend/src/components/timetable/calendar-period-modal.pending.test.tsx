// #6593: Solange ein Kalenderzeitraum gespeichert oder gelöscht wird, ist das
// Panel gesperrt; eine späte Antwort schließt kein neu geöffnetes Panel.
// Echter SlideOver und echter Löschdialog.
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type {
  CalendarPeriod,
  CalendarPeriodWarning,
} from "~/lib/calendar-period-helpers";
import { catalogText } from "~/test/error-catalog-text";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

const { mockToastSuccess, mockUpdate, mockDelete } = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockUpdate: vi.fn(),
  mockDelete: vi.fn(),
}));

vi.mock("vaul", () => import("~/test/mocks/vaul"));

vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mockToastSuccess, error: vi.fn() }),
}));

vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: {
    create: vi.fn(),
    update: mockUpdate,
    delete: mockDelete,
  },
}));

import { CalendarPeriodModal } from "./calendar-period-modal";

const TITLE = "Kalenderzeitraum bearbeiten";
const CLOSE = "Zeitraum schließen";

const period: CalendarPeriod = {
  id: "5",
  tenantId: "1",
  name: "Schuljahr 2026/2027",
  periodType: "school_year",
  startDate: "2026-08-01",
  endDate: "2027-07-31",
  weekCycleLength: 1,
  weekCycleAnchor: null,
  isActive: true,
  createdAt: "2026-05-01T00:00:00Z",
  updatedAt: "2026-05-01T00:00:00Z",
} as CalendarPeriod;

function renderPanel() {
  const onClose = vi.fn();
  const onSaved = vi.fn();
  const onDeleted = vi.fn();
  const panel = (isOpen: boolean) => (
    <CalendarPeriodModal
      isOpen={isOpen}
      onClose={onClose}
      onSaved={onSaved}
      onDeleted={onDeleted}
      initial={period}
    />
  );
  const view = render(panel(true));
  return {
    ...view,
    onClose,
    onSaved,
    onDeleted,
    reopen: () => {
      view.rerender(panel(false));
      view.rerender(panel(true));
    },
  };
}

function renderAndSave() {
  const answer = deferred<{
    period: CalendarPeriod;
    warnings: CalendarPeriodWarning[];
  }>();
  mockUpdate.mockReturnValue(answer.promise);
  const view = renderPanel();
  fireEvent.change(screen.getByLabelText("Bezeichnung"), {
    target: { value: "Neues Schuljahr" },
  });
  const saveButton = screen.getByRole("button", { name: "Speichern" });
  expect(saveButton).toBeEnabled();
  fireEvent.click(saveButton);
  expect(mockUpdate).toHaveBeenCalledOnce();
  return { ...view, answer };
}

function renderAndDelete() {
  const answer = deferred();
  mockDelete.mockReturnValue(answer.promise);
  const view = renderPanel();
  fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
  const dialog = screen.getByRole("dialog", {
    name: "Kalenderzeitraum löschen",
  });
  fireEvent.click(within(dialog).getByRole("button", { name: "Löschen" }));
  fireEvent.click(
    within(dialog).getByRole("button", { name: "Endgültig löschen" }),
  );
  expect(mockDelete).toHaveBeenCalledWith("5");
  return { ...view, answer, dialog };
}

describe("CalendarPeriodModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("sperrt das Panel beim Speichern und gibt es nach einem Fehler mit allen Eingaben frei", async () => {
    const { answer, onClose } = renderAndSave();

    expectSlideOverSaveLocked(
      TITLE,
      ["Löschen", "Abbrechen", "Speichere …"],
      CLOSE,
    );

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      catalogText("general.server", "das Speichern des Kalenderzeitraums"),
    );
    expectSlideOverUnlocked(TITLE, CLOSE);
    expect(screen.getByLabelText("Bezeichnung")).toHaveValue("Neues Schuljahr");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("schließt das Panel nach dem Speichern", async () => {
    const { answer, onClose, onSaved } = renderAndSave();

    await act(async () => {
      answer.resolve({ period, warnings: [] });
    });

    expect(onSaved).toHaveBeenCalledWith(period);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("frischt nach einer späten Antwort auf, schließt das neu geöffnete Panel aber nicht", async () => {
    const { answer, onClose, onSaved, reopen } = renderAndSave();

    reopen();
    expectSlideOverUnlocked(TITLE, CLOSE);
    await act(async () => {
      answer.resolve({ period, warnings: [] });
    });

    expect(onSaved).toHaveBeenCalledWith(period);
    expect(mockToastSuccess).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("heading", { name: TITLE })).toBeInTheDocument();
  });

  it("sperrt das Panel beim Löschen und gibt es nach einem Fehler frei", async () => {
    const { answer, dialog, onClose, onDeleted } = renderAndDelete();

    const panel = screen
      .getByRole("heading", { name: TITLE })
      .closest('[role="dialog"]');
    expect(panel).toHaveAttribute("aria-busy", "true");
    expect(screen.getByRole("button", { name: CLOSE })).toBeDisabled();

    await act(async () => {
      answer.reject(
        new ApiError("fk violation", 409, {
          code: "timetable.calendar_period_care_offering_conflict",
        }),
      );
    });

    expect(
      await within(dialog).findByText(
        catalogText(
          "timetable.calendar_period_care_offering_conflict",
          "das Löschen des Kalenderzeitraums",
        ),
      ),
    ).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE, CLOSE);
    expect(onDeleted).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("schließt das Panel nach dem Löschen", async () => {
    const { answer, onClose, onDeleted } = renderAndDelete();

    await act(async () => {
      answer.resolve();
    });

    expect(onDeleted).toHaveBeenCalledWith(period);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("frischt nach einem späten Löschen auf, schließt aber nichts", async () => {
    const { answer, onClose, onDeleted, unmount } = renderAndDelete();

    unmount();
    await act(async () => {
      answer.resolve();
    });

    expect(mockToastSuccess).toHaveBeenCalledOnce();
    expect(onDeleted).toHaveBeenCalledWith(period);
    expect(onClose).not.toHaveBeenCalled();
  });
});
