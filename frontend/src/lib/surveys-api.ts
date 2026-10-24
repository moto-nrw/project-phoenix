/**
 * Client des Mitarbeiterportals für Umfragen (#4781). Spricht mit den
 * Next.js-Routen unter /api/surveys, die mit dem Mitarbeiter-JWT an das
 * Backend weiterreichen. IDs kommen schon als Strings.
 *
 * Schutzziel Anonymität: Antworten gehen nur im Rumpf von
 * `submitSurveyAnswers` hinaus. Kein Aufruf hier schreibt sie in eine
 * Adresse, und die Fehlermeldungen tragen nie Antwortinhalte.
 */

import { ApiError, apiErrorFromText, transportFetch } from "~/lib/api-error";
import { downloadBlob, filenameFromDisposition } from "~/lib/file-download";
import type { AnnouncementTarget } from "~/lib/parent-announcements-api";
import type {
  SurveyAnswersRequest,
  SurveyForm,
  SurveyQuestion,
  SurveyQuestionKind,
} from "~/lib/survey-form";

export type SurveyAudience = "parents" | "staff";
export type SurveyStatus = "draft" | "open" | "closed";

/** Dieselbe Form wie die Zielgruppen der Elternmitteilungen. */
export type SurveyTarget = AnnouncementTarget;

interface SurveyQuestionInput {
  kind: SurveyQuestionKind;
  prompt: string;
  required: boolean;
  options?: string[];
}

export interface SurveyInput {
  title: string;
  intro: string;
  audience: SurveyAudience;
  anonymous: boolean;
  /** Nur bei Eltern: zusätzlich eine E-Mail beim Veröffentlichen. */
  send_email: boolean;
  /** Zeitpunkt (RFC 3339). */
  closes_at: string;
  /** Eltern: mindestens ein Ziel. Mitarbeitende: leer. */
  targets: SurveyTarget[];
  questions: SurveyQuestionInput[];
}

export interface Survey {
  id: string;
  title: string;
  intro: string;
  audience: SurveyAudience;
  anonymous: boolean;
  send_email: boolean;
  status: SurveyStatus;
  closes_at: string;
  published_at?: string;
  created_at: string;
  updated_at: string;
  participant_count: number;
  targets: SurveyTarget[];
  questions: SurveyQuestion[];
}

export interface SurveyQuestionResult {
  question_id: string;
  position: number;
  kind: SurveyQuestionKind;
  prompt: string;
  answer_count: number;
  options: { option_id: string; label: string; count: number }[];
  /** Fünf Werte, Index 0 = Wert 1. */
  scale_counts: number[];
  average: number | null;
  /** Ohne Zeit und Person, in zufälliger Reihenfolge. */
  texts: string[];
  /** Stärken: die Labels (bzw. "1" bis "5") mit den höchsten Werten. */
  highest: string[];
  /** Verbesserungsbedarf: die Labels mit den niedrigsten Werten. */
  lowest: string[];
}

export interface SurveyScaleSummary {
  question_id: string;
  prompt: string;
  average: number;
}

export interface SurveyResults {
  survey_id: string;
  title: string;
  anonymous: boolean;
  status: SurveyStatus;
  response_count: number;
  min_responses: number;
  /** false: noch zu wenige Antworten, `questions` ist dann leer. */
  available: boolean;
  questions: SurveyQuestionResult[];
  strengths: SurveyScaleSummary[];
  improvements: SurveyScaleSummary[];
}

/** Nur bei nicht anonymen Umfragen. */
export interface SurveyRespondent {
  name: string;
  answers: {
    question_id: string;
    options: string[];
    scale_value?: number;
    text?: string;
  }[];
}

export interface SurveyToAnswer {
  id: string;
  title: string;
  intro: string;
  anonymous: boolean;
  closes_at: string;
  participated: boolean;
}

interface ApiEnvelope<T> {
  data?: T;
}

const BASE = "/api/surveys";

async function throwApiError(
  response: Response,
  fallback: string,
): Promise<never> {
  let text = "";
  try {
    text = await response.text();
  } catch {
    // Body already consumed or the stream broke: the status still classifies.
  }
  throw apiErrorFromText(fallback, response.status, text);
}

async function request<T>(
  url: string,
  init: RequestInit | undefined,
  fallback: string,
): Promise<T | undefined> {
  const response = await transportFetch(url, init);
  if (!response.ok) await throwApiError(response, fallback);
  if (response.status === 204) return undefined;
  const body = (await response.json()) as ApiEnvelope<T>;
  return body.data;
}

