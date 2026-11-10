// #6533: Schließen während des Speicherns ist gesperrt, eine späte Antwort
// schließt keinen anderen Dialog. Echter FormModal.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { Student } from "~/lib/student-helpers";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

const mocks = vi.hoisted(() => ({ bulkUpsert: vi.fn() }));

vi.mock("~/lib/student-arrival-api", async () => ({
  ...(await vi.importActual<typeof import("~/lib/student-arrival-api")>(
    "~/lib/student-arrival-api",
  )),
  fetchArrivalSettings: vi.fn(async () => ({
    careDaysSource: "weekly_plan",
    schoolPeriods: [],
  })),
  fetchBulkArrivalScheduleStatus: vi.fn(async () => 0),
  fetchClassArrivalTimes: vi.fn(async () => ({
    school_class: "3a",
    times: {},
  })),
  bulkUpsertArrivalSchedules: mocks.bulkUpsert,
}));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }),
}));

import { FilteredBulkArrivalModal } from "./class-bulk-arrival-modal";

const TITLE = "Unterrichtsschluss für Klasse 3a";
const student = {
  id: "1",
  name: "S 1",
  first_name: "S",
  second_name: "1",
  school_class: "3a",
  current_location: "class",
} as Student;

async function renderAndSave() {
  const save = deferred<unknown>();
  mocks.bulkUpsert.mockReturnValue(save.promise);
  const onClose = vi.fn();
  const onSuccess = vi.fn();
  const view = render(
    <FilteredBulkArrivalModal
      isOpen
      onClose={onClose}
      filter={{ type: "school_class", schoolClass: "3a" }}
      filterLabel="3a"
      studentsInFilter={[student]}
      onSuccess={onSuccess}
    />,
  );
  await waitFor(() => expect(screen.getByLabelText("Montag")).toBeEnabled());
  fireEvent.change(screen.getByLabelText("Montag"), {
    target: { value: "08:00" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  return { ...view, save, onClose, onSuccess };
}

describe("FilteredBulkArrivalModal pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the dialog shut while saving and unlocks it after a failure", async () => {
    const { save, onClose } = await renderAndSave();

    expectSaveLocksDialog(TITLE);

    await act(async () => {
      save.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked(TITLE);
    expect(screen.getByLabelText("Montag")).toHaveValue("08:00");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("refreshes the data but closes nothing when the answer arrives after unmount", async () => {
    const { save, onClose, onSuccess, unmount } = await renderAndSave();
    expect(mocks.bulkUpsert).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      save.resolve({});
    });

    expect(onSuccess).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
  });
});
