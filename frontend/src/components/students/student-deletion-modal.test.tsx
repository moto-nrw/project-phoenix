import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ModalProvider } from "~/components/dashboard/modal-context";
import type { StudentDeletionImpact } from "~/lib/student-api";
import { StudentDeletionModal } from "./student-deletion-modal";

const { mockFetchImpact, mockDeleteStudent } = vi.hoisted(() => ({
  mockFetchImpact: vi.fn(),
  mockDeleteStudent: vi.fn(),
}));

vi.mock("~/lib/student-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/student-api")>();
  return {
    ...actual,
    fetchStudentDeletionImpact: mockFetchImpact,
    deleteStudentWithData: mockDeleteStudent,
  };
});

const impact: StudentDeletionImpact = {
  confirmation_name: "Mia Muster",
  fingerprint: "abc123",
  total: 5,
  counts: {
    timetable_assignments: 2,
    activity_enrollments: 0,
    attendance_records: 1,
    care_schedules: 1,
    guardian_links: 1,
    companion_links: 0,
    communications: 0,
    consents: 0,
    enrollment_references: 0,
    other_records: 0,
  },
  preserved: {
    guardian_profiles: true,
    parent_accounts: true,
    other_students: true,
    shared_instances: true,
  },
};

const NAME_LABEL = "Zur Sicherheit: Name des Kindes erneut eingeben";
const CONFIRM = "Kind endgültig löschen";

function renderModal(
  onDeleted = vi.fn(),
  onClose = vi.fn(),
  completionId?: string,
  extra: Partial<React.ComponentProps<typeof StudentDeletionModal>> = {},
) {
  render(
    <ModalProvider>
      <StudentDeletionModal
        isOpen
        studentId="42"
        displayName="Mia Muster"
        completionId={completionId}
        onClose={onClose}
        onDeleted={onDeleted}
        {...extra}
      />
    </ModalProvider>,
  );
  return { onDeleted, onClose };
}

// Löschgrund + Bestätigungshaken sind die Voraussetzungen; die Namenseingabe
// ist das Gate der ConfirmDeleteModal (#3110).
async function completePrerequisites() {
  const confirmButton = await screen.findByRole("button", { name: CONFIRM });
  expect(confirmButton).toBeDisabled();

  fireEvent.click(screen.getByRole("combobox", { name: "Löschgrund" }));
  fireEvent.click(await screen.findByRole("option", { name: "Testdaten" }));
  fireEvent.click(screen.getByRole("checkbox"));

  return confirmButton;
}

describe("StudentDeletionModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchImpact.mockResolvedValue(impact);
    mockDeleteStudent.mockResolvedValue(undefined);
  });

  it("shows the impact and requires reason, acknowledgement and exact name", async () => {
    const { onDeleted } = renderModal();

    expect(await screen.findByText("Stundenplan-Zuordnungen")).toBeVisible();
    expect(
      screen.getByRole("dialog", { name: "Mia Muster löschen" }),
    ).toBeVisible();
    expect(screen.getByText("Anwesenheits- und Statusdaten")).toBeVisible();
    expect(
      screen.getByText(/Elternkonten und Profile der Erziehungsberechtigten/),
    ).toBeVisible();

    const finalButton = await completePrerequisites();
    // Reason and acknowledgement alone do not open the gate.
    expect(finalButton).toBeDisabled();

    fireEvent.change(screen.getByLabelText(NAME_LABEL), {
      target: { value: "Mia Muste" },
    });
    expect(finalButton).toBeDisabled();
    fireEvent.change(screen.getByLabelText(NAME_LABEL), {
      target: { value: "Mia Muster" },
    });
    expect(finalButton).toBeEnabled();
    fireEvent.click(finalButton);

    await waitFor(() => {
      expect(mockDeleteStudent).toHaveBeenCalledWith(
        "42",
        {
          expected_fingerprint: "abc123",
          confirmation_name: "Mia Muster",
          reason: "test_data",
          acknowledged: true,
        },
        undefined,
      );
    });
    expect(onDeleted).toHaveBeenCalledOnce();
  });

  it("keeps the typed name from opening the gate before the prerequisites", async () => {
    renderModal();
    const finalButton = await screen.findByRole("button", { name: CONFIRM });
    await screen.findByText("Stundenplan-Zuordnungen");

    fireEvent.change(screen.getByLabelText(NAME_LABEL), {
      target: { value: "Mia Muster" },
    });
    expect(finalButton).toBeDisabled();
    expect(mockDeleteStudent).not.toHaveBeenCalled();
  });

  it("names the immediate deletion when opened from an open withdrawal", async () => {
    renderModal(vi.fn(), vi.fn(), "completion-1", { skipsLastCareDay: true });

    expect(
      await screen.findByText(/Auch ein späterer letzter Betreuungstag/),
    ).toBeVisible();
    expect(mockFetchImpact).toHaveBeenCalledWith("42", "completion-1");
  });

  it("returns to a refreshed impact when the backend reports a conflict", async () => {
    const { StudentDeletionApiError } = await import("~/lib/student-api");
    mockDeleteStudent.mockRejectedValueOnce(
      new StudentDeletionApiError(
        409,
        "Die Daten haben sich geändert.",
        "students.deletion_preview_changed",
      ),
    );
    mockFetchImpact
      .mockResolvedValueOnce(impact)
      .mockResolvedValueOnce({ ...impact, fingerprint: "def456" });
    renderModal();

    await completePrerequisites();
    fireEvent.change(screen.getByLabelText(NAME_LABEL), {
      target: { value: "Mia Muster" },
    });
    fireEvent.click(screen.getByRole("button", { name: CONFIRM }));

    expect(
      await screen.findByText("Die Daten haben sich geändert."),
    ).toBeVisible();
    expect(mockFetchImpact).toHaveBeenCalledTimes(2);
    // Acknowledgement and typed name reset on the refreshed preview, so the
    // deletion cannot be re-confirmed without looking again.
    expect(screen.getByRole("checkbox")).not.toBeChecked();
    expect(screen.getByLabelText(NAME_LABEL)).toHaveValue("");
    expect(screen.getByRole("button", { name: CONFIRM })).toBeDisabled();
  });

  it("does not expose the destructive controls when the preview fails", async () => {
    mockFetchImpact.mockRejectedValueOnce(
      new Error("Vorschau nicht verfügbar"),
    );
    renderModal();

    expect(await screen.findByText("Vorschau nicht verfügbar")).toBeVisible();
    expect(screen.getByRole("button", { name: CONFIRM })).toBeDisabled();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("does not offer a second deletion when refreshing after success fails", async () => {
    const onDeleted = vi.fn().mockRejectedValue(new Error("refresh failed"));
    const onClose = vi.fn();
    renderModal(onDeleted, onClose);

    await completePrerequisites();
    fireEvent.change(screen.getByLabelText(NAME_LABEL), {
      target: { value: "Mia Muster" },
    });
    fireEvent.click(screen.getByRole("button", { name: CONFIRM }));

    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(mockDeleteStudent).toHaveBeenCalledOnce();
    expect(
      screen.queryByText("Das Kind konnte nicht gelöscht werden."),
    ).not.toBeInTheDocument();
  });
});
