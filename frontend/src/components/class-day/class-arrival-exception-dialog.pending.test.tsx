// #6533: Speichert das eingebettete Panel, sperrt der Dialog X, Escape,
// Hintergrund und Fußleiste; eine späte Antwort meldet sich nicht in einem
// neu geöffneten Dialog. Echter FormModal.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { toISODate } from "~/lib/date-helpers";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  upsert: vi.fn(),
}));

vi.mock("~/lib/school-class-day-api", () => ({
  fetchClassArrivalExceptionsSchool: mocks.list,
  upsertClassArrivalExceptionSchool: mocks.upsert,
  deleteClassArrivalExceptionSchool: vi.fn(),
  fetchClassBlockStartSchool: vi.fn(),
}));
vi.mock("~/components/ui/date-picker", () => ({
  DatePicker: ({
    id,
    value,
    onChange,
  }: {
    id?: string;
    value: Date | null;
    onChange: (date: Date | null) => void;
  }) => (
    <input
      id={id}
      type="date"
      value={value ? toISODate(value) : ""}
      onChange={(event) =>
        onChange(
          event.target.value
            ? new Date(`${event.target.value}T00:00:00`)
            : null,
        )
      }
    />
  ),
}));

import { ClassArrivalExceptionDialog } from "./class-arrival-exception-dialog";

const TITLE = "Ankunftszeit an einem Tag für Klasse 4a";

async function renderAndSave() {
  const answer = deferred<unknown>();
  mocks.upsert.mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const onChanged = vi.fn();
  const dialog = (isOpen: boolean) => (
    <ClassArrivalExceptionDialog
      isOpen={isOpen}
      onClose={onClose}
      schoolClass="4a"
      defaultDate={new Date("2099-03-02T00:00:00")}
      onChanged={onChanged}
    />
  );
  const view = render(dialog(true));
  fireEvent.change(await screen.findByLabelText("Kommt um"), {
    target: { value: "12:45" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Tag speichern" }));
  return { ...view, dialog, answer, onClose, onChanged };
}

describe("ClassArrivalExceptionDialog pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.list.mockResolvedValue({
      school_class: "4a",
      can_edit: true,
      exceptions: [],
    });
  });

  it("locks the dialog while the embedded panel saves and unlocks it after a failure", async () => {
    const { answer, onClose } = await renderAndSave();

    expectSaveLocksDialog(TITLE, ["Schließen"]);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked(TITLE);
    expect(screen.getByLabelText("Kommt um")).toHaveValue("12:45");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("shows a late success not in the dialog opened again meanwhile", async () => {
    const { answer, dialog, rerender, onChanged } = await renderAndSave();
    expect(mocks.upsert).toHaveBeenCalledOnce();

    rerender(dialog(false));
    rerender(dialog(true));
    // Anker: der neu geöffnete Dialog zeigt sein leeres Formular.
    expect(await screen.findByLabelText("Kommt um")).toHaveValue("");

    await act(async () => {
      answer.resolve({
        school_class: "4a",
        date: "2099-03-02",
        arrival_time: "12:45",
        reason: null,
        created_at: "2099-03-01T10:00:00Z",
        origin: "school",
      });
    });

    // Die Daten frischt die Antwort trotzdem auf.
    expect(onChanged).toHaveBeenCalledOnce();
    expect(screen.queryByText(/kommt am 02.03.2099/)).not.toBeInTheDocument();
  });
});
