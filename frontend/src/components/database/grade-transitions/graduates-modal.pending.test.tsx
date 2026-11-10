// #6593: Die endgültige Löschung läuft in der Bestätigung, das Panel ist so
// lange ausgeblendet. Es kommt gesperrt zurück, bis die Liste das Ergebnis
// zeigt, damit dieselben Kinder nicht ein zweites Mal gelöscht werden. Eine
// späte Antwort lädt nichts in ein geschlossenes Panel. Echter SlideOver über
// dem gemeinsamen Vaul-Mock, echte ConfirmDeleteModal.
import {
  act,
  render as rtlRender,
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type {
  GradeTransition,
  TransitionHistoryEntry,
} from "~/lib/grade-transition-api";
import { catalogText } from "~/test/error-catalog-text";
import { deferred } from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { api } = vi.hoisted(() => ({
  api: {
    fetchTransitionHistory: vi.fn(),
    purgeGraduatedStudent: vi.fn(),
  },
}));
vi.mock("~/lib/grade-transition-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/grade-transition-api")>()),
  ...api,
}));

import { GraduatesModal } from "./graduates-modal";

const TITLE = "Abgänge: Schuljahreswechsel 2026-2027";

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

const alma: TransitionHistoryEntry = {
  id: "1",
  transitionId: "8",
  studentId: "100",
  personName: "Alma Alumna",
  fromClass: "4a",
  toClass: null,
  action: "graduated",
  createdAt: "2026-08-01T06:00:00Z",
  studentState: "alumnus",
};

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

function renderModal() {
  const onClose = vi.fn();
  const onPurged = vi.fn();
  const view = render(
    <GraduatesModal
      transition={transition}
      onClose={onClose}
      onPurged={onPurged}
      canDelete
    />,
  );
  return { ...view, onClose, onPurged };
}

/** Selects Alma and confirms the deletion with the text gate. */
async function purgeAlma() {
  fireEvent.click(
    await screen.findByRole("checkbox", { name: /Alma Alumna auswählen/i }),
  );
  const purge = screen.getByRole("button", { name: "Endgültig löschen" });
  expect(purge).toBeEnabled();
  fireEvent.click(purge);
  fireEvent.change(
    await screen.findByLabelText("Zur Bestätigung LÖSCHEN eingeben:"),
    { target: { value: "LÖSCHEN" } },
  );
  const confirm = screen.getByRole("button", {
    name: "1 Kind endgültig löschen",
  });
  expect(confirm).toBeEnabled();
  fireEvent.click(confirm);
}

function panel(): HTMLElement {
  const dialog = screen
    .getByRole("heading", { name: TITLE })
    .closest<HTMLElement>('[role="dialog"]');
  expect(dialog).not.toBeNull();
  return dialog as HTMLElement;
}

// The shared helpers find the X by its name; here the footer's button carries
// the same name „Schließen“, so both are checked.

/**
 * The panel is locked: busy, both „Schließen“ (X and footer) and the delete
 * disabled, no swipe; Escape and the backdrop do not close it.
 */
function expectPanelLocked(onClose: () => void) {
  const dialog = panel();
  expect(dialog).toHaveAttribute("aria-busy", "true");
  expect(dialog.closest("[data-dismissible]")).toHaveAttribute(
    "data-dismissible",
    "false",
  );
  const closes = within(dialog).getAllByRole("button", { name: "Schließen" });
  expect(closes).toHaveLength(2);
  for (const close of closes) expect(close).toBeDisabled();
  expect(
    within(dialog).getByRole("button", { name: "Endgültig löschen" }),
  ).toBeDisabled();
  fireEvent.keyDown(dialog, { key: "Escape" });
  fireEvent.click(document.querySelector("[data-vaul-overlay]") as Element);
  expect(onClose).not.toHaveBeenCalled();
}

/** The panel can be closed again: not busy, both „Schließen“ enabled. */
function expectPanelUnlocked() {
  const dialog = panel();
  expect(dialog).not.toHaveAttribute("aria-busy");
  expect(dialog.closest("[data-dismissible]")).toHaveAttribute(
    "data-dismissible",
    "true",
  );
  const closes = within(dialog).getAllByRole("button", { name: "Schließen" });
  expect(closes).toHaveLength(2);
  for (const close of closes) expect(close).toBeEnabled();
}

describe("GraduatesModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the panel locked until the list shows the deletion", async () => {
    const purged = deferred();
    const reload = deferred<TransitionHistoryEntry[]>();
    api.fetchTransitionHistory
      .mockResolvedValueOnce([alma])
      .mockReturnValueOnce(reload.promise);
    api.purgeGraduatedStudent.mockReturnValueOnce(purged.promise);
    const { onClose, onPurged } = renderModal();
    await purgeAlma();

    // Die Bestätigung löscht, das Panel ist ausgeblendet (#2774).
    expect(
      screen.getByRole("button", { name: "Wird gelöscht…" }),
    ).toBeDisabled();
    expect(screen.queryByRole("heading", { name: TITLE })).toBeNull();

    await act(async () => {
      purged.resolve();
    });

    expect(await screen.findByRole("heading", { name: TITLE })).toBeVisible();
    expectPanelLocked(onClose);
    expect(onPurged).toHaveBeenCalledOnce();

    await act(async () => {
      reload.resolve([{ ...alma, studentState: "purged" }]);
    });

    expect(await screen.findByText("Endgültig gelöscht")).toBeInTheDocument();
    expectPanelUnlocked();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("unlocks the panel after a refused deletion and reports it", async () => {
    const refused = deferred();
    api.fetchTransitionHistory.mockResolvedValue([alma]);
    api.purgeGraduatedStudent.mockReturnValueOnce(refused.promise);
    const { onClose, onPurged } = renderModal();
    await purgeAlma();

    await act(async () => {
      refused.reject(
        new ApiError("nein", 409, { code: "general.business_rejection" }),
      );
    });

    expect(
      await screen.findByText(
        catalogText(
          "general.business_rejection",
          "das Löschen von Alma Alumna",
        ),
      ),
    ).toBeInTheDocument();
    await waitFor(() => expectPanelUnlocked());
    expect(onPurged).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("refreshes the list behind but loads nothing after unmount", async () => {
    const purged = deferred();
    api.fetchTransitionHistory.mockResolvedValue([alma]);
    api.purgeGraduatedStudent.mockReturnValueOnce(purged.promise);
    const { onClose, onPurged, unmount } = renderModal();
    await purgeAlma();
    expect(api.purgeGraduatedStudent).toHaveBeenCalledWith("100");

    unmount();
    await act(async () => {
      purged.resolve();
    });

    // Gelöscht ist gelöscht: die Liste dahinter lädt neu.
    expect(onPurged).toHaveBeenCalledOnce();
    expect(api.fetchTransitionHistory).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
  });
});