async function requireData<T>(
  url: string,
  init: RequestInit | undefined,
  fallback: string,
): Promise<T> {
  const data = await request<T>(url, init, fallback);
  // Eine Erfolgsantwort ohne Rumpf: das Backend hat den Vertrag gebrochen.
  if (data === undefined) throw new ApiError(fallback, 500);
  return data;
}

function postJson(body: unknown, method = "POST"): RequestInit {
  return {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  };
}

function surveyPath(id: string, suffix = ""): string {
  return `${BASE}/${encodeURIComponent(id)}${suffix}`;
}

export async function fetchSurveys(): Promise<Survey[]> {
  const data = await request<Survey[]>(
    BASE,
    undefined,
    "Umfragen konnten nicht geladen werden",
  );
  return data ?? [];
}

export function fetchSurvey(id: string): Promise<Survey> {
  return requireData<Survey>(
    surveyPath(id),
    undefined,
    "Umfrage konnte nicht geladen werden",
  );
}

export function createSurvey(input: SurveyInput): Promise<Survey> {
  return requireData<Survey>(
    BASE,
    postJson(input),
    "Umfrage konnte nicht angelegt werden",
  );
}

export function updateSurvey(id: string, input: SurveyInput): Promise<Survey> {
  return requireData<Survey>(
    surveyPath(id),
    postJson(input, "PUT"),
    "Umfrage konnte nicht gespeichert werden",
  );
}

export async function deleteSurvey(id: string): Promise<void> {
  await request<unknown>(
    surveyPath(id),
    { method: "DELETE" },
    "Umfrage konnte nicht gelöscht werden",
  );
}

export function publishSurvey(id: string): Promise<Survey> {
  return requireData<Survey>(
    surveyPath(id, "/publish"),
    postJson({}),
    "Umfrage konnte nicht veröffentlicht werden",
  );
}

export function closeSurvey(id: string): Promise<Survey> {
  return requireData<Survey>(
    surveyPath(id, "/close"),
    postJson({}),
    "Umfrage konnte nicht beendet werden",
  );
}

/** Erinnert alle, die noch nicht teilgenommen haben. Liefert nur die Zahl. */
export async function remindSurvey(id: string): Promise<number> {
  const data = await request<{ reminded: number }>(
    surveyPath(id, "/remind"),
    postJson({}),
    "Erinnerung konnte nicht verschickt werden",
  );
  return data?.reminded ?? 0;
}

export function fetchSurveyResults(id: string): Promise<SurveyResults> {
  return requireData<SurveyResults>(
    surveyPath(id, "/results"),
    undefined,
    "Auswertung konnte nicht geladen werden",
  );
}

export async function fetchSurveyRespondents(
  id: string,
): Promise<SurveyRespondent[]> {
  const data = await request<SurveyRespondent[]>(
    surveyPath(id, "/respondents"),
    undefined,
    "Antworten konnten nicht geladen werden",
  );
  return data ?? [];
}

/**
 * Lädt die Auswertung als PDF unter dem Dateinamen des Backends. Erst
 * holen, dann speichern: ein Fehler bleibt als Meldung auf der Seite.
 */
export async function downloadSurveyResultsPdf(id: string): Promise<void> {
  const fallback = "Die Datei konnte nicht erstellt werden";
  const response = await transportFetch(surveyPath(id, "/results/export"));
  if (!response.ok) await throwApiError(response, fallback);
  const blob = await response.blob();
  downloadBlob(
    blob,
    filenameFromDisposition(response) ?? `umfrage-${id}-auswertung.pdf`,
  );
}

/* --- Beantworten (jede Person mit Mitarbeiter-Datensatz) ----------------- */

export async function fetchSurveysToAnswer(): Promise<SurveyToAnswer[]> {
  const data = await request<SurveyToAnswer[]>(
    `${BASE}/to-answer`,
    undefined,
    "Umfragen konnten nicht geladen werden",
  );
  return data ?? [];
}

export function fetchSurveyForm(id: string): Promise<SurveyForm> {
  return requireData<SurveyForm>(
    surveyPath(id, "/form"),
    undefined,
    "Umfrage konnte nicht geladen werden",
  );
}

export async function submitSurveyAnswers(
  id: string,
  body: SurveyAnswersRequest,
): Promise<void> {
  await request<{ submitted: boolean }>(
    surveyPath(id, "/answers"),
    postJson(body),
    "Antwort konnte nicht gespeichert werden",
  );
}
