import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { newQuestionDraft, type SurveyEditorDraft } from "~/lib/survey-editor";
import type { Survey } from "~/lib/surveys-api";

import {
  SURVEY_FORM_CHECK_MESSAGE,
  SurveyCreateSlideOver,
} from "./survey-create-slide-over";

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

function validDraft(title: string): SurveyEditorDraft {
  return {
    title,
    intro: "",
    audience: "staff",
    anonymous: true,
    sendEmail: false,
    closesDay: new Date(2026, 8, 30),
    closesTime: "18:00",
    targets: [],
    questions: [{ ...newQuestionDraft("scale"), prompt: "Wie geht es?" }],
  };
}

// Die Eingabefelder sind nicht Gegenstand: Jeder Knopf setzt den Entwurf,
// den die Person zuletzt sieht, über denselben `onChange` wie das Formular.
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
      <button type="button" onClick={() => onChange(validDraft("ALT"))}>
        Entwurf ALT
      </button>
      <button type="button" onClick={() => onChange(validDraft("NEU"))}>
        Entwurf NEU
      </button>
      <button type="button" onClick={() => onChange(validDraft(""))}>
        Titel leeren
      </button>
    </div>
  ),
}));

beforeEach(() => vi.clearAllMocks());

async function failFirstSaveOf(title: string) {
  createSurvey.mockRejectedValueOnce(
    new ApiError("boom", 500, { code: "general.server" }),
  );
  const onCreated = vi.fn();
  render(<SurveyCreateSlideOver onClose={vi.fn()} onCreated={onCreated} />);
  fireEvent.click(screen.getByRole("button", { name: `Entwurf ${title}` }));
  fireEvent.click(
    screen.getByRole("button", { name: "Als Entwurf speichern" }),
  );
  const retry = await screen.findByRole("button", { name: "Wiederholen" });
  expect(createSurvey).toHaveBeenCalledOnce();
  expect(createSurvey).toHaveBeenLastCalledWith(
    expect.objectContaining({ title }),
  );
  return { retry, onCreated };
}

it("„Wiederholen“ speichert den nach dem Fehler geänderten Entwurf (#6503)", async () => {
  const { retry, onCreated } = await failFirstSaveOf("ALT");
  const created = { id: "5", title: "NEU" } as Survey;
  createSurvey.mockResolvedValueOnce(created);

  fireEvent.click(screen.getByRole("button", { name: "Entwurf NEU" }));
  expect(screen.getByText("Sichtbarer Titel: NEU")).toBeInTheDocument();
  fireEvent.click(retry);

  await waitFor(() => expect(onCreated).toHaveBeenCalledWith(created));
  expect(createSurvey).toHaveBeenCalledTimes(2);
  expect(createSurvey).toHaveBeenLastCalledWith(
    expect.objectContaining({ title: "NEU" }),
  );
});

it("„Wiederholen“ prüft den geänderten Entwurf und sendet einen ungültigen nicht (#6503)", async () => {
  const { retry, onCreated } = await failFirstSaveOf("ALT");

  fireEvent.click(screen.getByRole("button", { name: "Titel leeren" }));
  fireEvent.click(retry);

  expect(
    await screen.findByText(SURVEY_FORM_CHECK_MESSAGE),
  ).toBeInTheDocument();
  expect(createSurvey).toHaveBeenCalledOnce();
  expect(onCreated).not.toHaveBeenCalled();
});
