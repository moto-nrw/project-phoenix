/**
 * Der Fragebogen einer Umfrage (#4781), wie ihn das Mitarbeiterportal und das
 * Elternportal gleichermaßen beantworten. Hier stehen nur Typen und reine
 * Hilfen: der Entwurf der Antworten lebt im Zustand des Formulars und
 * verlässt den Browser nur als Antwort an die eigene Schule. Er gehört in
 * kein Log, kein Analyseereignis und keine Adresse.
 */

export type SurveyQuestionKind = "single" | "multiple" | "scale" | "text";

interface SurveyOption {
  readonly id: string;
  readonly label: string;
}

export interface SurveyQuestion {
  readonly id: string;
  readonly position: number;
  readonly kind: SurveyQuestionKind;
  readonly prompt: string;
  readonly required: boolean;
  readonly options: readonly SurveyOption[];
}

export interface SurveyForm {
  readonly survey: {
    readonly id: string;
    readonly title: string;
    readonly intro: string;
    readonly anonymous: boolean;
    readonly closes_at: string;
    readonly questions: readonly SurveyQuestion[];
  };
  readonly school_name: string;
  readonly participated: boolean;
}

interface SurveyAnswer {
  readonly question_id: string;
  readonly option_ids?: string[];
  readonly scale_value?: number;
  readonly text?: string;
}

export interface SurveyAnswersRequest {
  readonly answers: SurveyAnswer[];
}

/** Die Skala ist fest 1 bis 5. */
export const SURVEY_SCALE_VALUES = [1, 2, 3, 4, 5] as const;

/** Höchstlänge einer Freitextantwort im Formular. */
export const SURVEY_TEXT_MAX_LENGTH = 2000;

/** Was auf dem Schirm gewählt ist, je Frage. */
export interface SurveyDraftAnswer {
  readonly optionIds: readonly string[];
  readonly scaleValue: number | null;
  readonly text: string;
}

export type SurveyDraft = Readonly<Record<string, SurveyDraftAnswer>>;

const EMPTY_ANSWER: SurveyDraftAnswer = {
  optionIds: [],
  scaleValue: null,
  text: "",
};

export function draftAnswer(
  draft: SurveyDraft,
  questionId: string,
): SurveyDraftAnswer {
  return draft[questionId] ?? EMPTY_ANSWER;
}

/** Wählt eine Option: Einfachauswahl ersetzt, Mehrfachauswahl schaltet um. */
export function toggleSurveyOption(
  draft: SurveyDraft,
  question: SurveyQuestion,
  optionId: string,
): SurveyDraft {
  const current = draftAnswer(draft, question.id);
  const optionIds =
    question.kind === "multiple"
      ? current.optionIds.includes(optionId)
        ? current.optionIds.filter((id) => id !== optionId)
        : [...current.optionIds, optionId]
      : [optionId];
  return { ...draft, [question.id]: { ...current, optionIds } };
}

export function withSurveyScale(
  draft: SurveyDraft,
  questionId: string,
  value: number,
): SurveyDraft {
  const current = draftAnswer(draft, questionId);
  return { ...draft, [questionId]: { ...current, scaleValue: value } };
}

export function withSurveyText(
  draft: SurveyDraft,
  questionId: string,
  text: string,
): SurveyDraft {
  const current = draftAnswer(draft, questionId);
  return { ...draft, [questionId]: { ...current, text } };
}

function isAnswered(
  question: SurveyQuestion,
  answer: SurveyDraftAnswer,
): boolean {
  switch (question.kind) {
    case "single":
    case "multiple":
      return answer.optionIds.length > 0;
    case "scale":
      return answer.scaleValue !== null;
    case "text":
      return answer.text.trim().length > 0;
  }
}

/** Die Pflichtfragen, die noch keine Antwort haben, in Reihenfolge. */
export function missingRequiredQuestions(
  questions: readonly SurveyQuestion[],
  draft: SurveyDraft,
): SurveyQuestion[] {
  return questions.filter(
    (question) =>
      question.required &&
      !isAnswered(question, draftAnswer(draft, question.id)),
  );
}

/**
 * Der Rumpf für `POST …/answers`. Unbeantwortete freiwillige Fragen fehlen
 * ganz, damit eine leere Antwort nicht als „bewusst nichts gewählt“ zählt.
 * Optionen, die es im Fragebogen nicht gibt, fallen weg.
 */
export function buildSurveyAnswers(
  questions: readonly SurveyQuestion[],
  draft: SurveyDraft,
): SurveyAnswersRequest {
  const answers: SurveyAnswer[] = [];
  for (const question of questions) {
    const answer = draftAnswer(draft, question.id);
    if (!isAnswered(question, answer)) continue;
    switch (question.kind) {
      case "single":
      case "multiple": {
        const known = new Set(question.options.map((option) => option.id));
        const optionIds = answer.optionIds.filter((id) => known.has(id));
        answers.push({
          question_id: question.id,
          option_ids:
            question.kind === "single" ? optionIds.slice(0, 1) : optionIds,
        });
        break;
      }
      case "scale":
        answers.push({
          question_id: question.id,
          scale_value: answer.scaleValue ?? undefined,
        });
        break;
      case "text":
        answers.push({
          question_id: question.id,
          text: answer.text.trim().slice(0, SURVEY_TEXT_MAX_LENGTH),
        });
        break;
    }
  }
  return { answers };
}

/** Fragen in der Reihenfolge, die die Schule festgelegt hat. */
export function sortedSurveyQuestions(
  questions: readonly SurveyQuestion[],
): SurveyQuestion[] {
  return [...questions].sort((a, b) => a.position - b.position);
}
