// grade-transition-api.ts
// Client for the Jahrgangsstufenwechsel (grade transition) admin flow (#405).
// Talks to the Next.js proxy routes under /api/admin/grade-transitions which
// forward to the Go backend. Backend snake_case is mapped to camelCase here.

const BASE = "/api/admin/grade-transitions";

// ---------------------------------------------------------------------------
// Backend (snake_case) types — mirror backend/api/admin/grade_transitions.go
// ---------------------------------------------------------------------------

export type TransitionStatus = "draft" | "applied" | "reverted";
export type MappingAction = "promote" | "graduate";
export type HistoryAction = "promoted" | "graduated" | "unchanged";

export interface BackendTransitionMapping {
  id: number;
  from_class: string;
  to_class?: string | null;
  action: MappingAction;
}

export interface BackendGradeTransition {
  id: number;
  academic_year: string;
  status: TransitionStatus;
  applied_at?: string | null;
  applied_by?: number | null;
  reverted_at?: string | null;
  reverted_by?: number | null;
  created_at: string;
  created_by: number;
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
  transition_id: number;
  academic_year: string;
  total_students: number;
  to_promote: number;
  to_graduate: number;
  by_mapping: BackendMappingPreview[];
  unmapped_classes?: BackendUnmappedClass[] | null;
  warnings?: string[] | null;
}

export interface BackendTransitionResult {
  transition_id: number;
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
}

export interface BackendTransitionHistoryEntry {
  id: number;
  transition_id: number;
  student_id: number;
  person_name: string;
  from_class: string;
  to_class?: string | null;
  action: HistoryAction;
  created_at: string;
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
}

export interface TransitionHistoryEntry {
  id: string;
  transitionId: string;
  studentId: string;
  personName: string;
  fromClass: string;
  toClass: string | null;
  action: HistoryAction;
  createdAt: string;
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
    id: data.id.toString(),
    academicYear: data.academic_year,
    status: data.status,
    appliedAt: data.applied_at ?? null,
    revertedAt: data.reverted_at ?? null,
    createdAt: data.created_at,
    notes: data.notes ?? null,
    mappings: (data.mappings ?? []).map((m) => ({
      id: m.id.toString(),
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
    transitionId: data.transition_id.toString(),
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
  };
}

export function mapResult(data: BackendTransitionResult): TransitionResult {
  return {
    transitionId: data.transition_id.toString(),
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
  };
}

export function mapHistoryEntry(
  data: BackendTransitionHistoryEntry,
): TransitionHistoryEntry {
  return {
    id: data.id.toString(),
    transitionId: data.transition_id.toString(),
    studentId: data.student_id.toString(),
    personName: data.person_name,
    fromClass: data.from_class,
    toClass: data.to_class ?? null,
    action: data.action,
    createdAt: data.created_at,
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
  try {
    const body = (await response.json()) as {
      error?: string;
      message?: string;
    };
    return body.error ?? body.message ?? `HTTP ${response.status}`;
  } catch {
    return `HTTP ${response.status}`;
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

export async function listGradeTransitions(): Promise<GradeTransition[]> {
  const response = await fetch(`${BASE}?page_size=100`, {
    cache: "no-store",
  });
  const data = await readJSON<BackendGradeTransition[]>(response);
  return (data ?? []).map(mapTransition);
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

export async function applyGradeTransition(
  id: string,
): Promise<TransitionResult> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}/apply`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({}),
  });
  return mapResult(await readJSON<BackendTransitionResult>(response));
}

export async function revertGradeTransition(
  id: string,
): Promise<TransitionResult> {
  const response = await fetch(`${BASE}/${encodeURIComponent(id)}/revert`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({}),
  });
  return mapResult(await readJSON<BackendTransitionResult>(response));
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
