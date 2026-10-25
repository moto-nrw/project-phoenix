/**
 * Der Entwurf einer Umfrage im Formular (#4781): Zustand, Prüfung und
 * Umwandlung in den Rumpf für `POST/PUT /api/surveys`. Rein, ohne React,
 * damit die Regeln einzeln getestet werden.
 */

import {
  berlinClockFromISO,
  berlinDateTimeISO,
  berlinDayFromISO,
} from "~/lib/date-helpers";
import type { SurveyQuestionKind } from "~/lib/survey-form";
import type {
  Survey,
  SurveyAudience,
  SurveyInput,
  SurveyTarget,
} from "~/lib/surveys-api";

export const SURVEY_MAX_QUESTIONS = 30;
const SURVEY_MIN_OPTIONS = 2;
export const SURVEY_MAX_OPTIONS = 10;
export const SURVEY_TITLE_MAX_LENGTH = 200;
export const SURVEY_PROMPT_MAX_LENGTH = 300;
export const SURVEY_OPTION_MAX_LENGTH = 120;
/** Teilnahme bis: höchstens ein Jahr in der Zukunft. */
const MAX_CLOSE_MS = 366 * 24 * 60 * 60 * 1000;
const SURVEY_DEFAULT_CLOSE_TIME = "18:00";

export const SURVEY_QUESTION_KIND_LABEL: Record<SurveyQuestionKind, string> = {
  single: "Einfachauswahl",
  multiple: "Mehrfachauswahl",
  scale: "Skala 1 bis 5",
  text: "Freitext",
};

export interface SurveyQuestionDraft {
  /** Nur für React: ein Fragenentwurf hat vor dem Speichern keine ID. */
  readonly key: string;
  readonly kind: SurveyQuestionKind;
  readonly prompt: string;
  readonly required: boolean;
  readonly options: readonly string[];
}

export interface SurveyEditorDraft {
  readonly title: string;
  readonly intro: string;
  readonly audience: SurveyAudience;
  readonly anonymous: boolean;
  readonly sendEmail: boolean;
  readonly closesDay: Date | null;
  readonly closesTime: string;
  readonly targets: readonly SurveyTarget[];
  readonly questions: readonly SurveyQuestionDraft[];
}

let questionKeyCounter = 0;
function nextQuestionKey(): string {
  questionKeyCounter += 1;
  return `frage-${questionKeyCounter}`;
}

export function newQuestionDraft(
  kind: SurveyQuestionKind = "scale",
): SurveyQuestionDraft {
  return {
    key: nextQuestionKey(),
    kind,
    prompt: "",
    required: true,
    options: kind === "single" || kind === "multiple" ? ["", ""] : [],
  };
}

export function emptySurveyDraft(): SurveyEditorDraft {
  return {
    title: "",
    intro: "",
    audience: "parents",
    anonymous: true,
    sendEmail: false,
    closesDay: null,
    closesTime: SURVEY_DEFAULT_CLOSE_TIME,
    targets: [],
    questions: [newQuestionDraft("scale")],
  };
}

export function surveyDraftFromSurvey(survey: Survey): SurveyEditorDraft {
  return {
    title: survey.title,
    intro: survey.intro,
    audience: survey.audience,
    anonymous: survey.anonymous,
    sendEmail: survey.send_email,
    closesDay: survey.closes_at ? berlinDayFromISO(survey.closes_at) : null,
    closesTime: survey.closes_at
      ? berlinClockFromISO(survey.closes_at) || SURVEY_DEFAULT_CLOSE_TIME
      : SURVEY_DEFAULT_CLOSE_TIME,
    targets: survey.targets,
    questions: [...survey.questions]
      .sort((a, b) => a.position - b.position)
      .map((question) => ({
        key: nextQuestionKey(),
        kind: question.kind,
        prompt: question.prompt,
        required: question.required,
        options: question.options.map((option) => option.label),
      })),
  };
}

/** Wechselt die Art; Auswahlfragen behalten oder bekommen zwei Felder. */
export function changeQuestionKind(
  question: SurveyQuestionDraft,
  kind: SurveyQuestionKind,
): SurveyQuestionDraft {
  const needsOptions = kind === "single" || kind === "multiple";
  const options = needsOptions
    ? question.options.length >= SURVEY_MIN_OPTIONS
      ? question.options
      : [...question.options, "", ""].slice(0, SURVEY_MIN_OPTIONS)
    : [];
  return { ...question, kind, options };
}

export function moveItem<T>(
  items: readonly T[],
  from: number,
  to: number,
): T[] {
  if (to < 0 || to >= items.length || from === to) return [...items];
  const next = [...items];
  const [moved] = next.splice(from, 1);
  if (moved !== undefined) next.splice(to, 0, moved);
  return next;
}

