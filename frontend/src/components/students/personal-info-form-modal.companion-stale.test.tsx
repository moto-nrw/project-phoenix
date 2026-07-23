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
    onChange,
  }: {
    companions?: { companion_student_id: string; weekdays: string[] }[];
    onCompanionsChange?: (
      next: { companion_student_id: string; weekdays: string[] }[],
    ) => void;
    onChange?: (next: Record<string, string[]>) => void;
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
      {/* Drops Tuesday from the plan without touching the picker. */}
      <button
        type="button"
        data-testid="narrow-plan"
        onClick={() => onChange?.({ mon: ["accompanied"] })}
      >
        narrow
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

// A child accompanied on Monday AND Tuesday, so the plan can be narrowed to
// Monday alone without the picker being touched.
const accompaniedStudent = {
  ...student,
  allowed_departure_modes: {
    mon: ["accompanied"],
    tue: ["accompanied"],
  },
} as ExtendedStudent;

function companion(id: string) {
  return { companion_student_id: id, weekdays: ["mon"] as const };
}

async function renderOpenModal(
  onSave = vi.fn().mockResolvedValue(undefined),
  studentProp: ExtendedStudent = student,
) {
  render(
    <PersonalInfoFormModal
      isOpen
      onClose={vi.fn()}
      student={studentProp}
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

  // The list REPLACES the stored one, so the backend has to be told WHICH
  // stored list it replaces — a second editor starting from the same snapshot
  // would otherwise silently delete the first one's link.
  it("submits the fingerprint of the list it loaded", async () => {
    fetchStudentCompanionsMock.mockResolvedValueOnce([companion("2")]);
    const onSave = await renderOpenModal();
    await waitFor(() =>
      expect(screen.getByTestId("companion-count").textContent).toBe("1"),
    );

    fireEvent.click(screen.getByTestId("edit-companions"));
    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(onSave.mock.calls[0]?.[0]).toMatchObject({
      companions: [{ companion_student_id: "9", weekdays: ["mon"] }],
      companions_fingerprint: "2:mon",
    });
  });

  // The in-tab bus cannot see a write from another browser, so the backend's
  // 409 is the only signal there — and it has to leave the form in the same
  // state a local announcement would: draft kept, save blocked, reload offered.
  it("blocks the form when the backend reports the list as stale", async () => {
    fetchStudentCompanionsMock.mockResolvedValueOnce([companion("2")]);
    const staleError = Object.assign(
      new Error(
        "Die Laufgemeinschaft dieses Kindes wurde zwischenzeitlich geändert. Bitte neu laden und noch einmal speichern.",
      ),
      {
        name: "CompanionsChangedError",
        body: JSON.stringify({ code: "companions_changed" }),
      },
    );
    const onSave = vi.fn().mockRejectedValue(staleError);
    await renderOpenModal(onSave);
    await waitFor(() =>
      expect(screen.getByTestId("companion-count").textContent).toBe("1"),
    );

    fireEvent.click(screen.getByTestId("edit-companions"));
    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => expect(toastErrorMock).toHaveBeenCalled());
    expect(toastErrorMock.mock.calls.at(-1)?.[0]).toContain(
      "zwischenzeitlich geändert",
    );
    // The draft survives, and the form now offers the explicit way out.
    expect(screen.getByTestId("companion-count").textContent).toBe("1");
    expect(
      await screen.findByText(/zwischenzeitlich an anderer Stelle geändert/),
    ).toBeInTheDocument();
  });

  // Narrowing the plan is a pending DELETE of every link on the days it drops
  // (the backend trims them), so it has to protect the draft exactly like an
  // edited list does. Refetching here would make the other person's brand-new
  // Tuesday link this save's own baseline — and then trim it away.
  it("blocks the save when only the departure plan was narrowed", async () => {
    fetchStudentCompanionsMock.mockResolvedValueOnce([companion("2")]);
    const onSave = await renderOpenModal(
      vi.fn().mockResolvedValue(undefined),
      accompaniedStudent,
    );
    await waitFor(() =>
      expect(screen.getByTestId("companion-count").textContent).toBe("1"),
    );

    fireEvent.click(screen.getByTestId("narrow-plan"));

    // Someone else adds a link on the weekday this draft is about to drop.
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
    // No refetch: the loaded list stays the baseline, so it cannot be swapped
    // for one that already contains the foreign link.
    expect(fetchStudentCompanionsMock).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByText("Speichern"));
    await waitFor(() => expect(toastErrorMock).toHaveBeenCalled());
    expect(onSave).not.toHaveBeenCalled();
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
