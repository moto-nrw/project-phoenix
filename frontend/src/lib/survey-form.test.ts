import { describe, expect, it } from "vitest";

import {
  buildSurveyAnswers,
  missingRequiredQuestions,
  toggleSurveyOption,
  withSurveyScale,
  withSurveyText,
  type SurveyDraft,
  type SurveyQuestion,
} from "./survey-form";

const single: SurveyQuestion = {
  id: "q1",
  position: 1,
  kind: "single",
  prompt: "Wie kommt Ihr Kind?",
  required: true,
  options: [
    { id: "o1", label: "Zu Fuß" },
    { id: "o2", label: "Mit dem Bus" },
  ],
};
const multiple: SurveyQuestion = {
  ...single,
  id: "q2",
  position: 2,
  kind: "multiple",
  required: false,
};
const scale: SurveyQuestion = {
  id: "q3",
  position: 3,
  kind: "scale",
  prompt: "Mein Kind fühlt sich wohl.",
  required: true,
  options: [],
};
const text: SurveyQuestion = {
  id: "q4",
  position: 4,
  kind: "text",
  prompt: "Was sollen wir beibehalten?",
  required: false,
  options: [],
};
const questions = [single, multiple, scale, text];

describe("survey answer draft", () => {
  it("replaces the choice of a single question and toggles a multiple one", () => {
    let draft: SurveyDraft = {};
    draft = toggleSurveyOption(draft, single, "o1");
    draft = toggleSurveyOption(draft, single, "o2");
    draft = toggleSurveyOption(draft, multiple, "o1");
    draft = toggleSurveyOption(draft, multiple, "o2");
    draft = toggleSurveyOption(draft, multiple, "o1");

    expect(draft.q1?.optionIds).toEqual(["o2"]);
    expect(draft.q2?.optionIds).toEqual(["o2"]);
  });

  it("names the required questions that still lack an answer", () => {
    const draft = withSurveyText(
      toggleSurveyOption({}, single, "o1"),
      "q4",
      "  ",
    );

    expect(missingRequiredQuestions(questions, draft).map((q) => q.id)).toEqual(
      ["q3"],
    );
    expect(
      missingRequiredQuestions(questions, withSurveyScale(draft, "q3", 4)),
    ).toEqual([]);
  });

  it("sends only answered questions, trimmed and limited to known options", () => {
    let draft: SurveyDraft = toggleSurveyOption({}, single, "o1");
    draft = toggleSurveyOption(draft, multiple, "unbekannt");
    draft = toggleSurveyOption(draft, multiple, "o2");
    draft = withSurveyScale(draft, "q3", 5);
    draft = withSurveyText(draft, "q4", "  Die AGs  ");

    expect(buildSurveyAnswers(questions, draft)).toEqual({
      answers: [
        { question_id: "q1", option_ids: ["o1"] },
        { question_id: "q2", option_ids: ["o2"] },
        { question_id: "q3", scale_value: 5 },
        { question_id: "q4", text: "Die AGs" },
      ],
    });
  });

  it("leaves out an optional question nobody answered", () => {
    const draft = withSurveyScale(
      toggleSurveyOption({}, single, "o2"),
      "q3",
      1,
    );

    expect(
      buildSurveyAnswers(questions, draft).answers.map((a) => a.question_id),
    ).toEqual(["q1", "q3"]);
  });
});
