// graduates-modal.test.tsx
// Behaviour of the Abgänge view — the only surface that shows children a grade
// transition soft-deleted, and the only one offering the endgültige Löschung
// (#405).

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { GraduatesModal } from "./graduates-modal";
import type {
  GradeTransition,
  TransitionHistoryEntry,
} from "~/lib/grade-transition-api";

const toastSuccess = vi.fn();
const toastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: toastSuccess,
    error: toastError,
    info: vi.fn(),
    warning: vi.fn(),
  }),
}));

const api = {
  fetchTransitionHistory: vi.fn<() => Promise<TransitionHistoryEntry[]>>(),
  purgeGraduatedStudent: vi.fn<(id: string) => Promise<void>>(),
};

vi.mock("~/lib/grade-transition-api", async (importOriginal) => {
  const original =
    await importOriginal<typeof import("~/lib/grade-transition-api")>();
  return {
    ...original,
    fetchTransitionHistory: (...args: unknown[]) =>
      api.fetchTransitionHistory(...(args as [])),
    purgeGraduatedStudent: (...args: unknown[]) =>
      api.purgeGraduatedStudent(...(args as [string])),
  };
});

const transition: GradeTransition = {
  id: "8",
  academicYear: "2026-2027",
  status: "applied",
  appliedAt: "2026-08-01T06:00:00Z",
  revertedAt: null,
  createdAt: "2026-07-24T09:00:00Z",
  notes: null,
  mappings: [],
  canModify: false,
  canApply: false,
  canRevert: true,
};

function entry(
  overrides: Partial<TransitionHistoryEntry>,
): TransitionHistoryEntry {
  return {
    id: "1",
    transitionId: "8",
    studentId: "100",
    personName: "Mika Muster",
    fromClass: "4a",
    toClass: null,
    action: "graduated",
    createdAt: "2026-08-01T06:00:00Z",
    studentState: "alumnus",
    ...overrides,
  };
}

const stillGone = entry({
  id: "1",
  studentId: "100",
  personName: "Alma Alumna",
  studentState: "alumnus",
});
const broughtBack = entry({
  id: "2",
  studentId: "200",
  personName: "Rudi Rueckkehr",
  studentState: "restored",
});
const alreadyPurged = entry({
  id: "3",
  studentId: "300",
  personName: "Gelöschtes Kind",
  studentState: "purged",
});
const promoted = entry({
  id: "4",
  studentId: "400",
  personName: "Paula Promoted",
  fromClass: "1a",
  toClass: "2a",
  action: "promoted",
  studentState: "restored",
});

beforeEach(() => {
  vi.clearAllMocks();
  api.purgeGraduatedStudent.mockResolvedValue(undefined);
});

function renderModal(canDelete = true) {
  const onClose = vi.fn();
  const onPurged = vi.fn();
  render(
    <GraduatesModal
      transition={transition}
      onClose={onClose}
      onPurged={onPurged}
      canDelete={canDelete}
    />,
  );
  return { onClose, onPurged };
}

