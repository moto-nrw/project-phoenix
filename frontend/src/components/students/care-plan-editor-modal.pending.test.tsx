// #6593: Während eine Ausnahme, die gebuchte Abholzeit oder ein Hinweis
// gespeichert wird, bleibt das Panel zu; eine späte Antwort schließt keinen
// anderen Tag. Echter SlideOver mit vaul-Mock, echte Bestätigungsdialoge.
import { act, fireEvent, render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { ArrivalDayData } from "~/lib/arrival-schedule-helpers";
import type { DayData as PickupDayData } from "~/lib/pickup-schedule-helpers";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { toastSuccess } = vi.hoisted(() => ({ toastSuccess: vi.fn() }));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: toastSuccess, error: vi.fn() }),
}));

import { CarePlanEditorModal } from "./care-plan-editor-modal";

const MONDAY_TITLE = "Ausnahme für Montag, 25.05.";
const TUESDAY_TITLE = "Ausnahme für Dienstag, 26.05.";
const CLOSE = "Fenster schließen";

function arrivalDay(date: Date, weekday: number): ArrivalDayData {
  return {
    date,
    weekday,
    isToday: false,
    showSick: false,
    showClassTrip: false,
    showExcused: false,
    exception: undefined,
    baseSchedule: {
      id: "1",
      studentId: "42",
      weekday,
      weekdayName: "Montag",
      expectedArrival: "08:00",
      notes: "",
      createdBy: "1",
      createdAt: "2026-05-01T00:00:00Z",
      updatedAt: "2026-05-01T00:00:00Z",
    },
    effectiveTime: "08:00",
    effectiveReason: "",
    isException: false,
    isAbsent: false,
    notes: [],
  };
}

function pickupDay(date: Date, weekday: number): PickupDayData {
  return {
    date,
    weekday,
    isToday: false,
    showSick: false,
    showClassTrip: false,
    showExcused: false,
    exception: undefined,
    baseSchedule: {
      id: "1",
      studentId: "42",
      weekday,
      weekdayName: "Montag",
      pickupTime: "15:00",
      notes: "",
      createdBy: "1",
      createdAt: "2026-05-01T00:00:00Z",
      updatedAt: "2026-05-01T00:00:00Z",
    },
    offeringSchedule: {
      id: "0",
      studentId: "42",
      weekday,
      weekdayName: "Montag",
      pickupTime: "15:30",
      source: "care_offering",
      careOfferingName: "Ganztagsbetreuung",
      createdBy: "0",
      createdAt: "0001-01-01T00:00:00Z",
      updatedAt: "0001-01-01T00:00:00Z",
    },
    effectiveTime: "15:00",
    effectiveNotes: "",
    isException: false,
    notes: [],
  };
}

const MONDAY = new Date("2026-05-25T00:00:00");
const TUESDAY = new Date("2026-05-26T00:00:00");

type Props = ComponentProps<typeof CarePlanEditorModal>;

function renderEditor(props: Partial<Props> = {}) {
  const onClose = vi.fn();
  const onSubmitException = vi.fn().mockResolvedValue(undefined);
  const editor = (next: Partial<Props> = {}) => (
    <CarePlanEditorModal
      isOpen
      onClose={onClose}
      arrivalDay={arrivalDay(MONDAY, 1)}
      pickupDay={pickupDay(MONDAY, 1)}
      onSubmitException={onSubmitException}
      {...props}
      {...next}
    />
  );
  const view = render(editor());
  return { ...view, editor, onClose, onSubmitException };
}

function markArrivalAbsentAndSave() {
  fireEvent.click(screen.getByRole("button", { name: "Kommt nicht" }));
  const save = screen.getByRole("button", { name: "Speichern" });
  expect(save).toBeEnabled();
  fireEvent.click(save);
}