interface SurveyQuestionErrors {
  prompt?: string;
  options?: string;
}

export interface SurveyEditorErrors {
  title?: string;
  closesAt?: string;
  targets?: string;
  questions?: string;
  /** Je Frage, Schlüssel = `SurveyQuestionDraft.key`. */
  perQuestion: Readonly<Record<string, SurveyQuestionErrors>>;
}

export function hasEditorErrors(errors: SurveyEditorErrors): boolean {
  return Boolean(
    errors.title ??
    errors.closesAt ??
    errors.targets ??
    errors.questions ??
    Object.keys(errors.perQuestion).length > 0,
  );
}

function isValidTime(value: string): boolean {
  return /^([01]\d|2[0-3]):[0-5]\d$/.test(value);
}

/** Der Zeitpunkt „Teilnahme bis“ als RFC 3339, oder null ohne gültige Angabe. */
function draftClosesAt(draft: SurveyEditorDraft): string | null {
  if (!draft.closesDay || !isValidTime(draft.closesTime)) return null;
  return berlinDateTimeISO(draft.closesDay, draft.closesTime);
}

/** Prüft wie das Backend, damit das Formular den Fehler am Feld zeigt. */
export function validateSurveyDraft(
  draft: SurveyEditorDraft,
  now: Date = new Date(),
): SurveyEditorErrors {
  const errors: {
    -readonly [K in keyof SurveyEditorErrors]: SurveyEditorErrors[K];
  } & { perQuestion: Record<string, SurveyQuestionErrors> } = {
    perQuestion: {},
  };

  if (!draft.title.trim()) errors.title = "Bitte geben Sie einen Titel ein.";

  const closesAt = draftClosesAt(draft);
  if (!draft.closesDay) {
    errors.closesAt = "Bitte wählen Sie, bis wann man teilnehmen kann.";
  } else if (!closesAt) {
    errors.closesAt = "Bitte geben Sie die Uhrzeit im Format 18:00 ein.";
  } else {
    const instant = new Date(closesAt).getTime();
    if (instant <= now.getTime()) {
      errors.closesAt = "Der Zeitpunkt muss in der Zukunft liegen.";
    } else if (instant - now.getTime() > MAX_CLOSE_MS) {
      errors.closesAt = "Der Zeitpunkt darf höchstens ein Jahr entfernt sein.";
    }
  }

  if (draft.audience === "parents" && draft.targets.length === 0) {
    errors.targets = "Bitte wählen Sie, welche Eltern antworten sollen.";
  }

  if (draft.questions.length === 0) {
    errors.questions = "Bitte legen Sie mindestens eine Frage an.";
  } else if (draft.questions.length > SURVEY_MAX_QUESTIONS) {
    errors.questions = `Höchstens ${SURVEY_MAX_QUESTIONS} Fragen sind möglich.`;
  }

  for (const question of draft.questions) {
    const own: SurveyQuestionErrors = {};
    if (!question.prompt.trim()) own.prompt = "Bitte geben Sie die Frage ein.";
    if (question.kind === "single" || question.kind === "multiple") {
      const labels = question.options.map((option) => option.trim());
      const filled = labels.filter(Boolean);
      if (filled.length < SURVEY_MIN_OPTIONS) {
        own.options = `Bitte geben Sie mindestens ${SURVEY_MIN_OPTIONS} Antworten ein.`;
      } else if (filled.length > SURVEY_MAX_OPTIONS) {
        own.options = `Höchstens ${SURVEY_MAX_OPTIONS} Antworten sind möglich.`;
      } else if (filled.length !== labels.length) {
        own.options =
          "Bitte füllen Sie jede Antwort aus oder entfernen Sie sie.";
      } else if (
        new Set(filled.map((label) => label.toLowerCase())).size !==
        filled.length
      ) {
        own.options = "Jede Antwort darf nur einmal vorkommen.";
      }
    }
    if (own.prompt ?? own.options) errors.perQuestion[question.key] = own;
  }

  return errors;
}

/** Der Rumpf für das Backend. Vorher `validateSurveyDraft` prüfen. */
export function surveyInputFromDraft(draft: SurveyEditorDraft): SurveyInput {
  const parents = draft.audience === "parents";
  return {
    title: draft.title.trim(),
    intro: draft.intro.trim(),
    audience: draft.audience,
    anonymous: draft.anonymous,
    send_email: parents && draft.sendEmail,
    closes_at: draftClosesAt(draft) ?? "",
    targets: parents ? [...draft.targets] : [],
    questions: draft.questions.map((question) => {
      const choice = question.kind === "single" || question.kind === "multiple";
      return {
        kind: question.kind,
        prompt: question.prompt.trim(),
        required: question.required,
        ...(choice
          ? { options: question.options.map((option) => option.trim()) }
          : {}),
      };
    }),
  };
}
