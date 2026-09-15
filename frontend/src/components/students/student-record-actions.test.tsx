// Die Datensatz-Aktionen der Kindakte (#2487, #3115): Betreuung beenden, ein
// geplantes Ende ändern oder stornieren, nach dem Austritt wieder aufnehmen,
// Löschen. Sie lagen vorher im Pane der Kinderdaten und liegen jetzt im
// Kebab der einen Objektansicht.
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

import { StudentRecordActions } from "./student-record-actions";

const { mockCancelCareExit } = vi.hoisted(() => ({
  mockCancelCareExit: vi.fn(),
}));

vi.mock("~/lib/care-exit-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/care-exit-api")>();
  return { ...actual, cancelCareExit: mockCancelCareExit };
});

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: toastSuccess, error: toastError }),
}));

vi.mock("./care-exit-modal", () => ({
  CareExitModal: ({
    studentIds,
    plannedLastCareDay,
    onFinished,
  }: {
    studentIds: readonly string[];
    plannedLastCareDay?: string;
    onFinished: () => void;
  }) => (
    <div data-testid="care-exit-modal" data-planned={plannedLastCareDay ?? ""}>
      {studentIds.join(",")}
      <button type="button" onClick={onFinished}>
        Ende eintragen
      </button>
    </div>
  ),
}));

vi.mock("./care-resume-modal", () => ({
  CareResumeModal: ({ studentId }: { studentId: string }) => (
    <div data-testid="care-resume-modal">{studentId}</div>
  ),
}));

vi.mock("./student-deletion-modal", () => ({
  StudentDeletionModal: ({
    studentId,
    careEnded,
    onDeleted,
  }: {
    studentId: string;
    careEnded?: boolean;
    onDeleted: () => void;
  }) => (
    <div data-testid="deletion-modal" data-care-ended={String(careEnded)}>
      {studentId}
      <button type="button" onClick={onDeleted}>
        Endgültig löschen
      </button>
    </div>
  ),
}));

