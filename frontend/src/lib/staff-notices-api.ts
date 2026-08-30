/**
 * Client für die Tagesinformationen (#2180): interne Hinweise der Leitung an
 * das Team. Spricht die Next.js-Proxyrouten unter /api/staff-notices an, die
 * mit dem Personal-JWT ans Go-Backend weiterreichen. int64-Ids kommen bereits
 * als String an.
 */

export type StaffNoticePriority = "info" | "important";

/** 0 = jede Woche, 1 = Woche A, 2 = Woche B (Vokabular aus dem Stundenplan). */
export type StaffNoticeWeekPattern = 0 | 1 | 2;

export interface StaffNotice {
  id: string;
  title: string;
  body: string;
  priority: StaffNoticePriority;
  /** "YYYY-MM-DD" */
  valid_from: string;
  /** "YYYY-MM-DD"; fehlt = unbefristet */
  valid_until?: string;
  /** ISO-Wochentage 1..7; leer = jeder Tag im Zeitraum */
  weekdays: number[];
  week_pattern: StaffNoticeWeekPattern;
  requires_acknowledgement: boolean;
  active: boolean;
  /** Zeitpunkt der EIGENEN Kenntnisnahme, sofern erfolgt */
  acknowledged_at?: string;
  /** Wie viele Personen den Hinweis bestätigt haben */
  acknowledged_count: number;
}

export interface StaffNoticeInput {
  title: string;
  body: string;
  priority: StaffNoticePriority;
  valid_from: string;
  valid_until?: string | null;
  weekdays: number[];
  week_pattern: StaffNoticeWeekPattern;
  requires_acknowledgement: boolean;
  active: boolean;
}

interface ApiResponse<T> {
  status?: string;
  data?: T;
  error?: string;
}

async function throwApiError(
  response: Response,
  fallback: string,
): Promise<never> {
  let message = fallback;
  try {
    const body = (await response.json()) as { error?: string };
    if (body.error) message = body.error;
  } catch {
    // Kein JSON im Körper — beim deutschen Ersatztext bleiben.
  }
  throw new Error(message);
}

async function request<T>(
  url: string,
  init: RequestInit | undefined,
  fallback: string,
): Promise<T | undefined> {
  const response = await fetch(url, init);
  if (!response.ok) await throwApiError(response, fallback);
  const body = (await response.json()) as ApiResponse<T>;
  return body.data;
}

const BASE = "/api/staff-notices";

/** Die heute geltenden Hinweise — die Sicht des Teams auf der Startseite. */
export async function fetchTodaysNotices(): Promise<StaffNotice[]> {
  const data = await request<StaffNotice[]>(
    `${BASE}/today`,
    undefined,
    "Tagesinformationen konnten nicht geladen werden",
  );
  return data ?? [];
}

/** Alle Hinweise des Mandanten — Leitungssicht, auch abgeschaltete. */
export async function fetchStaffNotices(): Promise<StaffNotice[]> {
  const data = await request<StaffNotice[]>(
    BASE,
    undefined,
    "Tagesinformationen konnten nicht geladen werden",
  );
  return data ?? [];
}

function jsonBody(input: StaffNoticeInput): RequestInit {
  return {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  };
}

export async function createStaffNotice(
  input: StaffNoticeInput,
): Promise<StaffNotice> {
  const data = await request<StaffNotice>(
    BASE,
    jsonBody(input),
    "Tagesinformation konnte nicht erstellt werden",
  );
  if (!data) throw new Error("Tagesinformation konnte nicht erstellt werden");
  return data;
}

export async function updateStaffNotice(
  id: string,
  input: StaffNoticeInput,
): Promise<StaffNotice> {
  const data = await request<StaffNotice>(
    `${BASE}/${encodeURIComponent(id)}`,
    { ...jsonBody(input), method: "PUT" },
    "Tagesinformation konnte nicht gespeichert werden",
  );
  if (!data)
    throw new Error("Tagesinformation konnte nicht gespeichert werden");
  return data;
}

export async function deleteStaffNotice(id: string): Promise<void> {
  await request<unknown>(
    `${BASE}/${encodeURIComponent(id)}`,
    { method: "DELETE" },
    "Tagesinformation konnte nicht gelöscht werden",
  );
}

export async function acknowledgeStaffNotice(id: string): Promise<void> {
  await request<unknown>(
    `${BASE}/${encodeURIComponent(id)}/acknowledge`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: "{}",
    },
    "Kenntnisnahme konnte nicht gespeichert werden",
  );
}

const WEEKDAY_LABELS = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"] as const;

/**
 * Beschreibt die Wiederholung in einem Satzteil, wie ihn jemand lesen würde:
 * "dienstags", "Mo, Mi, Fr", "täglich" — plus die Woche, falls es eine gibt.
 */
export function describeRecurrence(notice: StaffNotice): string {
  const days =
    notice.weekdays.length === 0 || notice.weekdays.length === 7
      ? "täglich"
      : notice.weekdays
          .map((day) => WEEKDAY_LABELS[day - 1] ?? "")
          .filter(Boolean)
          .join(", ");

  if (notice.week_pattern === 1) return `${days} · Woche A`;
  if (notice.week_pattern === 2) return `${days} · Woche B`;
  return days;
}
