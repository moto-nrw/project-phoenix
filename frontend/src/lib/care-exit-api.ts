// Client-side API für "Betreuung beenden" (#2487).
//
// Ein regulärer Austritt ist keine Datenlöschung: Er setzt einen letzten
// Betreuungstag, beendet ab dem Folgetag alle laufenden Wirkungen und lässt
// die Historie unverändert stehen. Die endgültige Löschung (student-api.ts)
// bleibt daneben bestehen und ist strenger geschützt.

/** Austrittsgründe. Nur "other" trägt einen Freitext. */
export type CareExitReason = "moved_away" | "no_care_needed" | "other";

export const CARE_EXIT_REASON_LABELS: Record<CareExitReason, string> = {
  moved_away: "Umzug",
  no_care_needed: "Kein Betreuungsbedarf mehr",
  other: "Anderer Grund",
};

/** Maximale Auswahl pro Vorgang — gleiche Grenze wie im Backend. */
export const CARE_EXIT_MAX_SELECTION = 500;

/** Maximale Länge des Freitexts bei "Anderer Grund". */
export const CARE_EXIT_NOTE_MAX_LENGTH = 200;

/** Auswirkungen für ein Kind, so wie die Vorschau sie meldet. */
export interface CareExitImpact {
  studentId: string;
  firstName: string;
  lastName: string;
  schoolClass: string;
  /** Geplante Termine nach dem letzten Betreuungstag, die das Kind verliert. */
  plannedRosterRows: number;
  /** Angebots- und AG-Buchungen, die am letzten Betreuungstag enden. */
  activityBookings: number;
  /** Offene Eltern-Anfragen, die mit dem Austritt geschlossen werden. */
  openParentRequests: number;
  hasRfidTag: boolean;
  currentlyPresent: boolean;
  /** Bereits hinterlegtes Betreuungsende (dann ist der Vorgang eine Änderung). */
  plannedEndsOn: string | null;
  /** Leer, wenn das Kind beendet werden kann; sonst der konkrete Grund. */
  blocker: string;
}

export interface CareExitPreview {
  /** Unveränderbarer Vorschau-Stand, den die Bestätigung zurückgeben muss. */
  token: string;
  lastCareDay: string;
  reason: CareExitReason;
  reasonNote: string;
  /** true, sobald mindestens ein Kind einen Blocker hat. */
  blocked: boolean;
  students: CareExitImpact[];
}

export interface CareExitResult {
  studentsEnded: number;
  rosterRowsRemoved: number;
  bookingsEnded: number;
}

export interface EndedCareEntry {
  studentId: string;
  firstName: string;
  lastName: string;
  schoolClass: string;
  lastCareDay: string;
  reason: CareExitReason | null;
  reasonNote: string | null;
  recordedAt: string | null;
}

export interface EndedCarePage {
  items: EndedCareEntry[];
  total: number;
  page: number;
  pageSize: number;
}

export interface CareExitInput {
  studentIds: string[];
  lastCareDay: string;
  reason: CareExitReason;
  reasonNote?: string;
}

interface WireImpact {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class?: string;
  planned_roster_rows: number;
  activity_bookings: number;
  open_parent_requests: number;
  has_rfid_tag: boolean;
  currently_present: boolean;
  planned_ends_on?: string | null;
  blocker?: string;
}

interface WirePreview {
  token: string;
  last_care_day: string;
  reason: CareExitReason;
  reason_note?: string;
  blocked: boolean;
  students: WireImpact[];
}

interface WireEndedCare {
  student_id: string;
  first_name: string;
  last_name: string;
  school_class?: string;
  last_care_day: string;
  reason?: CareExitReason | null;
  reason_note?: string | null;
  recorded_at?: string | null;
}

interface Envelope<T> {
  data: T;
  error?: string;
  message?: string;
}

