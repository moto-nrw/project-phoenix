// grade-transition-api.ts
// Client for the Jahrgangsstufenwechsel (grade transition) admin flow (#405).
// Talks to the Next.js proxy routes under /api/admin/grade-transitions which
// forward to the Go backend. Backend snake_case is mapped to camelCase here.

const BASE = "/api/admin/grade-transitions";

// ---------------------------------------------------------------------------
// Backend (snake_case) types — mirror backend/api/admin/grade_transitions.go
// ---------------------------------------------------------------------------

/** A backend int64 identifier on the wire.
 *
 * The grade-transition endpoints serialize ids as JSON STRINGS (`json:"id,string"`)
 * precisely so they survive the trip: a JSON number is an IEEE-754 double here,
 * so an id beyond 2^53 would be rounded during parsing — before any mapper could
 * run — and two distinct rows could collapse onto one key or an apply/revert
 * could address a transition the admin never selected. `number` is still
 * accepted so a response from an older server (or a hand-written test fixture)
 * still maps; only the string form is exact, which is why nothing here ever
 * converts an id back to a number (#405 review). */
export type BackendID = string | number;

/** Normalizes a wire id to the string form the app uses everywhere. A string
 * passes through verbatim — never via Number() — so no precision is lost. */
export function toIdString(id: BackendID): string {
  return typeof id === "string" ? id : id.toString();
}

export type TransitionStatus = "draft" | "applied" | "reverted";
export type MappingAction = "promote" | "graduate";
export type HistoryAction = "promoted" | "graduated" | "unchanged";

export interface BackendTransitionMapping {
  id: BackendID;
  from_class: string;
  to_class?: string | null;
  action: MappingAction;
}

export interface BackendGradeTransition {
  id: BackendID;
  academic_year: string;
  status: TransitionStatus;
  applied_at?: string | null;
  applied_by?: BackendID | null;
  reverted_at?: string | null;
  reverted_by?: BackendID | null;
  created_at: string;
  created_by: BackendID;
  notes?: string | null;
  mappings?: BackendTransitionMapping[];
  can_modify: boolean;
  can_apply: boolean;
  can_revert: boolean;
}

export interface BackendMappingPreview {
  from_class: string;
  to_class?: string | null;
  student_count: number;
  action: MappingAction;
}

export interface BackendUnmappedClass {
  class_name: string;
  student_count: number;
}

export interface BackendTransitionPreview {
  transition_id: BackendID;
  academic_year: string;
  total_students: number;
  to_promote: number;
  to_graduate: number;
  by_mapping: BackendMappingPreview[];
  unmapped_classes?: BackendUnmappedClass[] | null;
  warnings?: string[] | null;
  /** Digest of the exact cohort this preview describes (mappings + affected
   * children + their classes). Sent back on apply so the backend can refuse a
   * confirmation that no longer matches what was shown (#405). */
  fingerprint?: string | null;
}

export interface BackendTransitionResult {
  transition_id: BackendID;
  status: string;
  students_promoted: number;
  students_graduated: number;
  can_revert: boolean;
  warnings?: string[] | null;
}

export interface BackendSuggestedMapping {
  from_class: string;
  to_class?: string | null;
  student_count: number;
  is_graduating: boolean;
  /** true = class name has no grade pattern; graduation guess is not confident */
  ambiguous?: boolean;
}

export interface BackendTransitionHistoryEntry {
  id: BackendID;
  transition_id: BackendID;
  student_id: BackendID;
  person_name: string;
  from_class: string;
  to_class?: string | null;
  action: HistoryAction;
  created_at: string;
  student_state?: GraduateState;
}

// ---------------------------------------------------------------------------
// Frontend (camelCase) types
// ---------------------------------------------------------------------------

export interface TransitionMapping {
  id: string;
  fromClass: string;
  toClass: string | null;
  action: MappingAction;
}

export interface GradeTransition {
  id: string;
  academicYear: string;
  status: TransitionStatus;
  appliedAt: string | null;
  revertedAt: string | null;
  createdAt: string;
  notes: string | null;
  mappings: TransitionMapping[];
  canModify: boolean;
  canApply: boolean;
  canRevert: boolean;
}

export interface MappingPreview {
  fromClass: string;
  toClass: string | null;
  studentCount: number;
  action: MappingAction;
}

