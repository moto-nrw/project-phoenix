// #6593: Das Anwenden läuft in der Bestätigung oder über „Wiederholen“ im
// Panel. In beiden Fällen ist das Panel gesperrt, und eine späte Antwort
// schließt keinen späteren Dialog. Echter SlideOver über dem gemeinsamen
// Vaul-Mock, echte ConfirmationModal.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type {
  GradeTransition,
  TransitionPreview,
  TransitionResult,
} from "~/lib/grade-transition-api";
import { catalogText } from "~/test/error-catalog-text";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { api } = vi.hoisted(() => ({
  api: {
    previewGradeTransition: vi.fn(),
    applyGradeTransition: vi.fn(),
  },
}));
vi.mock("~/lib/grade-transition-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/grade-transition-api")>()),
  ...api,
}));

import { TransitionPreviewModal } from "./transition-preview-modal";

const TITLE = "Vorschau: Schuljahreswechsel 2026-2027";
const FOOTER = ["Zurück zum Entwurf", "Schuljahreswechsel anwenden"];
const APPLY_ERROR = catalogText(
  "general.server",
  "das Anwenden des Schuljahreswechsels",
);

const transition: GradeTransition = {
  id: "8",
  academicYear: "2026-2027",
  status: "draft",
  appliedAt: null,
  revertedAt: null,
  createdAt: "2026-07-24T09:00:00Z",
  notes: null,
  mappings: [{ id: "1", fromClass: "1a", toClass: "2a", action: "promote" }],
  canModify: true,
  canApply: true,
  canRevert: false,
};

const preview: TransitionPreview = {
  transitionId: "8",
  academicYear: "2026-2027",
  totalStudents: 12,
  toPromote: 12,
  toGraduate: 0,
  byMapping: [
    { fromClass: "1a", toClass: "2a", studentCount: 12, action: "promote" },
  ],
  unmappedClasses: [],
  warnings: [],
  fingerprint: "abc",
};

const result: TransitionResult = {
  transitionId: "8",
  status: "applied",
  studentsPromoted: 12,
  studentsGraduated: 0,
  canRevert: true,
  warnings: [],
};

const serverError = () =>
  new ApiError("kaputt", 500, { code: "general.server" });

function renderPreview() {
  const onClose = vi.fn();
  const onBackToEditor = vi.fn();
  const onApplied = vi.fn();
  const view = render(
    <TransitionPreviewModal
      transition={transition}
      canApply
      canEdit
      onClose={onClose}
      onBackToEditor={onBackToEditor}
      onApplied={onApplied}
    />,
  );
  return { ...view, onClose, onBackToEditor, onApplied };
}

/** Applies through the confirmation; the panel hides meanwhile. */
async function applyThroughConfirmation() {
  const apply = await screen.findByRole("button", {
    name: "Schuljahreswechsel anwenden",
  });
  await waitFor(() => expect(apply).toBeEnabled());
  fireEvent.click(apply);
  fireEvent.click(await screen.findByRole("button", { name: "Ja, anwenden" }));
}

describe("TransitionPreviewModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.previewGradeTransition.mockResolvedValue(preview);
  });

  it("locks the panel while Wiederholen applies again and unlocks it after a failure", async () => {
    const { onClose, onBackToEditor, onApplied } = renderPreview();
    api.applyGradeTransition.mockRejectedValueOnce(serverError());
    await applyThroughConfirmation();

    // Die Bestätigung schließt mit dem Fehler; das Panel zeigt ihn.
    expect(await screen.findByText(APPLY_ERROR)).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);

    const retry = deferred<TransitionResult>();
    api.applyGradeTransition.mockReturnValueOnce(retry.promise);
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expectSlideOverSaveLocked(TITLE, FOOTER);
    expect(onClose).not.toHaveBeenCalled();
    expect(onBackToEditor).not.toHaveBeenCalled();

    await act(async () => {
      retry.reject(serverError());
    });

    expect(await screen.findByText(APPLY_ERROR)).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);
    expect(onApplied).not.toHaveBeenCalled();
    expect(api.applyGradeTransition).toHaveBeenLastCalledWith("8", "abc");
  });

  it("hands the result on after the confirmation applied it", async () => {
    const { onApplied } = renderPreview();
    const answer = deferred<TransitionResult>();
    api.applyGradeTransition.mockReturnValueOnce(answer.promise);
    await applyThroughConfirmation();

    // Ausgeblendet, solange die Bestätigung anwendet (#2774).
    // Anker: die Bestätigung ist offen.
    expect(
      screen.getByRole("heading", { name: "Wirklich anwenden?" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: TITLE })).toBeNull();

    await act(async () => {
      answer.resolve(result);
    });

    expect(onApplied).toHaveBeenCalledWith(result);
  });

  it("closes nothing when the answer arrives after unmount", async () => {
    const { onApplied, onClose, unmount } = renderPreview();
    const answer = deferred<TransitionResult>();
    api.applyGradeTransition.mockReturnValueOnce(answer.promise);
    await applyThroughConfirmation();
    expect(api.applyGradeTransition).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      answer.resolve(result);
    });

    expect(onApplied).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });
});