describe("CarePlanEditorModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the panel shut while saving and unlocks it after a failure", async () => {
    const answer = deferred();
    const { onClose } = renderEditor({
      onSubmitException: vi.fn(() => answer.promise),
    });
    markArrivalAbsentAndSave();

    expectSlideOverSaveLocked(
      MONDAY_TITLE,
      ["Abbrechen", "Wird gespeichert …"],
      CLOSE,
    );
    expect(onClose).not.toHaveBeenCalled();

    await act(async () => {
      answer.reject(new ApiError("down", 503, { code: "general.unavailable" }));
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Die Ausnahme ist gerade nicht erreichbar.",
    );
    expectSlideOverUnlocked(MONDAY_TITLE, CLOSE);
    // Der Entwurf steht noch: „Kommt nicht“ ist gewählt, also gibt es einen Grund.
    expect(
      screen.getByLabelText("Grund", { selector: "#exception-ankunft-reason" }),
    ).toBeVisible();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes the panel after a successful save", async () => {
    const answer = deferred();
    const { onClose } = renderEditor({
      onSubmitException: vi.fn(() => answer.promise),
    });
    markArrivalAbsentAndSave();

    await act(async () => {
      answer.resolve();
    });

    expect(toastSuccess).toHaveBeenCalledWith("Die Ausnahme ist gespeichert.");
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("reports but does not close another day when the old save answers late", async () => {
    const answer = deferred();
    const { onClose, rerender, editor } = renderEditor({
      onSubmitException: vi.fn(() => answer.promise),
    });
    markArrivalAbsentAndSave();

    rerender(
      editor({
        arrivalDay: arrivalDay(TUESDAY, 2),
        pickupDay: pickupDay(TUESDAY, 2),
      }),
    );
    expectSlideOverUnlocked(TUESDAY_TITLE, CLOSE);

    await act(async () => {
      answer.resolve();
    });

    // Gespeichert ist gespeichert; geschlossen wird der Dienstag nicht.
    expect(toastSuccess).toHaveBeenCalledWith("Die Ausnahme ist gespeichert.");
    expect(onClose).not.toHaveBeenCalled();
    expectSlideOverUnlocked(TUESDAY_TITLE, CLOSE);
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });

  it("locks the panel while the booked pickup time is taken over", async () => {
    const answer = deferred();
    renderEditor({ onResetPickupToOffering: vi.fn(() => answer.promise) });

    fireEvent.click(
      screen.getByRole("button", { name: "Gebuchte Abholzeit übernehmen" }),
    );

    expectSlideOverSaveLocked(MONDAY_TITLE, ["Abbrechen", "Speichern"], CLOSE);
    expect(
      screen.getByRole("button", { name: "Wird übernommen …" }),
    ).toBeDisabled();

    await act(async () => {
      answer.resolve();
    });
    expectSlideOverUnlocked(MONDAY_TITLE, CLOSE);
  });

  // Eine späte Antwort leert nicht den Entwurf des neuen Vorgangs.
  it("locks the panel while a day note is added and keeps a reopened draft", async () => {
    const answer = deferred();
    const { rerender, editor } = renderEditor({
      onCreateArrivalNote: vi.fn(() => answer.promise),
    });

    fireEvent.change(screen.getByLabelText("Ankunft Hinweis hinzufügen"), {
      target: { value: "Bitte klingeln" },
    });
    fireEvent.click(screen.getAllByRole("button", { name: "Hinzufügen" })[0]!);

    expectSlideOverSaveLocked(MONDAY_TITLE, ["Abbrechen", "Speichern"], CLOSE);

    rerender(editor({ isOpen: false }));
    rerender(editor());
    expectSlideOverUnlocked(MONDAY_TITLE, CLOSE);
    fireEvent.change(screen.getByLabelText("Ankunft Hinweis hinzufügen"), {
      target: { value: "Neuer Entwurf" },
    });

    await act(async () => {
      answer.resolve();
    });

    expect(screen.getByLabelText("Ankunft Hinweis hinzufügen")).toHaveValue(
      "Neuer Entwurf",
    );
    expectSlideOverUnlocked(MONDAY_TITLE, CLOSE);
  });
});