export interface UnmappedClass {
  className: string;
  studentCount: number;
}

export interface TransitionPreview {
  transitionId: string;
  academicYear: string;
  totalStudents: number;
  toPromote: number;
  toGraduate: number;
  byMapping: MappingPreview[];
  unmappedClasses: UnmappedClass[];
  warnings: string[];
  /** @see BackendTransitionPreview.fingerprint */
  fingerprint: string;
}

export interface TransitionResult {
  transitionId: string;
  status: string;
  studentsPromoted: number;
  studentsGraduated: number;
  canRevert: boolean;
  warnings: string[];
}

export interface SuggestedMapping {
  fromClass: string;
  toClass: string | null;
  studentCount: number;
  isGraduating: boolean;
  /** true = class name has no grade pattern; do not preselect Abgang */
  ambiguous?: boolean;
}

/**
 * What is left of a graduated child today. The ledger is append-only, so its
 * `action` still says "graduated" for a child a revert brought back or a purge
 * removed — only this decides which actions the Abgänge list may offer.
 */
export type GraduateState = "alumnus" | "restored" | "purged";

export interface TransitionHistoryEntry {
  id: string;
  transitionId: string;
  studentId: string;
  personName: string;
  fromClass: string;
  toClass: string | null;
  action: HistoryAction;
  createdAt: string;
  /**
   * Defaults to "alumnus" when a backend predating the field answers: that is
   * the state every graduated ledger row had before purging existed, so an old
   * response degrades to "shown, deletable" rather than to a wrong state.
   */
  studentState: GraduateState;
}

export interface MappingInput {
  fromClass: string;
  /** null = Abgang (Kind verlässt die OGS, wird als Alumnus deaktiviert) */
  toClass: string | null;
}

export interface CreateTransitionInput {
  academicYear: string;
  notes?: string;
  mappings: MappingInput[];
}

export interface UpdateTransitionInput {
  academicYear?: string;
  notes?: string;
  /** undefined = Mappings unverändert lassen; [] = alle entfernen */
  mappings?: MappingInput[];
}

// ---------------------------------------------------------------------------
// Mappers
// ---------------------------------------------------------------------------

export function mapTransition(data: BackendGradeTransition): GradeTransition {
  return {
    id: toIdString(data.id),
    academicYear: data.academic_year,
    status: data.status,
    appliedAt: data.applied_at ?? null,
    revertedAt: data.reverted_at ?? null,
    createdAt: data.created_at,
    notes: data.notes ?? null,
    mappings: (data.mappings ?? []).map((m) => ({
      id: toIdString(m.id),
      fromClass: m.from_class,
      toClass: m.to_class ?? null,
      action: m.action,
    })),
    canModify: data.can_modify,
    canApply: data.can_apply,
    canRevert: data.can_revert,
  };
}

export function mapPreview(data: BackendTransitionPreview): TransitionPreview {
  return {
    transitionId: toIdString(data.transition_id),
    academicYear: data.academic_year,
    totalStudents: data.total_students,
    toPromote: data.to_promote,
    toGraduate: data.to_graduate,
    byMapping: data.by_mapping.map((m) => ({
      fromClass: m.from_class,
      toClass: m.to_class ?? null,
      studentCount: m.student_count,
      action: m.action,
    })),
    unmappedClasses: (data.unmapped_classes ?? []).map((u) => ({
      className: u.class_name,
      studentCount: u.student_count,
    })),
    warnings: data.warnings ?? [],
    fingerprint: data.fingerprint ?? "",
  };
}

export function mapResult(data: BackendTransitionResult): TransitionResult {
  return {
    transitionId: toIdString(data.transition_id),
    status: data.status,
    studentsPromoted: data.students_promoted,
    studentsGraduated: data.students_graduated,
    canRevert: data.can_revert,
    warnings: data.warnings ?? [],
  };
}

export function mapSuggestion(data: BackendSuggestedMapping): SuggestedMapping {
  return {
    fromClass: data.from_class,
    toClass: data.to_class ?? null,
    studentCount: data.student_count,
    isGraduating: data.is_graduating,
    // Left undefined when the backend omits it (confident suggestion), which
    // the mapper's toEqual tests treat as absent.
    ambiguous: data.ambiguous,
  };
}

