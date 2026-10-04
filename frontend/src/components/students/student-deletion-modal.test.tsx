import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ModalProvider } from "~/components/dashboard/modal-context";
import { ApiError } from "~/lib/api-error";
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

// Löschgrund + Bestätigungshaken sind die Voraussetzungen für die erste
// bewusste Löschbestätigung (#3304).
async function completePrerequisites() {
  const firstStepButton = await screen.findByRole("button", {
    name: "Ja, löschen",
  });
  expect(firstStepButton).toBeDisabled();

  fireEvent.click(screen.getByRole("combobox", { name: "Löschgrund" }));
  fireEvent.click(await screen.findByRole("option", { name: "Testdaten" }));
  fireEvent.click(screen.getByRole("checkbox"));

  return firstStepButton;
}

async function advanceToFinalConfirmation() {
  const firstStepButton = await completePrerequisites();
  expect(firstStepButton).toBeEnabled();
  fireEvent.click(firstStepButton);
  return screen.findByRole("button", { name: CONFIRM });
}

describe("StudentDeletionModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchImpact.mockResolvedValue(impact);
    mockDeleteStudent.mockResolvedValue(undefined);
  });

  it("shows the impact and requires a reason and acknowledgement", async () => {
    const { onDeleted } = renderModal();

    expect(await screen.findByText("Stundenplan-Zuordnungen")).toBeVisible();
    expect(
      screen.getByRole("dialog", { name: "Mia Muster löschen" }),
    ).toBeVisible();
    expect(screen.getByText("Anwesenheits- und Statusdaten")).toBeVisible();
    expect(
      screen.getByText(/Elternkonten und Profile der Erziehungsberechtigten/),
    ).toBeVisible();

    expect(
      screen.queryByLabelText(
        "Zur Sicherheit: Name des Kindes erneut eingeben",
      ),
    ).not.toBeInTheDocument();

    const finalButton = await advanceToFinalConfirmation();
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

  it("keeps the deletion confirmation locked before the prerequisites", async () => {
    renderModal();
    const firstStepButton = await screen.findByRole("button", {
      name: "Ja, löschen",
    });
    await screen.findByText("Stundenplan-Zuordnungen");

    expect(firstStepButton).toBeDisabled();
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

    fireEvent.click(await advanceToFinalConfirmation());

    // Catalog text for students.deletion_preview_changed, not the server's
    // sentence (ADR 0006).
    expect(await screen.findByText(/wurde inzwischen geändert/)).toBeVisible();
    expect(
      screen.queryByText("Die Daten haben sich geändert."),
    ).not.toBeInTheDocument();
    expect(mockFetchImpact).toHaveBeenCalledTimes(2);
    // The acknowledgement and confirmation step reset on the refreshed
    // preview, so deletion cannot be re-confirmed without looking again.
    expect(screen.getByRole("checkbox")).not.toBeChecked();
    expect(screen.getByRole("button", { name: "Ja, löschen" })).toBeDisabled();
  });

  it("does not expose the destructive controls when the preview fails", async () => {
    mockFetchImpact.mockRejectedValueOnce(
      new ApiError("Vorschau nicht verfügbar", 503, {
        code: "general.unavailable",
        instance: "req-impact",
      }),
    );
    renderModal();

    expect(
      await screen.findByText(
        "Die Vorschau der Löschung ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Ja, löschen" })).toBeDisabled();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByRole("checkbox")).toBeInTheDocument();
    expect(mockFetchImpact).toHaveBeenCalledTimes(2);
  });

  it("shows a failed deletion with request ID but without a retry button", async () => {
    mockDeleteStudent.mockRejectedValueOnce(
      new ApiError("boom", 500, {
        code: "general.server",
        instance: "req-delete",
      }),
    );
    renderModal();

    fireEvent.click(await advanceToFinalConfirmation());

    expect(
      await screen.findByText(
        "Die Löschung konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("req-delete");
    expect(
      screen.queryByRole("button", { name: "Wiederholen" }),
    ).not.toBeInTheDocument();
    expect(mockDeleteStudent).toHaveBeenCalledOnce();
  });

  it("does not offer a second deletion when refreshing after success fails", async () => {
    const onDeleted = vi.fn().mockRejectedValue(new Error("refresh failed"));
    const onClose = vi.fn();
    renderModal(onDeleted, onClose);

    fireEvent.click(await advanceToFinalConfirmation());

    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(mockDeleteStudent).toHaveBeenCalledOnce();
    expect(
      screen.queryByText("Das Kind konnte nicht gelöscht werden."),
    ).not.toBeInTheDocument();
  });
});
