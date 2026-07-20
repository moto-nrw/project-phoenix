import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  cleanup,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";

/**
 * The EDITABLE Stammdaten view holds a draft plus the snapshot its dirty check
 * compares against, and the list it submits REPLACES the stored one. When
 * someone else changes the same child's links in the meantime, saving that
 * draft would delete their change (#1694). These tests pin the two reactions:
 * refetch while nothing is edited, block and ask while something is.
 */

const { fetchStudentCompanionsMock, toastErrorMock } = vi.hoisted(() => ({
  fetchStudentCompanionsMock: vi.fn(),
  toastErrorMock: vi.fn(),
}));

vi.mock("~/lib/student-companion-api", async (importOriginal) => {
  // Only the fetch is faked — subscribe/notify are the mechanism under test.
  const actual =
    await importOriginal<typeof import("~/lib/student-companion-api")>();
  return { ...actual, fetchStudentCompanions: fetchStudentCompanionsMock };
});

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    error: toastErrorMock,
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
  }),
}));

// The departure section owns the picker; the tests drive the companion list
// through this stand-in instead of the real weekday/child UI.
vi.mock("./student-form-fields", () => ({
  DepartureSection: ({
    companions,
    onCompanionsChange,
  }: {
    companions?: { companion_student_id: string; weekdays: string[] }[];
    onCompanionsChange?: (
      next: { companion_student_id: string; weekdays: string[] }[],
    ) => void;
  }) => (
    <div>
      <span data-testid="companion-count">{companions?.length ?? 0}</span>
      <button
        type="button"
        data-testid="edit-companions"
        onClick={() =>
          onCompanionsChange?.([
            { companion_student_id: "9", weekdays: ["mon"] },
          ])
        }
      >
        edit
      </button>
    </div>
  ),
  PersonalInfoSection: () => <div />,
  EnrollmentConsentsSection: () => <div />,
}));

import { PersonalInfoFormModal } from "./personal-info-form-modal";
import { notifyStudentCompanionsChanged } from "~/lib/student-companion-api";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";

const student = {
  id: "1",
  first_name: "Max",
  second_name: "Mustermann",
  name: "Max Mustermann",
  bus_days: {},
} as ExtendedStudent;

function companion(id: string) {
  return { companion_student_id: id, weekdays: ["mon"] as const };
}

async function renderOpenModal(onSave = vi.fn().mockResolvedValue(undefined)) {
  render(
    <PersonalInfoFormModal
      isOpen
      onClose={vi.fn()}
      student={student}
      onSave={onSave}
    />,
  );
  await waitFor(() =>
    expect(fetchStudentCompanionsMock).toHaveBeenCalledTimes(1),
  );
  return onSave;
}

describe("PersonalInfoFormModal — remote companion changes", () => {
  beforeEach(() => {
    fetchStudentCompanionsMock.mockReset();
    toastErrorMock.mockReset();
  });

  afterEach(() => {
    cleanup();
  });

  it("refetches the links when nothing has been edited yet", async () => {
    fetchStudentCompanionsMock.mockResolvedValueOnce([companion("2")]);
    await renderOpenModal();
    await waitFor(() =>
      expect(screen.getByTestId("companion-count").textContent).toBe("1"),
    );

    // Someone else added a second child to the Laufgemeinschaft.
    fetchStudentCompanionsMock.mockResolvedValueOnce([
      companion("2"),
      companion("3"),
    ]);
    act(() => {
      notifyStudentCompanionsChanged();
    });

    await waitFor(() =>
      expect(screen.getByTestId("companion-count").textContent).toBe("2"),
    );
  });

  it("blocks the save instead of overwriting a remote change with an in-progress edit", async () => {
    fetchStudentCompanionsMock.mockResolvedValueOnce([companion("2")]);
    const onSave = await renderOpenModal();
    await waitFor(() =>
      expect(screen.getByTestId("companion-count").textContent).toBe("1"),
    );

    fireEvent.click(screen.getByTestId("edit-companions"));

    // The remote change lands while the user is still editing — refetching now
    // would silently throw their draft away, so the modal asks instead.
    fetchStudentCompanionsMock.mockResolvedValueOnce([
      companion("2"),
      companion("3"),
    ]);
    act(() => {
      notifyStudentCompanionsChanged();
    });

    expect(
      await screen.findByText(/zwischenzeitlich an anderer Stelle geändert/),
    ).toBeInTheDocument();
    // The draft is still there — nothing was discarded behind the user's back.
    expect(screen.getByTestId("companion-count").textContent).toBe("1");
    expect(fetchStudentCompanionsMock).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByText("Speichern"));
    await waitFor(() => expect(toastErrorMock).toHaveBeenCalled());
    expect(onSave).not.toHaveBeenCalled();

    // "Neu laden" is the explicit way out: the stored links come back and the
    // form is saveable again.
    fireEvent.click(screen.getByText("Neu laden"));
    await waitFor(() =>
      expect(screen.getByTestId("companion-count").textContent).toBe("2"),
    );
    expect(
      screen.queryByText(/zwischenzeitlich an anderer Stelle geändert/),
    ).not.toBeInTheDocument();
  });

  it("does not flag itself stale for the announcement its own save makes", async () => {
    fetchStudentCompanionsMock.mockResolvedValueOnce([companion("2")]);
    const onSave = vi.fn().mockImplementation(async () => {
      // updateStudent announces on the same bus during the save.
      notifyStudentCompanionsChanged();
    });
    await renderOpenModal(onSave);
    await waitFor(() =>
      expect(screen.getByTestId("companion-count").textContent).toBe("1"),
    );

    fireEvent.click(screen.getByTestId("edit-companions"));
    fetchStudentCompanionsMock.mockResolvedValue([companion("9")]);
    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(toastErrorMock).not.toHaveBeenCalled();
    expect(
      screen.queryByText(/zwischenzeitlich an anderer Stelle geändert/),
    ).not.toBeInTheDocument();
  });
});