// Eigener Fetch statt authFetch: die Backend-Meldungen sind deutschsprachige
// Nutzertexte ("Die Betreuung wurde nicht beendet. …") und müssen die
// aufrufende UI erreichen — authFetch wirft nur den generischen Statustext.
async function request<T>(
  url: string,
  method: "GET" | "POST",
  body?: unknown,
): Promise<T> {
  const response = await fetch(url, {
    method,
    credentials: "include",
    cache: "no-store",
    ...(body !== undefined && {
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),
  });
  const payload = (await response.json().catch(() => null)) as
    (Envelope<unknown> & Record<string, unknown>) | null;
  if (!response.ok) {
    const message =
      (typeof payload?.error === "string" && payload.error) ||
      (typeof payload?.message === "string" && payload.message) ||
      `API error (${response.status})`;
    throw new Error(message);
  }
  return payload as T;
}

function toWireInput(input: CareExitInput, token?: string) {
  return {
    student_ids: input.studentIds,
    last_care_day: input.lastCareDay,
    reason: input.reason,
    // Nur "Anderer Grund" trägt einen Freitext; das Backend weist eine
    // Kombination aus kategorisiertem Grund und Freitext ab.
    reason_note: input.reason === "other" ? (input.reasonNote ?? "") : "",
    ...(token !== undefined && { token }),
  };
}

function mapImpact(wire: WireImpact): CareExitImpact {
  return {
    studentId: wire.student_id,
    firstName: wire.first_name,
    lastName: wire.last_name,
    schoolClass: wire.school_class ?? "",
    plannedRosterRows: wire.planned_roster_rows,
    activityBookings: wire.activity_bookings,
    openParentRequests: wire.open_parent_requests,
    hasRfidTag: wire.has_rfid_tag,
    currentlyPresent: wire.currently_present,
    plannedEndsOn: wire.planned_ends_on ?? null,
    blocker: wire.blocker ?? "",
  };
}

/** Fragt, was das Beenden bewirken würde. Ändert nichts. */
export async function previewCareExit(
  input: CareExitInput,
): Promise<CareExitPreview> {
  const envelope = await request<Envelope<WirePreview>>(
    "/api/students/care-end/preview",
    "POST",
    toWireInput(input),
  );
  const wire = envelope.data;
  return {
    token: wire.token,
    lastCareDay: wire.last_care_day,
    reason: wire.reason,
    reasonNote: wire.reason_note ?? "",
    blocked: wire.blocked,
    students: (wire.students ?? []).map(mapImpact),
  };
}

/**
 * Beendet die Betreuung für genau den Vorschau-Stand. Hat sich seither etwas
 * geändert, wird nichts geschrieben und das Backend nennt den Grund.
 */
export async function confirmCareExit(
  token: string,
  input: CareExitInput,
): Promise<CareExitResult> {
  const envelope = await request<
    Envelope<{
      students_ended: number;
      roster_rows_removed: number;
      bookings_ended: number;
    }>
  >("/api/students/care-end", "POST", toWireInput(input, token));
  return {
    studentsEnded: envelope.data.students_ended,
    rosterRowsRemoved: envelope.data.roster_rows_removed,
    bookingsEnded: envelope.data.bookings_ended,
  };
}

/** Storniert ein noch nicht wirksames Betreuungsende. */
export async function cancelCareExit(studentIds: string[]): Promise<number> {
  const envelope = await request<Envelope<{ students_cancelled: number }>>(
    "/api/students/care-end/cancel",
    "POST",
    { student_ids: studentIds },
  );
  return envelope.data.students_cancelled;
}

/**
 * Nimmt die Betreuung eines Kindes wieder auf. `checked` ist die ausdrückliche
 * Bestätigung, dass Gruppe, Angebote, Wochenplan sowie Ankunfts- und Gehzeiten
 * geprüft wurden — nichts wird automatisch wieder eingeschaltet.
 */
export async function resumeCare(
  studentId: string,
  newStart: string,
  checked: boolean,
): Promise<void> {
  await request<Envelope<unknown>>(
    `/api/students/${studentId}/care-end/resume`,
    "POST",
    { new_start: newStart, checked },
  );
}

/** Lädt die Ansicht "Beendete Betreuungen". */
export async function fetchEndedCare(params: {
  search?: string;
  page?: number;
  pageSize?: number;
}): Promise<EndedCarePage> {
  const query = new URLSearchParams();
  if (params.search) query.set("search", params.search);
  if (params.page) query.set("page", String(params.page));
  if (params.pageSize) query.set("page_size", String(params.pageSize));
  const suffix = query.toString() ? `?${query.toString()}` : "";

  const envelope = await request<
    Envelope<{
      items: WireEndedCare[] | null;
      total: number;
      page: number;
      page_size: number;
    }>
  >(`/api/students/ended-care${suffix}`, "GET");

  return {
    items: (envelope.data.items ?? []).map((wire) => ({
      studentId: wire.student_id,
      firstName: wire.first_name,
      lastName: wire.last_name,
      schoolClass: wire.school_class ?? "",
      lastCareDay: wire.last_care_day,
      reason: wire.reason ?? null,
      reasonNote: wire.reason_note ?? null,
      recordedAt: wire.recorded_at ?? null,
    })),
    total: envelope.data.total,
    page: envelope.data.page,
    pageSize: envelope.data.page_size,
  };
}

/**
 * Ist für dieses Kind ein Austritt hinterlegt, der noch nicht gegriffen hat?
 *
 * Entscheidend ist, ob die Schule das Ende eingetragen hat, nicht wie weit es
 * entfernt liegt: Ein Ende Monate voraus ist in aller Regel das reguläre Ende
 * der Anmeldephase (in der Demo-Schule tragen alle über die Anmeldung
 * erfassten Kinder denselben Tag im übernächsten Sommer). Nur ein
 * eingetragener Austritt bekommt den Hinweis in der Liste und die Knöpfe
 * "Ende ändern" und "Ende stornieren" — und der bekommt sie immer, sonst
 * ließe sich ein früh geplanter Austritt nicht mehr zurücknehmen.
 */
export function hasPlannedCareExit(student: {
  care_ends_on?: string;
  care_ended?: boolean;
  care_exit_recorded?: boolean;
}): boolean {
  return Boolean(
    student.care_exit_recorded && student.care_ends_on && !student.care_ended,
  );
}
