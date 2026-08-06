import { getCachedSession } from "./session-cache";

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
   * (see the type-mapping rule in CLAUDE.md). It also travels back to the
   * backend as a string — modeling it as a JS number would silently round ids
   * above Number.MAX_SAFE_INTEGER into a DIFFERENT child's id.
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
/**
 * Orders two entries by code point, exactly like Go's sort.Strings.
 *
 * Deliberately NOT localeCompare: the fingerprint is compared byte for byte
 * against the one CompanionLinksFingerprint builds in
 * backend/models/users/student_companion.go, and a collator reorders the ":"
 * and "," separators the entries are built from — every save would then be
 * refused as a stale list.
 */
function compareCodePoints(a: string, b: string): number {
  if (a < b) return -1;
  return a > b ? 1 : 0;
}

export function companionsFingerprint(companions: StudentCompanion[]): string {
  return companions
    .map((companion) => {
      const days = ALL_COMPANION_WEEKDAYS.filter((day) =>
        companion.weekdays.includes(day),
      );
      return `${companion.companion_student_id}:${days.join(",")}`;
    })
    .sort(compareCodePoints)
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
  const session = await getCachedSession();
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

/**
 * Consumers that want to be told when the stored "läuft mit" links may have
 * changed. Module-level and process-wide on purpose — see
 * {@link notifyStudentCompanionsChanged}.
 */
const companionChangeListeners = new Set<() => void>();

/**
 * Subscribe to companion-link changes. Returns the unsubscribe function, so a
 * React effect can return it directly.
 */
export function subscribeStudentCompanionsChanged(
  listener: () => void,
): () => void {
  companionChangeListeners.add(listener);
  return () => {
    companionChangeListeners.delete(listener);
  };
}

/**
 * Announce that the stored links may have changed, so every mounted view that
 * renders them refetches.
 *
 * Companion links live in their own table and do NOT ride along on the student
 * payload, so refreshing the student is not enough — without this a read-only
 * card keeps showing the pre-save list until it is remounted. Every writer of a
 * student's departure plan must call this, not just the ones that send a
 * `companions` list: the backend also TRIMS links whose weekday the new plan no
 * longer allows.
 *
 * Deliberately untargeted (no student id): links are symmetric, so a save on
 * one child changes the other child's list too, and a broad refetch is cheaper
 * than tracking which ids a write touched.
 */
export function notifyStudentCompanionsChanged(): void {
  // Copy first: a listener may unsubscribe while we iterate.
  for (const listener of [...companionChangeListeners]) listener();
}

/**
 * Consumers that render companion data read-only and therefore want to re-read
 * it when the LINKED CHILDREN changed, not the links — see
 * {@link notifyStudentCompanionDisplayChanged}.
 */
const companionDisplayListeners = new Set<() => void>();

/**
 * Subscribe to changes of the data a companion entry DISPLAYS. Returns the
 * unsubscribe function, so a React effect can return it directly.
 */
export function subscribeStudentCompanionDisplayChanged(
  listener: () => void,
): () => void {
  companionDisplayListeners.add(listener);
  return () => {
    companionDisplayListeners.delete(listener);
  };
}

/**
 * Announce that the CHILDREN behind the stored links may have changed, while
 * the links themselves did not.
 *
 * A rename or a group reassignment of a linked child emits only
 * `student_updated` — the edges are untouched, so no companion announcement
 * follows. The fetching card keys its request on its own student id, which such
 * a write never changes, so without this signal it keeps rendering the name it
 * loaded: after a group reassignment that can even keep showing a name the
 * backend would now redact for this viewer.
 *
 * Deliberately separate from {@link notifyStudentCompanionsChanged}: an editing
 * form answers THAT announcement by discarding its draft or blocking the save,
 * which must not happen for a rename somewhere in the school. This bus is for
 * read-only views only.
 */
export function notifyStudentCompanionDisplayChanged(): void {
  // Copy first: a listener may unsubscribe while we iterate.
  for (const listener of [...companionDisplayListeners]) listener();
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