export function mapHistoryEntry(
  data: BackendTransitionHistoryEntry,
): TransitionHistoryEntry {
  return {
    id: toIdString(data.id),
    transitionId: toIdString(data.transition_id),
    studentId: toIdString(data.student_id),
    personName: data.person_name,
    fromClass: data.from_class,
    toClass: data.to_class ?? null,
    action: data.action,
    createdAt: data.created_at,
    studentState: data.student_state ?? "alumnus",
  };
}

function toBackendMappings(mappings: MappingInput[]) {
  return mappings.map((m) => ({
    from_class: m.fromClass,
    to_class: m.toClass,
  }));
}

// ---------------------------------------------------------------------------
// Fetch plumbing (proxy routes wrap payloads in { data })
// ---------------------------------------------------------------------------

async function readError(response: Response): Promise<string> {
  return (await readErrorWithCode(response)).message;
}

/**
 * Stable backend error code returned (409) when an apply is refused because
 * graduating children are still checked in. Mirrors the code the Go handler
 * sets via ErrorConflictWithCode (#405).
 */
export const GRADUATES_CHECKED_IN_CODE = "graduates_checked_in";

/**
 * Stable backend error code returned (409) when a revert is refused because the
 * targeted transition is no longer the latest applied one — another
 * administrator applied a newer transition since this page loaded. Recoverable:
 * the caller must reload the list and revert the new latest first. Mirrors the
 * code the Go handler sets via ErrorConflictWithCode (#405).
 */
export const NOT_LATEST_TRANSITION_CODE = "not_latest_transition";

/**
 * Stable backend error code returned (409) when an apply is refused because the
 * previewed cohort has changed since the preview was rendered — another admin
 * edited the mappings or moved a child between classes. The UI must reload the
 * preview and ask for a fresh confirmation instead of retrying blindly (#405).
 */
export const PREVIEW_STALE_CODE = "preview_stale";

/**
 * Error thrown by {@link applyGradeTransition} and {@link revertGradeTransition}
 * that preserves the backend's stable error `code`, so the caller can
 * distinguish a recoverable conflict (graduates still checked in, stale revert
 * target) from a generic failure.
 */
export class TransitionRequestError extends Error {
  readonly code?: string;
  constructor(message: string, code?: string) {
    super(message);
    this.name = "TransitionRequestError";
    this.code = code;
  }
}

async function readErrorWithCode(
  response: Response,
): Promise<{ message: string; code?: string }> {
  try {
    const body = (await response.json()) as {
      error?: string;
      message?: string;
      code?: string;
    };
    return {
      message: body.error ?? body.message ?? `HTTP ${response.status}`,
      code: body.code,
    };
  } catch {
    return { message: `HTTP ${response.status}` };
  }
}

async function readJSON<T>(response: Response): Promise<T> {
  if (!response.ok) {
    throw new Error(await readError(response));
  }
  const body = (await response.json()) as { data?: T } | T;
  if (body && typeof body === "object" && "data" in body) {
    return (body as { data: T }).data;
  }
  return body as T;
}

// ---------------------------------------------------------------------------
// API calls
// ---------------------------------------------------------------------------

const LIST_PAGE_SIZE = 100;

// Safety bound against a backend that ignores `page` and would loop forever —
// 100 pages is 10.000 transitions, far past anything a school can produce, so
// it never truncates in practice.
const LIST_PAGE_LIMIT = 100;

// Fetches every transition, not just the first page. The list feeds both the
// table and the revert-target picker, and reverts must unwind newest-first: with
// more than one page of rows, a single created_at DESC window can hide the only
// revertable transition, or point the dialog at an applied row that is not the
// latest — which 409s, refreshes the same truncated window, and re-picks the
// same dead target (#405 review).
export async function listGradeTransitions(): Promise<GradeTransition[]> {
  // created_at DESC: a row inserted between page fetches shifts the rows behind
  // it, so pages can overlap. Keyed by id rather than concatenated — as the
  // exact string id, never a number, so two rows can never share a key through
  // float rounding (#405 review).
  const byID = new Map<string, BackendGradeTransition>();

  for (let page = 1; page <= LIST_PAGE_LIMIT; page++) {
    const response = await fetch(
      `${BASE}?page=${page}&page_size=${LIST_PAGE_SIZE}`,
      { cache: "no-store" },
    );
    const data = (await readJSON<BackendGradeTransition[]>(response)) ?? [];
    for (const transition of data) {
      byID.set(toIdString(transition.id), transition);
    }
    if (data.length < LIST_PAGE_SIZE) break;
  }

  return Array.from(byID.values()).map(mapTransition);
}

