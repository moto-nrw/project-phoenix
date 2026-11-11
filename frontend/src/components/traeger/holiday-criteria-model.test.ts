import { describe, expect, it } from "vitest";
import type { HolidayCriterion } from "~/lib/traeger-holiday-allocation-api";
import {
  CRITERION_TEMPLATES,
  criteriaChanged,
  draftsFromCriteria,
  emptyCriterion,
  fromTemplate,
  inputsFromDrafts,
  moveCriterion,
  templateUsed,
  validateCriteria,
} from "./holiday-criteria-model";

const STORED: HolidayCriterion[] = [
  {
    id: "2",
    name: "Geschwisterkind ist schon angemeldet",
    question: "Ist ein Geschwisterkind schon angemeldet?",
    scope: "child",
    active: true,
    position: 2,
    answered: false,
  },
  {
    id: "1",
    name: "Beide Eltern berufstätig",
    question: "Sind beide Eltern berufstätig?",
    scope: "family",
    active: true,
    position: 1,
    answered: true,
  },
];

describe("holiday criteria model (#4700)", () => {
  it("orders the drafts by priority and sends them in that order", () => {
    const drafts = draftsFromCriteria(STORED);
    expect(drafts.map((draft) => draft.id)).toEqual(["1", "2"]);
    expect(inputsFromDrafts(moveCriterion(drafts, 1, -1))).toEqual([
      {
        id: "2",
        name: "Geschwisterkind ist schon angemeldet",
        question: "Ist ein Geschwisterkind schon angemeldet?",
        scope: "child",
        active: true,
      },
      {
        id: "1",
        name: "Beide Eltern berufstätig",
        question: "Sind beide Eltern berufstätig?",
        scope: "family",
        active: true,
      },
    ]);
  });

  it("keeps the order when a move would leave the list", () => {
    const drafts = draftsFromCriteria(STORED);
    expect(moveCriterion(drafts, 0, -1)).toEqual(drafts);
    expect(moveCriterion(drafts, 1, 1)).toEqual(drafts);
  });

  it("names the fields like the backend and refuses duplicates and too many", () => {
    const drafts = [
      { ...emptyCriterion(), name: "Härtefall", question: "?" },
      { ...emptyCriterion(), name: " härtefall ", question: "" },
      emptyCriterion(),
    ];
    expect(validateCriteria(drafts)).toEqual({
      "criteria.1.name": "Diesen Namen gibt es schon.",
      "criteria.1.question": "Bitte schreiben Sie die Frage an die Eltern.",
      "criteria.2.name": "Bitte geben Sie einen Namen ein.",
      "criteria.2.question": "Bitte schreiben Sie die Frage an die Eltern.",
    });
    const many = Array.from({ length: 11 }, (_, index) => ({
      ...emptyCriterion(),
      name: `Kriterium ${index}`,
      question: "Trifft das zu?",
    }));
    expect(validateCriteria(many)).toEqual({
      criteria: "Es gehen höchstens 10 Kriterien.",
    });
  });

  it("offers templates without health or special needs and marks used ones", () => {
    const names = CRITERION_TEMPLATES.map((template) => template.name);
    expect(names).toEqual([
      "Beide Eltern berufstätig",
      "Alleinerziehend und berufstätig",
      "Geschwisterkind ist schon angemeldet",
      "Besonderer Härtefall",
    ]);
    expect(
      CRITERION_TEMPLATES.some((template) =>
        /gesundheit|förder|sonderpäd/i.test(
          `${template.name} ${template.question}`,
        ),
      ),
    ).toBe(false);
    const drafts = draftsFromCriteria(STORED);
    expect(templateUsed(drafts, "beide eltern berufstätig")).toBe(true);
    expect(templateUsed(drafts, "Besonderer Härtefall")).toBe(false);
    const fromSibling = fromTemplate(CRITERION_TEMPLATES[2]!);
    expect(fromSibling).toMatchObject({ id: "", scope: "child", active: true });
  });

  it("notices a change of order, text or switch", () => {
    const drafts = draftsFromCriteria(STORED);
    expect(criteriaChanged(drafts, STORED)).toBe(false);
    expect(criteriaChanged(moveCriterion(drafts, 0, 1), STORED)).toBe(true);
    expect(
      criteriaChanged(
        drafts.map((draft, index) =>
          index === 0 ? { ...draft, active: false } : draft,
        ),
        STORED,
      ),
    ).toBe(true);
  });
});
