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

function renderModal(
  onDeleted = vi.fn(),
  onClose = vi.fn(),
  completionId?: string,
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
      />
    </ModalProvider>,
  );
  return { onDeleted, onClose };
}

async function completeFirstStep() {
  const nextButton = await screen.findByRole("button", {
    name: "Weiter",
  });
  expect(nextButton).toBeDisabled();

  fireEvent.click(screen.getByRole("combobox", { name: "Löschgrund" }));
  fireEvent.click(await screen.findByRole("option", { name: "Testdaten" }));
  fireEvent.click(screen.getByRole("checkbox"));

  expect(nextButton).toBeEnabled();
  fireEvent.click(nextButton);
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
      screen.getByRole("heading", { name: "Mia Muster löschen" }),
    ).toBeVisible();
    expect(screen.getByText("Anwesenheits- und Statusdaten")).toBeVisible();
    expect(
      screen.getByText(/Elternkonten und Profile der Erziehungsberechtigten/),
    ).toBeVisible();

    await completeFirstStep();
    expect(
      screen.getByRole("heading", {
        name: "Löschung von Mia Muster bestätigen",
      }),
    ).toBeVisible();

    const finalButton = screen.getByRole("button", {
      name: "Kind endgültig löschen",
    });
    expect(finalButton).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Name erneut eingeben"), {
      target: { value: "Mia Muste" },
    });
    expect(finalButton).toBeDisabled();
    fireEvent.change(screen.getByLabelText("Name erneut eingeben"), {
      target: { value: "Mia Muster" },
    });
    expect(finalButton).toBeEnabled();
    fireEvent.click(finalButton);

    await waitFor(() => {
      expect(mockDeleteStudent).toHaveBeenCalledWith("42", {
        expected_fingerprint: "abc123",
        confirmation_name: "Mia Muster",
        reason: "test_data",
        acknowledged: true,
      });
    });
    expect(onDeleted).toHaveBeenCalledOnce();
  });

  it("returns to a refreshed impact when the backend reports a conflict", async () => {
    const { StudentDeletionApiError } = await import("~/lib/student-api");
    mockDeleteStudent.mockRejectedValueOnce(
      new StudentDeletionApiError(409, "Die Daten haben sich geändert."),
    );
    renderModal();

    await completeFirstStep();
    fireEvent.change(screen.getByLabelText("Name erneut eingeben"), {
      target: { value: "Mia Muster" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Kind endgültig löschen" }),
    );

    expect(
      await screen.findByRole("button", {
        name: "Weiter",
      }),
    ).toBeDisabled();
    expect(screen.getByText("Die Daten haben sich geändert.")).toBeVisible();
    expect(mockFetchImpact).toHaveBeenCalledTimes(2);
    expect(screen.getByRole("checkbox")).not.toBeChecked();
  });

  it("does not expose the destructive controls when the preview fails", async () => {
    mockFetchImpact.mockRejectedValueOnce(
      new Error("Vorschau nicht verfügbar"),
    );
    renderModal();

    expect(await screen.findByText("Vorschau nicht verfügbar")).toBeVisible();
    expect(screen.getByRole("button", { name: "Weiter" })).toBeDisabled();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("does not offer a second deletion when refreshing after success fails", async () => {
    const onDeleted = vi.fn().mockRejectedValue(new Error("refresh failed"));
    const onClose = vi.fn();
    renderModal(onDeleted, onClose);

    await completeFirstStep();
    fireEvent.change(screen.getByLabelText("Name erneut eingeben"), {
      target: { value: "Mia Muster" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Kind endgültig löschen" }),
    );

    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(mockDeleteStudent).toHaveBeenCalledOnce();
    expect(
      screen.queryByText("Das Kind konnte nicht gelöscht werden."),
    ).not.toBeInTheDocument();
  });
});