describe("GraduatesModal", () => {
  it("lists only the Abgänge, never the promoted children", async () => {
    api.fetchTransitionHistory.mockResolvedValue([
      stillGone,
      broughtBack,
      promoted,
    ]);
    renderModal();

    expect(await screen.findByText("Alma Alumna")).toBeInTheDocument();
    expect(screen.getByText("Rudi Rueckkehr")).toBeInTheDocument();
    // A promoted child was never hidden and is edited in the Kindersuche like
    // any other — showing it here would suggest it is deletable from here.
    expect(screen.queryByText("Paula Promoted")).not.toBeInTheDocument();
  });

  it("only offers deletion for children that are still gone", async () => {
    api.fetchTransitionHistory.mockResolvedValue([
      stillGone,
      broughtBack,
      alreadyPurged,
    ]);
    renderModal();

    await screen.findByText("Alma Alumna");

    // The ledger says "graduated" for all three; only the resolved state may
    // decide. Deleting a restored child would remove an active one.
    expect(
      screen.getByRole("checkbox", { name: /Alma Alumna auswählen/i }),
    ).toBeEnabled();
    expect(
      screen.getByRole("checkbox", { name: /Rudi Rueckkehr auswählen/i }),
    ).toBeDisabled();
    expect(
      screen.getByRole("checkbox", { name: /Gelöschtes Kind auswählen/i }),
    ).toBeDisabled();
  });

  it("purges the selected children and refreshes the list", async () => {
    api.fetchTransitionHistory
      .mockResolvedValueOnce([stillGone, broughtBack])
      .mockResolvedValueOnce([
        { ...stillGone, studentState: "purged" },
        broughtBack,
      ]);
    const { onPurged } = renderModal();

    await screen.findByText("Alma Alumna");
    fireEvent.click(
      screen.getByRole("checkbox", { name: /Alma Alumna auswählen/i }),
    );

    fireEvent.click(screen.getByRole("button", { name: /Endgültig löschen/i }));
    fireEvent.click(
      await screen.findByRole("button", { name: /Ja, endgültig löschen/i }),
    );

    await waitFor(() => {
      expect(api.purgeGraduatedStudent).toHaveBeenCalledWith("100");
    });
    expect(api.purgeGraduatedStudent).toHaveBeenCalledTimes(1);
    // Never the restored child, even though it sits in the same ledger.
    expect(api.purgeGraduatedStudent).not.toHaveBeenCalledWith("200");

    await waitFor(() => {
      expect(toastSuccess).toHaveBeenCalledWith("1 Kind endgültig gelöscht.");
    });
    // The transition list behind the modal counts Abgänge, so it has to refetch.
    expect(onPurged).toHaveBeenCalled();
    await waitFor(() => {
      expect(api.fetchTransitionHistory).toHaveBeenCalledTimes(2);
    });
  });

  it("reports a refused deletion without stopping the others", async () => {
    const second = entry({
      id: "5",
      studentId: "500",
      personName: "Bodo Begleiter",
      studentState: "alumnus",
    });
    api.fetchTransitionHistory.mockResolvedValue([stillGone, second]);
    // Mirrors the backend's companion precondition: deleting this child would
    // strand another child's Laufgemeinschaft.
    api.purgeGraduatedStudent.mockImplementation(async (id: string) => {
      if (id === "500") {
        throw new Error("Kind kann nicht gelöscht werden: verknüpfte Daten");
      }
    });
    renderModal();

    await screen.findByText("Alma Alumna");
    fireEvent.click(screen.getByRole("checkbox", { name: /Alle löschbaren/i }));
    fireEvent.click(screen.getByRole("button", { name: /Endgültig löschen/i }));
    fireEvent.click(
      await screen.findByRole("button", { name: /Ja, endgültig löschen/i }),
    );

    await waitFor(() => {
      expect(api.purgeGraduatedStudent).toHaveBeenCalledTimes(2);
    });
    // One failure must not swallow the success, nor the other way round.
    await waitFor(() => {
      expect(toastSuccess).toHaveBeenCalledWith("1 Kind endgültig gelöscht.");
    });
    expect(toastError).toHaveBeenCalledWith(
      expect.stringContaining("Bodo Begleiter"),
    );
  });

  it("hides the deletion entirely without the permission", async () => {
    api.fetchTransitionHistory.mockResolvedValue([stillGone]);
    renderModal(false);

    await screen.findByText("Alma Alumna");
    expect(
      screen.queryByRole("button", { name: /Endgültig löschen/i }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("says so when the transition had no Abgänge", async () => {
    api.fetchTransitionHistory.mockResolvedValue([promoted]);
    renderModal();

    expect(await screen.findByText(/keine Abgänge/i)).toBeInTheDocument();
  });

  it("surfaces a failed load instead of looking empty", async () => {
    api.fetchTransitionHistory.mockRejectedValue(new Error("boom"));
    renderModal();

    // An empty list and a broken request must not look the same: the first
    // means "nobody left", the second means "we do not know".
    expect(
      await screen.findByText(/konnten nicht geladen werden/i),
    ).toBeInTheDocument();
  });
});
