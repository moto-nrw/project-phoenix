// #6593: Während der Entwurf angelegt wird, lässt sich das Panel nicht
// schließen; eine späte Antwort öffnet keine Umfrage mehr.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { newQuestionDraft, type SurveyEditorDraft } from "~/lib/survey-editor";
import type { Survey } from "~/lib/surveys-api";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { createSurvey } = vi.hoisted(() => ({ createSurvey: vi.fn() }));
vi.mock("~/lib/surveys-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/surveys-api")>()),
  createSurvey,
}));

vi.mock("~/components/announcements/parent-target-picker", () => ({
  useParentTargetLookups: () => ({
    groups: [],
    activities: [],
    schoolClasses: [],
    lookupLoadError: null,
  }),
}));

// Wie in survey-create-slide-over.test.tsx: die Felder sind nicht
// Gegenstand, ein Knopf setzt einen gültigen Entwurf.
vi.mock("./survey-editor-fields", () => ({
  SurveyEditorFields: ({
    draft,
    onChange,
  }: {
    draft: SurveyEditorDraft;
    onChange: (next: SurveyEditorDraft) => void;
  }) => (
    <div>
      <p>Sichtbarer Titel: {draft.title || "(leer)"}</p>
      <button
        type="button"
        onClick={() =>
          onChange({
            title: "Elternabend",
            intro: "",
            audience: "staff",
            anonymous: true,
            sendEmail: false,
            closesDay: new Date(2026, 8, 30),
            closesTime: "18:00",
            targets: [],
            questions: [
              { ...newQuestionDraft("scale"), prompt: "Wie geht es?" },
            ],
          })
        }
      >
        Entwurf füllen
      </button>
    </div>
  ),
}));

import { SurveyCreateSlideOver } from "./survey-create-slide-over";

const TITLE = "Neue Umfrage";
const FOOTER = ["Abbrechen", "Wird gespeichert…"];

async function startSaving() {
  const answer = deferred<Survey>();
  createSurvey.mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const onCreated = vi.fn();
  const view = render(
    <SurveyCreateSlideOver onClose={onClose} onCreated={onCreated} />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Entwurf füllen" }));
  fireEvent.click(
    screen.getByRole("button", { name: "Als Entwurf speichern" }),
  );
  await screen.findByRole("button", { name: "Wird gespeichert…" });
  return { ...view, answer, onClose, onCreated };
}

describe("SurveyCreateSlideOver: Speichern sperrt das Panel (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("locks the panel while saving and unlocks it with the error after a failure", async () => {
    const { answer, onClose, onCreated } = await startSaving();

    expectSlideOverSaveLocked(TITLE, FOOTER);
    expect(onClose).not.toHaveBeenCalled();

    await act(async () => {
      answer.reject(new ApiError("boom", 500, { code: "general.server" }));
    });

    expect(
      await screen.findByRole("button", { name: "Wiederholen" }),
    ).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByText("Sichtbarer Titel: Elternabend")).toBeVisible();
    expect(onClose).not.toHaveBeenCalled();
    expect(onCreated).not.toHaveBeenCalled();
  });

  it("hands the created survey to the page", async () => {
    const { answer, onCreated } = await startSaving();
    const created = { id: "5", title: "Elternabend" } as Survey;

    await act(async () => {
      answer.resolve(created);
    });

    expect(onCreated).toHaveBeenCalledWith(created);
  });

  it("opens no survey when the answer arrives after unmount", async () => {
    const { answer, onCreated, unmount } = await startSaving();
    expect(createSurvey).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      answer.resolve({ id: "5", title: "Elternabend" } as Survey);
    });

    expect(onCreated).not.toHaveBeenCalled();
  });
});
