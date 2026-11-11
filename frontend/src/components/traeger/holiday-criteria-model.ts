/**
 * The editable list of the Träger's criteria (#4700): order is priority,
 * the templates a person can click, and the checks before saving. Field
 * names match the backend's `errors[].field` (`criteria.N.name`), so a
 * refusal lands at the same control as a local check.
 */

import {
  type HolidayCriterion,
  type HolidayCriterionInput,
  type HolidayCriterionScope,
  MAX_CRITERION_NAME,
  MAX_CRITERION_QUESTION,
  MAX_HOLIDAY_CRITERIA,
} from "~/lib/traeger-holiday-allocation-api";

export interface CriterionDraft {
  /** Stable React key; never sent. */
  readonly key: string;
  /** Empty for a new criterion. */
  readonly id: string;
  readonly name: string;
  readonly question: string;
  readonly scope: HolidayCriterionScope;
  readonly active: boolean;
  /** Parents answered it already: its scope stays. */
  readonly answered: boolean;
}

/**
 * Suggestions to start from; nothing is fixed. Only yes/no questions the
 * allocation needs: nothing on health or special needs.
 */
export const CRITERION_TEMPLATES: readonly Omit<
  CriterionDraft,
  "key" | "id" | "active" | "answered"
>[] = [
  {
    name: "Beide Eltern berufstätig",
    question:
      "Sind beide Eltern berufstätig oder ist die alleinerziehende Person berufstätig?",
    scope: "family",
  },
  {
    name: "Alleinerziehend und berufstätig",
    question: "Sind Sie alleinerziehend und berufstätig?",
    scope: "family",
  },
  {
    name: "Geschwisterkind ist schon angemeldet",
    question:
      "Ist ein Geschwisterkind schon für diese Ferienbetreuung angemeldet?",
    scope: "child",
  },
  {
    name: "Besonderer Härtefall",
    question:
      "Gibt es einen besonderen Härtefall in Ihrer Familie? Das Büro fragt dann bei Ihnen nach.",
    scope: "family",
  },
];

let keySequence = 0;
function nextKey(): string {
  keySequence += 1;
  return `criterion-${keySequence}`;
}

export function draftsFromCriteria(
  criteria: readonly HolidayCriterion[],
): CriterionDraft[] {
  return [...criteria]
    .sort((a, b) => a.position - b.position)
    .map((criterion) => ({
      key: nextKey(),
      id: criterion.id,
      name: criterion.name,
      question: criterion.question,
      scope: criterion.scope,
      active: criterion.active,
      answered: criterion.answered,
    }));
}

export function emptyCriterion(): CriterionDraft {
  return {
    key: nextKey(),
    id: "",
    name: "",
    question: "",
    scope: "family",
    active: true,
    answered: false,
  };
}

/** A template is used when a criterion with its name is in the list. */
export function templateUsed(
  drafts: readonly CriterionDraft[],
  name: string,
): boolean {
  const wanted = name.trim().toLowerCase();
  return drafts.some((draft) => draft.name.trim().toLowerCase() === wanted);
}

export function fromTemplate(
  template: (typeof CRITERION_TEMPLATES)[number],
): CriterionDraft {
  return { ...emptyCriterion(), ...template };
}

/** Moves one criterion up (-1) or down (+1); outside the list nothing moves. */
export function moveCriterion(
  drafts: readonly CriterionDraft[],
  index: number,
  delta: -1 | 1,
): CriterionDraft[] {
  const target = index + delta;
  if (index < 0 || index >= drafts.length) return [...drafts];
  if (target < 0 || target >= drafts.length) return [...drafts];
  const next = [...drafts];
  const [moved] = next.splice(index, 1);
  next.splice(target, 0, moved!);
  return next;
}

/** The checks before saving, keyed like the backend's field paths. */
export function validateCriteria(
  drafts: readonly CriterionDraft[],
): Record<string, string> {
  const errors: Record<string, string> = {};
  if (drafts.length > MAX_HOLIDAY_CRITERIA) {
    errors.criteria = `Es gehen höchstens ${MAX_HOLIDAY_CRITERIA} Kriterien.`;
  }
  const names = new Set<string>();
  drafts.forEach((draft, index) => {
    const name = draft.name.trim();
    if (!name) {
      errors[`criteria.${index}.name`] = "Bitte geben Sie einen Namen ein.";
    } else if (name.length > MAX_CRITERION_NAME) {
      errors[`criteria.${index}.name`] =
        `Der Name hat höchstens ${MAX_CRITERION_NAME} Zeichen.`;
    } else if (names.has(name.toLowerCase())) {
      errors[`criteria.${index}.name`] = "Diesen Namen gibt es schon.";
    }
    names.add(name.toLowerCase());
    const question = draft.question.trim();
    if (!question) {
      errors[`criteria.${index}.question`] =
        "Bitte schreiben Sie die Frage an die Eltern.";
    } else if (question.length > MAX_CRITERION_QUESTION) {
      errors[`criteria.${index}.question`] =
        `Die Frage hat höchstens ${MAX_CRITERION_QUESTION} Zeichen.`;
    }
  });
  return errors;
}

/** The request body; its order is the priority. */
export function inputsFromDrafts(
  drafts: readonly CriterionDraft[],
): HolidayCriterionInput[] {
  return drafts.map((draft) => ({
    id: draft.id,
    name: draft.name.trim(),
    question: draft.question.trim(),
    scope: draft.scope,
    active: draft.active,
  }));
}

/** Whether the list differs from the stored criteria. */
export function criteriaChanged(
  drafts: readonly CriterionDraft[],
  stored: readonly HolidayCriterion[],
): boolean {
  const saved = inputsFromDrafts(draftsFromCriteria(stored));
  const current = inputsFromDrafts(drafts);
  return JSON.stringify(saved) !== JSON.stringify(current);
}
