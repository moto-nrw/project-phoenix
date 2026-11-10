// #6593: Solange eine Schicht gespeichert oder gelöscht wird, lässt sich das
// Panel nicht schließen, auch wenn der Schreibvorgang aus der Rückfrage oder
// der Löschbestätigung startet; eine späte Antwort schließt nicht den Dialog
// einer anderen Schicht.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const api = vi.hoisted(() => ({
  updateShift: vi.fn(),
  deleteShift: vi.fn(),
  getSeries: vi.fn(),
  listPeriods: vi.fn(),
}));

vi.mock("~/lib/shift-api", () => ({
  staffShiftService: {
    createShift: vi.fn(),
    updateShift: api.updateShift,
    deleteShift: api.deleteShift,
    applyCancellation: vi.fn(),
  },
  staffShiftSeriesService: {
    createSeries: vi.fn(),
    splitSeries: vi.fn(),
    endSeries: vi.fn(),
    getSeries: api.getSeries,
  },
}));
vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: { list: api.listPeriods },
}));

import { ApiError } from "~/lib/api-error";
import type { StaffShift } from "~/lib/shift-helpers";
import { catalogText } from "~/test/error-catalog-text";
import { ShiftEditModal } from "./shift-edit-modal";

const TITLE = /^Schicht bearbeiten/;
const FOOTER = ["Schicht löschen", "Abbrechen", "Speichern…"];

const shift: StaffShift = {
  id: "1",
  staffId: "7",
  date: "2026-09-07",
  startTime: "08:00",
  endTime: "16:00",
  breakMinutes: 30,
  shiftTypeId: null,
  shiftTypeName: null,
  shiftTypeColor: null,
  notes: "",
  seriesId: null,
  detached: false,
  cancelled: false,
  changeReason: null,
  originShiftId: null,
};
const otherShift: StaffShift = { ...shift, id: "2", startTime: "09:00" };
const seriesShift: StaffShift = { ...shift, id: "9", seriesId: "5" };

function Modal({
  current = shift,
  onClose,
  onSaved,
}: {
  current?: StaffShift;
  onClose: () => void;
  onSaved: () => void;
}) {
  return (
    <ShiftEditModal
      isOpen
      mode="edit"
      staffId="7"
      staffName="Ada Lovelace"
      date={current.date}
      shift={current}
      shiftTypes={[]}
      onClose={onClose}
      onSaved={onSaved}
    />
  );
}

function startSave(current = shift) {
  const answer = deferred();
  api.updateShift.mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const onSaved = vi.fn();
  const view = render(
    <Modal current={current} onClose={onClose} onSaved={onSaved} />,
  );
  fireEvent.change(screen.getByLabelText("Ende"), {
    target: { value: "15:00" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Änderungen speichern" }));
  return { ...view, answer, onClose, onSaved };
}

async function startDelete() {
  const answer = deferred();
  api.deleteShift.mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const onSaved = vi.fn();
  const view = render(<Modal onClose={onClose} onSaved={onSaved} />);
  fireEvent.click(screen.getByRole("button", { name: "Schicht löschen" }));
  fireEvent.click(screen.getByRole("button", { name: "Ja, löschen" }));
  fireEvent.click(
    await screen.findByRole("button", { name: "Endgültig löschen" }),
  );
  await waitFor(() => expect(api.deleteShift).toHaveBeenCalledWith("1"));
  return { ...view, answer, onClose, onSaved };
}

describe("ShiftEditModal: Speichern sperrt das Panel (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.listPeriods.mockResolvedValue([]);
    api.getSeries.mockResolvedValue({
      id: "5",
      staffId: "7",
      weekdays: [1],
      startTime: "08:00",
      endTime: "16:00",
      breakMinutes: 30,
      shiftTypeId: null,
      calendarPeriodId: "5",
      weekPattern: 0,
      validFrom: "2026-09-01",
      validUntil: null,
      includeSchoolBreaks: false,
    });
  });

  it("locks the panel while saving and unlocks it with the error after a failure", async () => {
    const { answer, onClose } = startSave();
    await screen.findByRole("button", { name: "Speichern…" });

    expectSlideOverSaveLocked(TITLE, FOOTER);
    expect(onClose).not.toHaveBeenCalled();

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(
      await screen.findByText(catalogText("general.server", "die Schicht")),
    ).toBeVisible();
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByLabelText("Ende")).toHaveValue("15:00");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("refreshes the plan and closes after the shift is saved", async () => {
    const { answer, onClose, onSaved } = startSave();
    await screen.findByRole("button", { name: "Speichern…" });

    await act(async () => {
      answer.resolve();
    });

    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("does not close the dialog of another shift when a late answer arrives", async () => {
    const { answer, onClose, onSaved, rerender } = startSave();
    await screen.findByRole("button", { name: "Speichern…" });

    rerender(
      <Modal current={otherShift} onClose={onClose} onSaved={onSaved} />,
    );
    // Anker: the dialog now shows the other shift, unlocked.
    await waitFor(() =>
      expect(screen.getByLabelText("Beginn")).toHaveValue("09:00"),
    );
    expectSlideOverUnlocked(TITLE);

    await act(async () => {
      answer.resolve();
    });

    // The write happened, so the plan refreshes; the other dialog stays.
    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
    expectSlideOverUnlocked(TITLE);
  });

  it("runs a save chosen in the scope question and shows its error in the panel", async () => {
    const { answer, onClose } = startSave(seriesShift);
    fireEvent.click(
      await screen.findByRole("button", { name: /Nur diese Woche/ }),
    );
    await waitFor(() =>
      expect(api.updateShift).toHaveBeenCalledWith(
        "9",
        expect.objectContaining({ endTime: "15:00" }),
      ),
    );

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    // The panel comes back with the error and can be closed again.
    expect(
      await screen.findByText(catalogText("general.server", "die Schicht")),
    ).toBeVisible();
    expectSlideOverUnlocked(TITLE);
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("ShiftEditModal: Löschen läuft unter der Sperre des Panels (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.listPeriods.mockResolvedValue([]);
  });

  it("keeps a failed delete in the confirmation and the dialog open", async () => {
    const { answer, onClose } = await startDelete();
    const dialog = screen.getByRole("dialog", { name: "Schicht löschen" });

    await act(async () => {
      answer.reject(
        new ApiError("verboten", 403, { code: "general.permission" }),
      );
    });

    expect(dialog).toHaveTextContent(
      catalogText("general.permission", "die Schicht"),
    );
    expect(onClose).not.toHaveBeenCalled();
  });

  it("refreshes the plan and closes after the shift is deleted", async () => {
    const { answer, onClose, onSaved } = await startDelete();

    await act(async () => {
      answer.resolve();
    });

    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("does not close the dialog of another shift when a late delete answer arrives", async () => {
    const { answer, onClose, onSaved, rerender } = await startDelete();

    rerender(
      <Modal current={otherShift} onClose={onClose} onSaved={onSaved} />,
    );
    await act(async () => {
      answer.resolve();
    });

    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
  });
});