function isoInDays(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

const RUNNING = { id: "1" };
const PLANNED = {
  ...RUNNING,
  care_ends_on: isoInDays(14),
  care_ended: false,
  care_exit_recorded: true,
};
// Ein Ende weit voraus, das die Schule selbst eingetragen hat. Es muss sich
// genauso ändern und stornieren lassen wie ein nahes (#2487).
const PLANNED_FAR_AHEAD = {
  ...RUNNING,
  care_ends_on: isoInDays(200),
  care_ended: false,
  care_exit_recorded: true,
};
// Kein eingetragener Austritt, nur das Ende der Anmeldephase.
const PHASE_END_ONLY = {
  ...RUNNING,
  care_ends_on: isoInDays(200),
  care_ended: false,
};
const ENDED = {
  ...RUNNING,
  care_ends_on: isoInDays(-3),
  care_ended: true,
  care_exit_recorded: true,
};
// Beendet, aber ohne eingetragenen Austritt: die Anmeldephase lief aus.
const ENDED_WITHOUT_EXIT = {
  ...RUNNING,
  care_ends_on: isoInDays(-3),
  care_ended: true,
};

function renderWith(student: Record<string, unknown>) {
  const onChanged = vi.fn(() => Promise.resolve());
  const onDeleted = vi.fn(() => Promise.resolve());
  render(
    <StudentRecordActions
      student={student as { id: string }}
      displayName="Max Mustermann"
      onChanged={onChanged}
      onDeleted={onDeleted}
    />,
  );
  return { onChanged, onDeleted };
}

function openMenu() {
  fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
}

function menuItem(name: string | RegExp) {
  return screen.queryByRole("menuitem", { name });
}

describe("StudentRecordActions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCancelCareExit.mockResolvedValue(1);
  });

  it("offers ending the care for a child without a planned exit", () => {
    renderWith(RUNNING);
    openMenu();
    expect(menuItem(/Betreuung beenden/)).toBeVisible();
    expect(menuItem("Ende stornieren")).toBeNull();
    expect(menuItem("Löschen")).toBeVisible();
  });

  it("switches to change and cancel once an exit is planned", () => {
    renderWith(PLANNED);
    openMenu();
    expect(menuItem(/Ende ändern/)).toBeVisible();
    expect(menuItem("Ende stornieren")).toBeVisible();
  });

  it("opens the exit dialog with the planned day for a change", () => {
    renderWith(PLANNED);
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: /Ende ändern/ }));
    expect(screen.getByTestId("care-exit-modal")).toHaveAttribute(
      "data-planned",
      PLANNED.care_ends_on,
    );
  });

  it("keeps change and cancel for an exit planned far ahead", () => {
    renderWith(PLANNED_FAR_AHEAD);
    openMenu();
    expect(menuItem(/Ende ändern/)).toBeVisible();
    expect(menuItem("Ende stornieren")).toBeVisible();
  });

  it("offers no cancellation for a mere end of the enrolment phase", () => {
    renderWith(PHASE_END_ONLY);
    openMenu();
    expect(menuItem(/Betreuung beenden/)).toBeVisible();
    expect(menuItem("Ende stornieren")).toBeNull();
  });

  it("keeps the planned exit when the cancellation is dismissed", () => {
    renderWith(PLANNED);
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: "Ende stornieren" }));
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(mockCancelCareExit).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("dialog", { name: "Betreuungsende stornieren?" }),
    ).toBeNull();
  });

  // #3109: „Ende stornieren“ asks first; the exit is only cancelled from
  // the dialog's own „Ende stornieren“.
  function confirmCancelExit() {
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: "Ende stornieren" }));
    expect(mockCancelCareExit).not.toHaveBeenCalled();
    const dialog = screen.getByRole("dialog", {
      name: "Betreuungsende stornieren?",
    });
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Ende stornieren" }),
    );
  }

  it("cancels the planned exit, says so and refreshes the record", async () => {
    const { onChanged } = renderWith(PLANNED);
    confirmCancelExit();

    await waitFor(() => expect(mockCancelCareExit).toHaveBeenCalledWith(["1"]));
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith(
        expect.stringContaining("storniert"),
      ),
    );
    expect(onChanged).toHaveBeenCalled();
  });

  it("shows the server's reason when the cancellation is refused", async () => {
    mockCancelCareExit.mockRejectedValue(
      new Error("Die Betreuung ist bereits beendet."),
    );
    renderWith(PLANNED);
    confirmCancelExit();

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith(
        "Die Betreuung ist bereits beendet.",
      ),
    );
  });

  it("offers resuming the care once the exit took effect", () => {
    renderWith(ENDED);
    openMenu();
    expect(menuItem(/Wieder aufnehmen/)).toBeVisible();
    expect(menuItem(/Betreuung beenden/)).toBeNull();
    fireEvent.click(screen.getByRole("menuitem", { name: /Wieder aufnehmen/ }));
    expect(screen.getByTestId("care-resume-modal")).toHaveTextContent("1");
  });

  it("offers no resumption when no exit is recorded", () => {
    renderWith(ENDED_WITHOUT_EXIT);
    openMenu();
    expect(menuItem(/Wieder aufnehmen/)).toBeNull();
    expect(menuItem(/Betreuung beenden/)).toBeNull();
    expect(menuItem("Löschen")).toBeVisible();
  });

  it("refreshes the record after an exit was recorded", async () => {
    const { onChanged } = renderWith(RUNNING);
    openMenu();
    fireEvent.click(
      screen.getByRole("menuitem", { name: /Betreuung beenden/ }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Ende eintragen" }));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
  });

  it("leaves the record once the child was deleted", async () => {
    const { onDeleted } = renderWith(ENDED);
    openMenu();
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));
    expect(screen.getByTestId("deletion-modal")).toHaveAttribute(
      "data-care-ended",
      "true",
    );
    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));
    await waitFor(() => expect(onDeleted).toHaveBeenCalled());
    expect(toastSuccess).toHaveBeenCalledWith(
      expect.stringContaining("gelöscht"),
    );
  });
});
