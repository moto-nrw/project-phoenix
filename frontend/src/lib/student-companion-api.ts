import { getSession } from "next-auth/react";

/**
 * Laufgemeinschaft ("läuft mit"): the children a child walks home with.
 *
 * There is deliberately no group entity — a Laufgemeinschaft is the set of
 * children reachable through these links on one weekday. The backend stores
 * each link once as an undirected edge, so adding Tom to Lina's list also puts
 * Lina on Tom's list.
 */

/** Weekday keys, identical to the ones the departure plan already uses. */
export type CompanionWeekday = "mon" | "tue" | "wed" | "thu" | "fri";

export const COMPANION_WEEKDAYS: {
  value: CompanionWeekday;
  label: string;
  short: string;
}[] = [
  { value: "mon", label: "Montag", short: "Mo" },
  { value: "tue", label: "Dienstag", short: "Di" },
  { value: "wed", label: "Mittwoch", short: "Mi" },
  { value: "thu", label: "Donnerstag", short: "Do" },
  { value: "fri", label: "Freitag", short: "Fr" },
];

export const ALL_COMPANION_WEEKDAYS: CompanionWeekday[] =
  COMPANION_WEEKDAYS.map((day) => day.value);

/** One companion as returned by the API. */
export interface StudentCompanion {
  companion_student_id: number;
  first_name?: string;
  last_name?: string;
  weekdays: CompanionWeekday[];
}

interface ApiWrapper<T> {
  data: T;
}

function unwrap<T>(value: ApiWrapper<T> | T): T {
  if (
    value &&
    typeof value === "object" &&
    "data" in (value as ApiWrapper<T>)
  ) {
    return (value as ApiWrapper<T>).data;
  }
  return value as T;
}

async function authHeaders(): Promise<HeadersInit> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  const session = await getSession();
  if (session?.user?.token) {
    headers.Authorization = `Bearer ${session.user.token}`;
  }
  return headers;
}

/**
 * Pulls the German error text out of the backend's error envelope
 * ({ status, error }), falling back to the raw body and finally the status.
 */
async function errorMessage(response: Response): Promise<string> {
  const text = await response.text().catch(() => "");
  if (text) {
    try {
      const parsed: unknown = JSON.parse(text);
      if (parsed && typeof parsed === "object") {
        const body = parsed as { error?: unknown; status?: unknown };
        if (typeof body.error === "string" && body.error.trim()) {
          return body.error;
        }
        if (typeof body.status === "string" && body.status.trim()) {
          return body.status;
        }
      }
    } catch {
      return text;
    }
  }
  return `Anfrage fehlgeschlagen (${response.status})`;
}

async function parseResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    // The backend returns German validation messages verbatim (self-link,
    // unknown child, missing weekday) — surface them instead of a status code.
    throw new Error(await errorMessage(response));
  }
  if (response.status === 204) {
    return undefined as T;
  }
  const payload = (await response.json()) as ApiWrapper<T> | T;
  return unwrap<T>(payload);
}

export async function fetchStudentCompanions(
  studentId: string,
): Promise<StudentCompanion[]> {
  const response = await fetch(`/api/students/${studentId}/companions`, {
    method: "GET",
    headers: await authHeaders(),
    credentials: "include",
  });
  return (await parseResponse<StudentCompanion[] | null>(response)) ?? [];
}

/** Full name of a companion, falling back to the id when names are missing. */
export function companionDisplayName(companion: StudentCompanion): string {
  const name = [companion.first_name, companion.last_name]
    .filter(Boolean)
    .join(" ")
    .trim();
  return name || `Kind #${companion.companion_student_id}`;
}

/**
 * German summary of the weekdays a link applies to: "Mo–Fr" when it covers the
 * whole week, otherwise the individual short days.
 */
export function formatCompanionWeekdays(weekdays: CompanionWeekday[]): string {
  const selected = COMPANION_WEEKDAYS.filter((day) =>
    weekdays.includes(day.value),
  );
  if (selected.length === 0) return "kein Tag";
  if (selected.length === COMPANION_WEEKDAYS.length) return "Mo bis Fr";
  return selected.map((day) => day.short).join(", ");
}
