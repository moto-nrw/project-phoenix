// #6533: Schließen während des Speicherns ist gesperrt, eine späte Antwort
// schließt keinen anderen Dialog. Echter FormModal.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { Student } from "~/lib/student-helpers";
import { bulkCreateStudentStatusDays } from "~/lib/student-status-days-api";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
}));
vi.mock("~/lib/student-status-days-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/student-status-days-api")>()),
  bulkCreateStudentStatusDays: vi.fn(),
}));

import { ClassTripBulkStatusModal } from "./class-trip-bulk-status-modal";

const TITLE = "Klassenfahrt planen: Klasse 3a";
const students = [
  {
    id: "42",
    name: "Kevin Anders",
    first_name: "Kevin",
    second_name: "Anders",
    school_class: "3a",
    current_location: "Zuhause",
  },
] satisfies Student[];

function renderAndSave() {
  const save = deferred<{ student_count: number; date_count: number }>();
  vi.mocked(bulkCreateStudentStatusDays).mockReturnValue(save.promise);
  const onClose = vi.fn();
  const onSuccess = vi.fn();
  const view = render(
    <ClassTripBulkStatusModal
      isOpen
      onClose={onClose}
      onSuccess={onSuccess}
      targetLabel="Klasse 3a"
      students={students}
    />,
  );
  fireEvent.click(
    screen.getByRole("button", { name: "Für 1 Schüler speichern" }),
  );
  return { ...view, save, onClose, onSuccess };
}

describe("ClassTripBulkStatusModal pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the dialog shut while saving and unlocks it after a failure", async () => {
    const { save, onClose } = renderAndSave();

    expectSaveLocksDialog(TITLE);

    await act(async () => {
      save.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked(TITLE);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("refreshes the data but closes nothing when the answer arrives after unmount", async () => {
    const { save, onClose, onSuccess, unmount } = renderAndSave();
    expect(bulkCreateStudentStatusDays).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      save.resolve({ student_count: 1, date_count: 1 });
    });

    expect(onSuccess).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
  });
});
