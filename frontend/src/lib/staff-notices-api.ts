/**
 * Client für die Tagesinformationen (#2180): interne Hinweise der Leitung an
 * das Team. Spricht die Next.js-Proxyrouten unter /api/staff-notices an, die
 * mit dem Personal-JWT ans Go-Backend weiterreichen. int64-Ids kommen bereits
 * als String an.
 *
 * Die Lesefunktionen (heute geltende Hinweise, Kenntnisnahme) gibt es auch für
 * moto schule (#2208) über `createStaffNoticesReadApi` mit dem Schul-Pfad;
 * Verwaltung und Bestätigungsliste bleiben im OGS-Portal.
 */

export type StaffNoticePriority = "info" | "important";

/**
 * Fenster-Ereignis, mit dem eine Kenntnisnahme die Zähler in der Navigation
 * sofort nachziehen lässt — in beiden Portalen dasselbe Ereignis.
 */
export const STAFF_NOTICES_REFRESH_EVENT = "staff-notices-refresh";

/**
 * Zielgruppe (#2208): die ganze Einrichtung, nur die Betreuung im OGS-Portal
 * oder nur die Lehrkräfte in moto schule. Drei feste Werte, kein Verteiler.
 */
export type StaffNoticeAudience = "all" | "staff" | "lehrkraft";

/** 0 = jede Woche, 1 = Woche A, 2 = Woche B (Vokabular aus dem Stundenplan). */
export type StaffNoticeWeekPattern = 0 | 1 | 2;

export interface StaffNotice {
  id: string;
  title: string;
  body: string;
  priority: StaffNoticePriority;
  audience: StaffNoticeAudience;
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
  acknowledged_count?: number;
}

export interface StaffNoticeInput {
  title: string;
  body: string;
  priority: StaffNoticePriority;
  audience: StaffNoticeAudience;
  valid_from: string;
  valid_until?: string | null;
  weekdays: number[];
  week_pattern: StaffNoticeWeekPattern;
  requires_acknowledgement: boolean;
  active: boolean;
}

/** Eine Zeile der Bestätigungsliste (#2208): wer wann bestätigt hat. */
export interface StaffNoticeAcknowledger {
  account_id: string;
  name: string;
  /** RFC 3339 */
  acknowledged_at: string;
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
  if (response.status === 204) return undefined;
  const body = (await response.json()) as ApiResponse<T>;
  return body.data;
}

const BASE = "/api/staff-notices";

/**
 * Die Lesefläche eines Portals: heute geltende Hinweise und die eigene
 * Kenntnisnahme. Das OGS-Portal nutzt sie über /api/staff-notices, moto schule
 * über /api/school/staff-notices — dieselben Aufrufe, andere Sitzung.
 */
export interface StaffNoticesReadApi {
  fetchTodaysNotices: () => Promise<StaffNotice[]>;
  acknowledgeStaffNotice: (id: string) => Promise<void>;
}

export function createStaffNoticesReadApi(base: string): StaffNoticesReadApi {
  return {
    async fetchTodaysNotices() {
      const data = await request<StaffNotice[] | { data?: StaffNotice[] }>(
        `${base}/today`,
        undefined,
        "Tagesinformationen konnten nicht geladen werden",
      );
      // Der Schul-Proxy reicht die Backend-Antwort `{ data: [...] }` als
      // Ganzes in seiner Hülle weiter, der OGS-Proxy packt sie vorher aus.
      // Beide Formen landen hier bei derselben Liste.
      if (Array.isArray(data)) return data;
      return data?.data ?? [];
    },
    async acknowledgeStaffNotice(id: string) {
      await request<unknown>(
        `${base}/${encodeURIComponent(id)}/acknowledge`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: "{}",
        },
        "Kenntnisnahme konnte nicht gespeichert werden",
      );
    },
  };
}

const tenantReadApi = createStaffNoticesReadApi(BASE);

/** Die heute geltenden Hinweise — die Sicht des Teams im OGS-Portal. */
export const fetchTodaysNotices = tenantReadApi.fetchTodaysNotices;

/** Kenntnisnahme im OGS-Portal. */
export const acknowledgeStaffNotice = tenantReadApi.acknowledgeStaffNotice;

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

/**
 * Die Bestätigungsliste eines Hinweises (#2208): wer wann zur Kenntnis
 * genommen hat, neueste zuerst. Nur für die Leitung erreichbar.
 */
export async function fetchNoticeAcknowledgements(
  id: string,
): Promise<StaffNoticeAcknowledger[]> {
  const data = await request<StaffNoticeAcknowledger[]>(
    `${BASE}/${encodeURIComponent(id)}/acknowledgements`,
    undefined,
    "Die Bestätigungen konnten nicht geladen werden",
  );
  return data ?? [];
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

/** Die Zielgruppe so, wie sie in der Verwaltung steht. */
export const AUDIENCE_LABELS: Readonly<Record<StaffNoticeAudience, string>> = {
  all: "Alle",
  staff: "Nur Betreuung",
  lehrkraft: "Nur Lehrkräfte",
};

/** Beschreibt die Zielgruppe für die Liste der Leitung: "Für alle". */
export function describeAudience(audience: StaffNoticeAudience): string {
  switch (audience) {
    case "staff":
      return "Nur für die Betreuung";
    case "lehrkraft":
      return "Nur für Lehrkräfte";
    default:
      return "Für alle";
  }
}
