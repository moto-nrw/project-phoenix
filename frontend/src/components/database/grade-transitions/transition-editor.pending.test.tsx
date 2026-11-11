// #6593: Während der Entwurf gespeichert wird, ist das Panel gesperrt; eine
// späte Antwort wechselt keinen geschlossenen Editor zur Vorschau. Echter
// SlideOver über dem gemeinsamen Vaul-Mock.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { GradeTransition } from "~/lib/grade-transition-api";
import { catalogText } from "~/test/error-catalog-text";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { api } = vi.hoisted(() => ({
  api: {
    fetchTransitionClasses: vi.fn(),
    fetchSuggestedMappings: vi.fn(),
    updateGradeTransition: vi.fn(),
    createGradeTransition: vi.fn(),
  },
}));
vi.mock("~/lib/grade-transition-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/grade-transition-api")>()),
  ...api,
}));

import { TransitionEditor } from "./transition-editor";

const TITLE = "Schuljahreswechsel bearbeiten";
const FOOTER = ["Abbrechen", "Speichern …"];

const draft: GradeTransition = {
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

function renderAndSave() {
  const answer = deferred<GradeTransition>();
  api.updateGradeTransition.mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const onReadyForPreview = vi.fn();
  const view = render(
    <TransitionEditor
      existingDraft={draft}
      onClose={onClose}
      onReadyForPreview={onReadyForPreview}
    />,
  );
  fireEvent.change(screen.getByLabelText("Notiz (optional)"), {
    target: { value: "Sommer" },
  });
  const save = screen.getByRole("button", { name: "Weiter zur Vorschau" });
  expect(save).toBeEnabled();
  fireEvent.click(save);
  return { ...view, answer, onClose, onReadyForPreview };
}

describe("TransitionEditor pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.fetchTransitionClasses.mockResolvedValue(["1a"]);
  });

  it("locks the panel while saving and unlocks it after a failure", async () => {
    const { answer, onClose, onReadyForPreview } = renderAndSave();

    expectSlideOverSaveLocked(TITLE, FOOTER);
    expect(onClose).not.toHaveBeenCalled();

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(
      await screen.findByText(
        catalogText("general.server", "das Speichern des Entwurfs"),
      ),
    ).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByLabelText("Notiz (optional)")).toHaveValue("Sommer");
    expect(onReadyForPreview).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("hands the saved draft to the preview", async () => {
    const { answer, onReadyForPreview } = renderAndSave();

    await act(async () => {
      answer.resolve({ ...draft, notes: "Sommer" });
    });

    expect(onReadyForPreview).toHaveBeenCalledWith(
      expect.objectContaining({ id: "8", notes: "Sommer" }),
    );
  });

  it("does not switch to the preview when the answer arrives after unmount", async () => {
    const { answer, onReadyForPreview, unmount } = renderAndSave();
    expect(api.updateGradeTransition).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      answer.resolve(draft);
    });

    expect(onReadyForPreview).not.toHaveBeenCalled();
  });
});