export async function getGradeTransition(id: string): Promise<GradeTransition> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}`, {
    cache: "no-store",
  });
  return mapTransition(await readJSON<BackendGradeTransition>(response));
}

export async function createGradeTransition(
  input: CreateTransitionInput,
): Promise<GradeTransition> {
  const response = await fetch(BASE, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      academic_year: input.academicYear,
      notes: input.notes,
      mappings: toBackendMappings(input.mappings),
    }),
  });
  return mapTransition(await readJSON<BackendGradeTransition>(response));
}

export async function updateGradeTransition(
  id: string,
  input: UpdateTransitionInput,
): Promise<GradeTransition> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      academic_year: input.academicYear,
      notes: input.notes,
      mappings: input.mappings ? toBackendMappings(input.mappings) : undefined,
    }),
  });
  return mapTransition(await readJSON<BackendGradeTransition>(response));
}

export async function deleteGradeTransition(id: string): Promise<void> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}

export async function previewGradeTransition(
  id: string,
): Promise<TransitionPreview> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}/preview`, {
    cache: "no-store",
  });
  return mapPreview(await readJSON<BackendTransitionPreview>(response));
}

/**
 * Reads an apply/revert response, throwing a {@link TransitionRequestError} that
 * keeps the backend's stable `code` on failure. The generic {@link readJSON}
 * path drops that code, which would leave the caller unable to tell a
 * recoverable 409 conflict from a real failure.
 */
async function readResultOrThrow(
  response: Response,
): Promise<TransitionResult> {
  if (!response.ok) {
    const { message, code } = await readErrorWithCode(response);
    throw new TransitionRequestError(message, code);
  }
  const body = (await response.json()) as
    { data?: BackendTransitionResult } | BackendTransitionResult;
  const data =
    body && typeof body === "object" && "data" in body
      ? (body as { data: BackendTransitionResult }).data
      : (body as BackendTransitionResult);
  return mapResult(data);
}

/**
 * Applies a transition. `expectedFingerprint` binds the call to the preview the
 * admin confirmed: the backend refuses with {@link PREVIEW_STALE_CODE} when the
 * affected children or mappings have changed since (#405).
 */
export async function applyGradeTransition(
  id: string,
  expectedFingerprint?: string,
): Promise<TransitionResult> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}/apply`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(
      expectedFingerprint ? { expected_fingerprint: expectedFingerprint } : {},
    ),
  });
  return readResultOrThrow(response);
}

export async function revertGradeTransition(
  id: string,
): Promise<TransitionResult> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}/revert`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({}),
  });
  return readResultOrThrow(response);
}

export async function fetchTransitionClasses(): Promise<string[]> {
  const response = await fetch(`${BASE}/classes`, { cache: "no-store" });
  const data = await readJSON<string[]>(response);
  return data ?? [];
}

export async function fetchSuggestedMappings(): Promise<SuggestedMapping[]> {
  const response = await fetch(`${BASE}/suggest`, { cache: "no-store" });
  const data = await readJSON<BackendSuggestedMapping[]>(response);
  return (data ?? []).map(mapSuggestion);
}

export async function fetchTransitionHistory(
  id: string,
): Promise<TransitionHistoryEntry[]> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}/history`, {
    cache: "no-store",
  });
  const data = await readJSON<BackendTransitionHistoryEntry[]>(response);
  return (data ?? []).map(mapHistoryEntry);
}

/**
 * Permanently deletes a child that a grade transition graduated — student row,
 * person row, photo, and their name in the transition ledger.
 *
 * This is the counterpart to the soft delete graduation performs: the child is
 * hidden everywhere and every ordinary student route answers 404 for them, so
 * without this there is no way to honour a retention rule or an erasure
 * request for a departed child. Not reversible by `Zurücksetzen` afterwards —
 * the ledger row survives to keep the transition's count honest, but there is
 * nothing left to restore.
 */
export async function purgeGraduatedStudent(studentId: string): Promise<void> {
  const response = await fetch(
    `/api/students/${encodeURIComponent(studentId)}/purge`,
    { method: "DELETE" },
  );
  if (!response.ok) {
    throw new Error(await readError(response));
  }
}
