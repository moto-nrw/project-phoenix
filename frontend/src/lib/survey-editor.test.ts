import { describe, expect, it } from "vitest";

import { TEST_CLOCK_INSTANT } from "~/test/clock";
import {
  changeQuestionKind,
  emptySurveyDraft,
  hasEditorErrors,
  moveItem,
  newQuestionDraft,
  surveyInputFromDraft,
  validateSurveyDraft,
  type SurveyEditorDraft,
} from "./survey-editor";

const now = new Date(TEST_CLOCK_INSTANT);

function validDraft(patch: Partial<SurveyEditorDraft> = {}): SurveyEditorDraft {
  const base = emptySurveyDraft();
  return {
    ...base,
    title: " Zufriedenheit ",
    closesDay: new Date(2026, 8, 30),
    closesTime: "18:00",
    targets: [{ target_type: "school_all" }],
    questions: [
      { ...newQuestionDraft("scale"), prompt: "Mein Kind ist gern hier." },
    ],
    ...patch,
  };
}

describe("validateSurveyDraft", () => {
  it("accepts a complete parent survey", () => {
    expect(hasEditorErrors(validateSurveyDraft(validDraft(), now))).toBe(false);
  });

  it("needs a title, a future closing time and parent targets", () => {
    const errors = validateSurveyDraft(
      validDraft({
        title: "  ",
        closesDay: new Date(2026, 8, 1),
        targets: [],
      }),
      now,
    );

    expect(errors.title).toBe("Bitte geben Sie einen Titel ein.");
    expect(errors.closesAt).toBe("Der Zeitpunkt muss in der Zukunft liegen.");
    expect(errors.targets).toBe(
      "Bitte wählen Sie, welche Eltern antworten sollen.",
    );
  });

  it("rejects a closing time more than a year ahead", () => {
    const errors = validateSurveyDraft(
      validDraft({ closesDay: new Date(2027, 9, 1) }),
      now,
    );
    expect(errors.closesAt).toBe(
      "Der Zeitpunkt darf höchstens ein Jahr entfernt sein.",
    );
  });

  it("needs no targets for a staff survey", () => {
    const errors = validateSurveyDraft(
      validDraft({ audience: "staff", targets: [] }),
      now,
    );
    expect(hasEditorErrors(errors)).toBe(false);
  });

  it("needs two distinct filled options for a choice question", () => {
    const question = {
      ...newQuestionDraft("single"),
      prompt: "Wie?",
      options: ["Bus", "bus"],
    };
    const errors = validateSurveyDraft(
      validDraft({ questions: [question] }),
      now,
    );

    expect(errors.perQuestion[question.key]?.options).toBe(
      "Jede Antwort darf nur einmal vorkommen.",
    );
  });
});

describe("surveyInputFromDraft", () => {
  it("drops targets and e-mail for staff and trims the texts", () => {
    const input = surveyInputFromDraft(
      validDraft({ audience: "staff", sendEmail: true }),
    );

    expect(input).toMatchObject({
      title: "Zufriedenheit",
      audience: "staff",
      anonymous: true,
      send_email: false,
      targets: [],
      questions: [
        { kind: "scale", prompt: "Mein Kind ist gern hier.", required: true },
      ],
    });
    // 18:00 Berlin (Sommerzeit) ist 16:00 UTC.
    expect(input.closes_at).toBe("2026-09-30T16:00:00.000Z");
  });

  it("keeps options only on choice questions", () => {
    const choice = changeQuestionKind(newQuestionDraft("text"), "multiple");
    expect(choice.options).toEqual(["", ""]);
    expect(changeQuestionKind(choice, "scale").options).toEqual([]);
  });
});

describe("moveItem", () => {
  it("moves within bounds and ignores moves outside", () => {
    expect(moveItem(["a", "b", "c"], 0, 2)).toEqual(["b", "c", "a"]);
    expect(moveItem(["a", "b"], 0, -1)).toEqual(["a", "b"]);
  });
});
