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
  /**
   * Backend int64 id, carried as a string like every other id in the frontend
   * (see the type-mapping rule in CLAUDE.md). Converted back to a number only
   * at the validated backend boundary in prepareStudentForBackend — modeling
   * it as a JS number would silently round ids above Number.MAX_SAFE_INTEGER.
   */
  companion_student_id: string;
  first_name?: string;
  last_name?: string;
  weekdays: CompanionWeekday[];
}

/**
 * One confirmed answer to "may we add 'Anderes Kind' to this child's own
 * Heimweg?" — the child and the weekdays the question actually named.
 *
 * It travels with the save because the backend re-checks the conflicts against
 * freshly locked rows: a conflict that appeared while the user was reading the
 * question was never on screen, so the backend asks again instead of widening
 * a child nobody agreed to.
 */
export interface CompanionExtensionConfirmation {
  companion_student_id: string;
  weekdays: CompanionWeekday[];
}

/**
 * Folds a new confirmation into the ones already given, merging the weekdays of
 * a child that is confirmed twice. Order is irrelevant — the backend only asks
 * whether a conflicting day is covered.
 */
export function mergeCompanionConfirmations(
  current: CompanionExtensionConfirmation[],
  incoming: CompanionExtensionConfirmation[],
): CompanionExtensionConfirmation[] {
  const byStudent = new Map<string, Set<CompanionWeekday>>();
  for (const entry of [...current, ...incoming]) {
    const days =
      byStudent.get(entry.companion_student_id) ?? new Set<CompanionWeekday>();
    for (const day of entry.weekdays) days.add(day);
    byStudent.set(entry.companion_student_id, days);
  }
  return [...byStudent].map(([companion_student_id, days]) => ({
    companion_student_id,
    weekdays: ALL_COMPANION_WEEKDAYS.filter((day) => days.has(day)),
  }));
}

/**
 * Order-independent fingerprint of a companion list, so reordering (or a
 * re-fetch handing the same links back in another order) is not mistaken for an
 * edit.
 *
 * Every form that can submit the list needs it: the submitted list REPLACES the
 * stored one, so it may only travel when the user actually edited it — sending
 * the loaded snapshot on an unrelated save would overwrite whatever someone
 * else changed in the meantime.
 */
export function companionsFingerprint(companions: StudentCompanion[]): string {
  return companions
    .map((companion) => {
      const days = ALL_COMPANION_WEEKDAYS.filter((day) =>
        companion.weekdays.includes(day),
      );
      return `${companion.companion_student_id}:${days.join(",")}`;
    })
    .sort()
    .join("|");
}

/** The companion entry as it arrives on the wire (backend int64 → JSON number). */
interface RawStudentCompanion {
  companion_student_id: number | string;
  first_name?: string;
  last_name?: string;
  weekdays?: CompanionWeekday[];
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
  const raw =
    (await parseResponse<RawStudentCompanion[] | null>(response)) ?? [];
  return raw.map((companion) => ({
    companion_student_id: String(companion.companion_student_id),
    first_name: companion.first_name,
    last_name: companion.last_name,
    weekdays: companion.weekdays ?? [],
  }));
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
