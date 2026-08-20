// Staff API service for fetching all staff members and their supervision status

import { sessionFetch } from "./session-cache";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffAPI" });

// Backend response types (already mapped by the API route handler)
export interface BackendStaffResponse {
  id: string;
  name: string;
  firstName: string;
  lastName: string;
  specialization?: string | null;
  role: string | null;
  qualifications: string | null;
  account_role?: string | null;
  tag_id: string | null;
  staff_notes: string | null;
  created_at: string;
  updated_at: string;
  staff_id?: string;
  teacher_id?: string;
  employment_type?: string | null;
  was_present_today?: boolean;
  work_status?: string;
  absence_type?: string;
  /** The school's own Abwesenheitsart wording for today's absence (#2403). */
  absence_type_label?: string;
}

interface ActiveSupervisionResponse {
  id: number;
  staff_id: number;
  group_id: number;
  role: string;
  start_date: string;
  end_date?: string;
  is_active: boolean;
  active_group?: {
    id: number;
    name: string;
    room_id?: number;
    room?: {
      id: number;
      name: string;
    };
  };
}

// Individual supervision entry for a staff member
interface StaffSupervision {
  roomId: string;
  roomName: string;
  activeGroupId: string;
}

// Frontend types
export interface Staff {
  id: string;
  name: string;
  firstName: string;
  lastName: string;
  email?: string;
  role?: string; // Display role (custom position from teacher profile)
  accountRole?: string; // Auth role (Admin/Betreuer/Extern)
  specialization?: string;
  qualifications?: string;
  staffNotes?: string;
  employmentType?: string; // full_time, part_time, minijob
  hasRfid: boolean;
  isTeacher: boolean;
  // Supervision status
  isSupervising: boolean;
  currentLocation?: string;
  supervisionRole?: string;
  supervisions: StaffSupervision[]; // Array of active supervisions
  wasPresentToday?: boolean;
  // Time-tracking
  workStatus?: string;
  absenceType?: string;
  /** The school's own Abwesenheitsart wording, "" for a standard type (#2403). */
  absenceTypeLabel?: string;
  isFinancialProfile?: boolean;
  isLimitedProfile?: boolean;
}

export interface StaffFilters {
  search?: string;
  status?: "all" | "supervising" | "available";
  type?: "all" | "teachers" | "staff";
}

export interface StaffDocumentDirectoryEntry {
  id: string;
  name: string;
  firstName: string;
  lastName: string;
}

/** Active group with supervisors and room info */
interface ActiveGroupInfo {
  supervisors?: Array<{
    staff_id?: number;
    role?: string;
  }>;
  room?: {
    id: number;
    name: string;
  };
}

/** Supervised group entry for staff mapping */
interface SupervisedGroupEntry {
  group: ActiveGroupInfo;
  role?: string;
}

/** Active group with ID for supervision mapping */
interface ActiveGroupWithId extends ActiveGroupInfo {
  id?: number;
}

/**
 * Supervised group entry with group ID
 */
interface SupervisedGroupEntryWithId extends SupervisedGroupEntry {
  groupId?: number;
}

/**
 * Extracts staff list from various API response formats
 */
function extractStaffList(
  data: BackendStaffResponse[] | { data: BackendStaffResponse[] },
): BackendStaffResponse[] {
  if (Array.isArray(data)) {
    return data;
  }
  if (
    data &&
    typeof data === "object" &&
    "data" in data &&
    Array.isArray(data.data)
  ) {
    return data.data;
  }
  return [];
}

/**
 * Extracts active groups from potentially wrapped API response
 */
function extractActiveGroups(data: unknown): ActiveGroupWithId[] {
  if (Array.isArray(data)) {
    return data as ActiveGroupWithId[];
  }

  if (!data || typeof data !== "object" || !("data" in data)) {
    return [];
  }

  const wrappedData = (data as { data?: unknown }).data;

  // Double wrapped - frontend wrapper around backend response
  if (wrappedData && typeof wrappedData === "object" && "data" in wrappedData) {
    const backendResponse = wrappedData as { data?: unknown };
    if (Array.isArray(backendResponse.data)) {
      return backendResponse.data as ActiveGroupWithId[];
    }
  }

  // Single wrapped - just frontend wrapper
  if (Array.isArray(wrappedData)) {
    return wrappedData as ActiveGroupWithId[];
  }

  return [];
}

/**
 * Builds a map of staff_id to their supervised groups for O(1) lookup
 */
function buildStaffGroupsMap(
  activeGroups: ActiveGroupWithId[],
): Record<string, SupervisedGroupEntryWithId[]> {
  const map: Record<string, SupervisedGroupEntryWithId[]> = {};

  for (const group of activeGroups) {
    for (const supervisor of group.supervisors ?? []) {
      if (supervisor.staff_id !== undefined) {
        const staffIdStr = supervisor.staff_id.toString();
        map[staffIdStr] ??= [];
        map[staffIdStr].push({
          group,
          role: supervisor.role,
          groupId: group.id,
        });
      }
    }
  }

  return map;
}

/** Absence type label mapping */
const absenceLabels: Record<string, string> = {
  sick: "Krank",
  vacation: "Urlaub",
  training: "Fortbildung",
  comp_time: "Freizeitausgleich",
  other: "Abwesend", // Shows red, same as "not clocked in"
};

/**
 * Determines location and supervision info for a staff member.
 *
 * Badge shows time clock status. Absence only shown when NOT clocked in.
 * Supervisions are returned separately as an array of rooms.
 *
 * Priority for currentLocation (badge):
 * 1. Time clock present → "Anwesend" (green)
 * 2. Time clock home_office → "Homeoffice" (blue)
 * 3. Not clocked in + absence → "Krank"/"Urlaub"/"Fortbildung" (gray), "other" → "Abwesend" (red)
 * 4. Legacy fallback (no work_status) → "Anwesend"
 * 5. Not clocked in, no absence → "Abwesend" (red)
 */
function getSupervisionInfo(
  staffId: string | undefined,
  staffGroupsMap: Record<string, SupervisedGroupEntryWithId[]>,
  wasPresentToday?: boolean,
  workStatus?: string,
  absenceType?: string,
): {
  isSupervising: boolean;
  currentLocation: string;
  supervisionRole?: string;
  supervisions: StaffSupervision[];
} {
  // Build supervisions array (independent of time clock status)
  const supervisedGroups = staffId ? staffGroupsMap[staffId] : undefined;
  const supervisions: StaffSupervision[] = [];
  let supervisionRole: string | undefined;
  let isSupervising = false;

  if (supervisedGroups) {
    isSupervising = true;
    for (const { group, role, groupId } of supervisedGroups) {
      if (group.room) {
        supervisions.push({
          roomId: group.room.id.toString(),
          roomName: group.room.name,
          activeGroupId: groupId?.toString() ?? "",
        });
      }
      supervisionRole ??= role;
    }
  }

  // Determine badge location (time clock status only)
  let currentLocation: string;

  // Priority 1: Time clock present → always wins
  if (workStatus === "present") {
    currentLocation = "Anwesend";
  }
  // Priority 2: Time clock home office → always wins
  else if (workStatus === "home_office") {
    currentLocation = "Homeoffice";
  }
  // Priority 3: Not clocked in - check for absence reason
  // (checked_out, no work_status, or legacy fallback)
  else if (absenceType && absenceLabels[absenceType]) {
    // Absence provides more detail on WHY they're absent
    currentLocation = absenceLabels[absenceType];
  }
  // Priority 4: Legacy fallback (only if NO work_status and NO absence)
  else if (wasPresentToday && !workStatus) {
    currentLocation = "Anwesend";
  }
  // Priority 5: Not present (checked out or never clocked in, no absence)
  else {
    currentLocation = "Abwesend";
  }

  return { isSupervising, currentLocation, supervisionRole, supervisions };
}

/**
 * Maps a backend staff response to frontend Staff type
 */
function mapStaffMember(
  staff: BackendStaffResponse,
  staffGroupsMap: Record<string, SupervisedGroupEntryWithId[]>,
): Staff {
  const { isSupervising, currentLocation, supervisionRole, supervisions } =
    getSupervisionInfo(
      staff.staff_id,
      staffGroupsMap,
      staff.was_present_today,
      staff.work_status,
      staff.absence_type,
    );

  return {
    id: staff.id,
    name: staff.name,
    firstName: staff.firstName,
    lastName: staff.lastName,
    email: undefined,
    role: staff.role ?? undefined,
    accountRole: staff.account_role ?? undefined,
    specialization: staff.specialization?.trim() ?? undefined,
    qualifications: staff.qualifications ?? undefined,
    staffNotes: staff.staff_notes ?? undefined,
    hasRfid: !!staff.tag_id,
    isTeacher: !!staff.teacher_id,
    isSupervising,
    currentLocation,
    supervisionRole,
    supervisions,
    wasPresentToday: staff.was_present_today,
    workStatus: staff.work_status,
    absenceType: staff.absence_type,
    absenceTypeLabel: staff.absence_type_label,
  };
}

/**
 * Applies client-side filters to staff list
 */
function applyStaffFilters(staff: Staff[], filters?: StaffFilters): Staff[] {
  let result = staff;

  if (filters?.status === "supervising") {
    result = result.filter((s) => s.isSupervising);
  } else if (filters?.status === "available") {
    result = result.filter((s) => !s.isSupervising);
  }

  if (filters?.type === "teachers") {
    result = result.filter((s) => s.isTeacher);
  } else if (filters?.type === "staff") {
    result = result.filter((s) => !s.isTeacher);
  }

  return result;
}

/**
 * Fetches active groups data, returning empty array on failure
 */
async function fetchActiveGroups(): Promise<ActiveGroupWithId[]> {
  try {
    const response = await sessionFetch("/api/active/groups?active=true");
    if (!response.ok) return [];
    const data = (await response.json()) as unknown;
    return extractActiveGroups(data);
  } catch {
    return [];
  }
}

// Staff service
class StaffService {
  async getDocumentDirectory(): Promise<StaffDocumentDirectoryEntry[]> {
    const response = await sessionFetch("/api/staff/documents-directory");
    if (!response.ok) {
      throw new Error(
        `Failed to fetch document directory: ${response.statusText}`,
      );
    }
    return ((await response.json()) as { data: StaffDocumentDirectoryEntry[] })
      .data;
  }

  // Get all staff members with their current supervision status.
  // `options.strict` opts into the /api/staff route's non-swallowing path so a
  // backend failure rejects here instead of masquerading as an empty staff list
  // (the lenient default returns [] on backend errors) — callers that must tell
  // "fetch failed" from "no staff" apart pass it (#1840).
  async getAllStaff(
    filters?: StaffFilters,
    options?: { strict?: boolean },
  ): Promise<Staff[]> {
    // Build staff URL with search filter
    let staffUrl = filters?.search
      ? `/api/staff?search=${encodeURIComponent(filters.search)}`
      : "/api/staff";
    if (options?.strict) {
      staffUrl += `${staffUrl.includes("?") ? "&" : "?"}strict=1`;
    }

    // Fetch staff and active groups in parallel
    const [staffResponse, activeGroups] = await Promise.all([
      sessionFetch(staffUrl),
      fetchActiveGroups(),
    ]);

    if (!staffResponse.ok) {
      throw new Error(`Failed to fetch staff: ${staffResponse.statusText}`);
    }

    const staffData = (await staffResponse.json()) as
      BackendStaffResponse[] | { data: BackendStaffResponse[] };
    const staffList = extractStaffList(staffData);
    const staffGroupsMap = buildStaffGroupsMap(activeGroups);

    const mappedStaff = staffList.map((staff) =>
      mapStaffMember(staff, staffGroupsMap),
    );

    return applyStaffFilters(mappedStaff, filters);
  }

  // Get a single staff member by ID
  async getStaffById(id: string): Promise<Staff> {
    const response = await sessionFetch(`/api/staff/${id}`);
    if (!response.ok) {
      throw new Error(`Failed to fetch staff member: ${response.statusText}`);
    }
    const json = (await response.json()) as {
      data: BackendStaffResponse & { employment_type?: string | null };
    };
    const staff = json.data;
    const { currentLocation, supervisionRole } = getSupervisionInfo(
      staff.staff_id,
      {},
      staff.was_present_today,
      staff.work_status,
      staff.absence_type,
    );
    return {
      id: staff.id,
      name: staff.name,
      firstName: staff.firstName ?? staff.name?.split(" ")[0] ?? "",
      lastName:
        staff.lastName ?? staff.name?.split(" ").slice(1).join(" ") ?? "",
      email: undefined,
      role: staff.role ?? undefined,
      accountRole: staff.account_role ?? undefined,
      specialization: staff.specialization?.trim() ?? undefined,
      qualifications: staff.qualifications ?? undefined,
      staffNotes: staff.staff_notes ?? undefined,
      employmentType: staff.employment_type ?? undefined,
      hasRfid: !!staff.tag_id,
      isTeacher: !!staff.teacher_id,
      isSupervising: false,
      currentLocation,
      supervisionRole,
      supervisions: [],
      wasPresentToday: staff.was_present_today,
      workStatus: staff.work_status,
      absenceType: staff.absence_type,
      absenceTypeLabel: staff.absence_type_label,
    };
  }

  async getFinancialProfile(id: string): Promise<Staff> {
    return this.getMinimalProfile(
      `/api/staff/financial-profile/${id}`,
      "financial",
    );
  }

  async getDocumentProfile(id: string): Promise<Staff> {
    return this.getMinimalProfile(
      `/api/staff/documents-profile/${id}`,
      "document",
    );
  }

  async getMinimalProfile(
    url: string,
    profileType: "financial" | "document",
  ): Promise<Staff> {
    const response = await sessionFetch(url);
    if (!response.ok) {
      throw new Error(
        `Failed to fetch ${profileType} staff profile: ${response.statusText}`,
      );
    }
    const json = (await response.json()) as {
      data: Omit<
        Pick<Staff, "id" | "name" | "firstName" | "lastName">,
        "id"
      > & { id: number };
    };
    return {
      ...json.data,
      id: json.data.id.toString(),
      hasRfid: false,
      isTeacher: false,
      isSupervising: false,
      supervisions: [],
      isFinancialProfile: profileType === "financial",
      isLimitedProfile: true,
    };
  }

  // Get active supervisions for a specific staff member
  async getStaffSupervisions(
    staffId: string,
  ): Promise<ActiveSupervisionResponse[]> {
    try {
      const response = await sessionFetch(
        `/api/active/supervisors/staff/${staffId}/active`,
      );

      if (!response.ok) {
        throw new Error(
          `Failed to fetch staff supervisions: ${response.statusText}`,
        );
      }

      const data = (await response.json()) as
        ActiveSupervisionResponse[] | { data: ActiveSupervisionResponse[] };

      if (Array.isArray(data)) {
        return data;
      } else if (
        data &&
        typeof data === "object" &&
        "data" in data &&
        Array.isArray(data.data)
      ) {
        return data.data;
      }

      return [];
    } catch (error) {
      logger.error("error fetching supervisions for staff", {
        staff_id: staffId,
        error: String(error),
      });
      return [];
    }
  }
}

// Work Schedule types and service
interface ScheduleEntry {
  weekIndex: number;
  dayOfWeek: number;
  targetMinutes: number;
  startTime?: string;
}

interface ScheduleModelInfo {
  id: string;
  name: string;
  rotationLength: number;
  rotationAnchorDate: string;
}

export interface StaffSchedule {
  mode: "template" | "custom";
  model: ScheduleModelInfo | null;
  rotationLength: number;
  rotationAnchorDate: string;
  entries: ScheduleEntry[];
  weeklyTotals: number[];
  // Earliest date on which any version of this schedule was in effect.
  // Anchored as YYYY-MM-DD. Empty string when no schedule rows exist.
  validFrom: string;
}

interface BackendScheduleEntry {
  week_index: number;
  day_of_week: number;
  target_minutes: number;
  start_time?: string | null;
}

interface BackendScheduleModel {
  id: number;
  name: string;
  rotation_length: number;
  rotation_anchor_date: string;
}

interface BackendScheduleResponse {
  mode: "template" | "custom";
  model: BackendScheduleModel | null;
  rotation_length: number;
  rotation_anchor_date: string;
  entries: BackendScheduleEntry[];
  weekly_totals: number[];
  valid_from?: string;
}

function mapScheduleResponse(data: BackendScheduleResponse): StaffSchedule {
  return {
    mode: data.mode ?? "custom",
    model: data.model
      ? {
          id: data.model.id.toString(),
          name: data.model.name,
          rotationLength: data.model.rotation_length,
          rotationAnchorDate: data.model.rotation_anchor_date,
        }
      : null,
    rotationLength: data.rotation_length ?? 1,
    rotationAnchorDate: data.rotation_anchor_date ?? "",
    entries: (data.entries ?? []).map((e) => ({
      weekIndex: e.week_index,
      dayOfWeek: e.day_of_week,
      targetMinutes: e.target_minutes,
      startTime: e.start_time ?? undefined,
    })),
    weeklyTotals: data.weekly_totals ?? [],
    validFrom: (data.valid_from ?? "").slice(0, 10),
  };
}

interface UpdateScheduleCustomRequest {
  mode: "custom";
  rotationLength: number;
  rotationAnchorDate?: string;
  entries: Array<{
    weekIndex: number;
    dayOfWeek: number;
    targetMinutes: number;
    startTime?: string;
  }>;
  saveAsTemplate?: string;
}

interface UpdateScheduleTemplateRequest {
  mode: "template";
  modelId: string;
}

export type UpdateScheduleRequest =
  UpdateScheduleCustomRequest | UpdateScheduleTemplateRequest;

class StaffScheduleService {
  async getSchedule(staffId: string): Promise<StaffSchedule> {
    const response = await sessionFetch(`/api/staff/${staffId}/schedule`);
    const json = (await response.json()) as {
      data: BackendScheduleResponse;
    };
    return mapScheduleResponse(json.data);
  }

  async updateSchedule(
    staffId: string,
    update: UpdateScheduleRequest,
  ): Promise<StaffSchedule> {
    const body =
      update.mode === "template"
        ? { mode: "template", model_id: Number.parseInt(update.modelId, 10) }
        : {
            mode: "custom",
            rotation_length: update.rotationLength,
            rotation_anchor_date: update.rotationAnchorDate,
            entries: update.entries.map((e) => ({
              week_index: e.weekIndex,
              day_of_week: e.dayOfWeek,
              target_minutes: e.targetMinutes,
              start_time: e.startTime,
            })),
            save_as_template: update.saveAsTemplate,
          };
    const response = await sessionFetch(`/api/staff/${staffId}/schedule`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const json = (await response.json()) as {
      data: BackendScheduleResponse;
    };
    return mapScheduleResponse(json.data);
  }
}

// Work Time Models (tenant templates)
interface WorkTimeModelEntry {
  weekIndex: number;
  dayOfWeek: number;
  targetMinutes: number;
  startTime?: string;
}

export interface WorkTimeModel {
  id: string;
  name: string;
  rotationLength: number;
  rotationAnchorDate: string;
  entries: WorkTimeModelEntry[];
  weeklyTotals: number[];
}

interface BackendWorkTimeModel {
  id: number;
  name: string;
  rotation_length: number;
  rotation_anchor_date: string;
  entries: BackendScheduleEntry[];
  weekly_totals: number[];
}

function mapWorkTimeModel(m: BackendWorkTimeModel): WorkTimeModel {
  return {
    id: m.id.toString(),
    name: m.name,
    rotationLength: m.rotation_length,
    rotationAnchorDate: m.rotation_anchor_date,
    entries: (m.entries ?? []).map((e) => ({
      weekIndex: e.week_index,
      dayOfWeek: e.day_of_week,
      targetMinutes: e.target_minutes,
      startTime: e.start_time ?? undefined,
    })),
    weeklyTotals: m.weekly_totals ?? [],
  };
}

class WorkTimeModelService {
  async list(): Promise<WorkTimeModel[]> {
    const response = await sessionFetch(`/api/work-time-models`);
    const json = (await response.json()) as { data: BackendWorkTimeModel[] };
    return (json.data ?? []).map(mapWorkTimeModel);
  }
}

// Staff history types (reuses time-tracking-helpers types)
// One break of a work session. Narrowed to the fields the metric helpers read
// from the wire (SessionResponse.Breaks); the payload carries more.
interface StaffSessionBreak {
  started_at: string;
  ended_at: string | null;
}

export interface StaffHistorySession {
  id?: number;
  date: string;
  status?: "present" | "home_office";
  // Channel the row was created on. `app` = self-service Web/App,
  // `nfc` = kiosk auto-stamp, `unknown` = pre-migration legacy row.
  // Comes from active.work_sessions.source (Issue #1368).
  source?: "app" | "nfc" | "unknown";
  net_minutes: number;
  check_in_time: string;
  check_out_time: string | null;
  break_minutes: number;
  // Break rows for display. NOT needed to correct `net_minutes`: the server
  // already deducts a running break from it (#1842).
  breaks?: readonly StaffSessionBreak[] | null;
  auto_checked_out?: boolean;
  notes?: string;
  edit_count?: number;
  audit_count?: number;
}

class StaffHistoryService {
  async getHistory(
    staffId: string,
    from: string,
    to: string,
  ): Promise<StaffHistorySession[]> {
    const params = new URLSearchParams({ from, to });
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/history?${params}`,
    );
    if (!response.ok) {
      throw new Error(`Failed to fetch staff history: ${response.statusText}`);
    }
    const json = (await response.json()) as {
      data: { sessions: StaffHistorySession[] };
    };
    return json.data.sessions ?? [];
  }
}

// Admin counterpart to /api/time-tracking/absences (which is self-scoped).
// Backend response shape matches BackendStaffAbsence in time-tracking-helpers.ts.
export interface StaffAbsenceRow {
  id: number;
  staff_id: number;
  absence_type: string;
  /** School-defined Abwesenheitsart (#2403); absent for the standard types. */
  absence_type_id?: string | null;
  /** The school's own wording; empty for the standard types. */
  absence_type_label?: string;
  date_start: string;
  date_end: string;
  half_day: boolean;
  start_half_day?: boolean;
  end_half_day?: boolean;
  note: string;
  status: string;
  approved_by?: number | null;
  approved_at?: string | null;
  decision_note?: string;
  working_days?: number | null;
  requested_at?: string;
  duration_days?: number;
}

// Eine Zeile des Anfragen-Moduls (#2433): der Antrag plus die Namen, die die
// Liste anzeigt. Leere Namen heißen "Unbekannt" (gelöschtes Konto).
export interface StaffAbsenceRequestRow extends Omit<
  StaffAbsenceRow,
  "id" | "staff_id" | "approved_by"
> {
  id: string;
  staff_id: string;
  approved_by?: string | null;
  staff_name: string;
  decided_by_name?: string;
  /** Zeitpunkt der letzten Änderung; trägt bei zurückgezogenen Anträgen das
   *  Rücknahme-Datum, für das es kein eigenes Feld gibt. */
  updated_at?: string;
}

// Vacation takeover at the moto introduction (#2132): days already taken
// before the Stichtag in the previous system. The row is its own audit
// record (Stichtag, entered Resturlaub, note, actor).
export interface StaffVacationOpening {
  id: string;
  staff_id: string;
  year: number;
  effective_date: string;
  taken_before_days: number;
  entered_remaining_days: number;
  note: string;
  decided_by: string;
  decided_at: string;
}

interface BackendStaffVacationOpening extends Omit<
  StaffVacationOpening,
  "id" | "staff_id" | "decided_by"
> {
  id: number;
  staff_id: number;
  decided_by: number;
}

function mapStaffVacationOpening(
  data: BackendStaffVacationOpening,
): StaffVacationOpening {
  return {
    ...data,
    id: data.id.toString(),
    staff_id: data.staff_id.toString(),
    decided_by: data.decided_by.toString(),
  };
}

export interface StaffVacationQuotaSummary {
  staff_id: number;
  year: number;
  entitled_days: number;
  carryover_days: number;
  taken_before_days: number;
  taken_days: number;
  reserved_days: number;
  remaining_days: number;
  opening?: StaffVacationOpening | null;
}

interface BackendStaffVacationQuotaSummary extends Omit<
  StaffVacationQuotaSummary,
  "opening"
> {
  opening?: BackendStaffVacationOpening | null;
}

function mapStaffVacationQuotaSummary(
  data: BackendStaffVacationQuotaSummary,
): StaffVacationQuotaSummary {
  return {
    ...data,
    opening: data.opening
      ? mapStaffVacationOpening(data.opening)
      : data.opening,
  };
}

// Body for the admin "Abwesenheit anlegen" endpoint (POST
// /api/staff/{id}/absences). Keys are snake_case to match the backend
// CreateAbsenceRequest one-to-one; the route handler forwards them verbatim.
// The backend struct only knows `half_day` (no per-boundary flags), so those
// are intentionally absent — sending them would be silently dropped (#1843).
// A `sick` absence triggers the Dienst-/Betreuungsplan cascade backend-side.
interface AdminCreateAbsenceBody {
  absence_type: string;
  date_start: string; // YYYY-MM-DD
  date_end: string; // YYYY-MM-DD
  half_day?: boolean;
  note?: string;
}

class StaffAbsenceService {
  async getAbsences(
    staffId: string,
    from: string,
    to: string,
  ): Promise<StaffAbsenceRow[]> {
    const params = new URLSearchParams({ from, to });
    const response = await sessionFetch(
      `/api/staff/${staffId}/absences?${params}`,
    );
    if (!response.ok) {
      throw new Error(`Failed to fetch staff absences: ${response.statusText}`);
    }
    const json = (await response.json()) as {
      data: StaffAbsenceRow[] | null;
    };
    return json.data ?? [];
  }

  async getVacationQuota(
    staffId: string,
    year?: number,
  ): Promise<StaffVacationQuotaSummary> {
    const qs = year ? `?year=${year}` : "";
    const response = await sessionFetch(
      `/api/staff/${staffId}/vacation/quota${qs}`,
    );
    if (!response.ok) {
      throw new Error(`Failed to fetch quota: ${response.statusText}`);
    }
    const json = (await response.json()) as {
      data: BackendStaffVacationQuotaSummary;
    };
    return mapStaffVacationQuotaSummary(json.data);
  }

  async setVacationQuota(
    staffId: string,
    payload: {
      year: number;
      entitled_days: number;
      carryover_days: number;
    },
  ): Promise<StaffVacationQuotaSummary> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/vacation/quota`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      },
    );
    if (!response.ok) {
      throw new Error(`Failed to save quota: ${response.statusText}`);
    }
    const json = (await response.json()) as {
      data: BackendStaffVacationQuotaSummary;
    };
    return mapStaffVacationQuotaSummary(json.data);
  }

  // Vacation takeover (#2132): the admin enters the Resturlaub as of the
  // Stichtag; the backend derives the pre-introduction days from the quota.
  async setVacationOpening(
    staffId: string,
    payload: {
      effectiveDate: string;
      remainingDays: number;
      note: string;
    },
  ): Promise<StaffVacationOpening> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/vacation/opening`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          effective_date: payload.effectiveDate,
          remaining_days: payload.remainingDays,
          note: payload.note,
        }),
      },
    );
    if (!response.ok) {
      const error = await readStaffAPIError(
        response,
        "Übernahme fehlgeschlagen",
      );
      if (error.code === "vacation_opening_already_exists") {
        throw new Error(
          "Für dieses Jahr existiert bereits eine Urlaubs-Übernahme. Lösche zuerst die bestehende Übernahme.",
        );
      }
      if (error.code === "vacation_opening_absences_before_cutoff") {
        throw new Error(
          "Es existieren bereits Urlaubs-Abwesenheiten vor dem Stichtag. Die Übernahme würde diese Tage doppelt zählen.",
        );
      }
      throw new Error(error.message);
    }
    const json = (await response.json()) as {
      data: BackendStaffVacationOpening;
    };
    return mapStaffVacationOpening(json.data);
  }

  async deleteVacationOpening(staffId: string, year: number): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/vacation/opening?year=${year}`,
      { method: "DELETE" },
    );
    if (!response.ok) {
      const error = await readStaffAPIError(response, "Löschen fehlgeschlagen");
      throw new Error(error.message);
    }
  }

  async approve(
    absenceId: number | string,
    decisionNote?: string,
  ): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/absences/${absenceId}/approve`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ decision_note: decisionNote ?? "" }),
      },
    );
    if (!response.ok) {
      const text = await response.text().catch(() => "");
      throw new Error(text || "Genehmigung fehlgeschlagen");
    }
  }

  // Tenant-wide open requests (status requested + question) for the /staff
  // inbox (#1419). Response rows carry only staff_id — callers join names
  // against the already-fetched staff list client-side.
  async listPending(): Promise<StaffAbsenceRow[]> {
    const response = await sessionFetch(`/api/staff/absences/pending`);
    if (!response.ok) {
      throw new Error(
        `Failed to fetch pending absences: ${response.statusText}`,
      );
    }
    const json = (await response.json()) as { data: StaffAbsenceRow[] | null };
    return json.data ?? [];
  }

  // Anfragen-Modul, Reiter Mitarbeitende (#2433): offene Anträge oder die
  // Historie der entschiedenen, jeweils mit den Namen, die die Liste zeigt.
  // Suche und Art-Filter wirken serverseitig.
  async listRequests(
    view: "open" | "history",
    filters: { search?: string; types?: readonly string[] } = {},
  ): Promise<StaffAbsenceRequestRow[]> {
    const params = new URLSearchParams({ view });
    if (filters.search?.trim()) params.set("search", filters.search.trim());
    if (filters.types && filters.types.length > 0)
      params.set("types", filters.types.join(","));
    const response = await sessionFetch(
      `/api/staff/absences/requests?${params.toString()}`,
    );
    if (!response.ok) {
      throw new Error(
        `Failed to fetch absence requests: ${response.statusText}`,
      );
    }
    const json = (await response.json()) as {
      data: StaffAbsenceRequestRow[] | null;
    };
    return json.data ?? [];
  }

  // Rückfrage: moves a requested absence into status "question" with a
  // mandatory note from the Leitung (#1419).
  async question(
    absenceId: number | string,
    decisionNote: string,
  ): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/absences/${absenceId}/question`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ decision_note: decisionNote }),
      },
    );
    if (!response.ok) {
      const text = await response.text().catch(() => "");
      throw new Error(text || "Rückfrage fehlgeschlagen");
    }
  }

  async deny(absenceId: number | string, decisionNote: string): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/absences/${absenceId}/deny`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ decision_note: decisionNote }),
      },
    );
    if (!response.ok) {
      const text = await response.text().catch(() => "");
      throw new Error(text || "Ablehnung fehlgeschlagen");
    }
  }

  // Admin-side "Abwesenheit anlegen" (POST /api/staff/{id}/absences). A
  // `sick` absence cancels planned shifts and marks Betreuungsblöcke as
  // absent server-side (#1843). Returns the created StaffAbsence row.
  async createAbsence(
    staffId: string,
    body: AdminCreateAbsenceBody,
  ): Promise<StaffAbsenceRow> {
    const response = await sessionFetch(`/api/staff/${staffId}/absences`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      const error = await readStaffAPIError(
        response,
        "Krankmeldung fehlgeschlagen",
      );
      if (error.code === "comp_time_exceeds_balance") {
        throw new Error(
          "Der Freizeitausgleich übersteigt die vor dem Startdatum verfügbaren Plus-Stunden.",
        );
      }
      throw new Error(error.message);
    }
    const json = (await response.json()) as { data: StaffAbsenceRow };
    return json.data;
  }

  // Deletes an absence (DELETE /api/staff/{id}/absences/{absenceId}).
  // Deleting a `sick` absence reverses the plan cascade backend-side.
  async deleteAbsence(
    staffId: string,
    absenceId: number | string,
  ): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/absences/${absenceId}`,
      { method: "DELETE" },
    );
    if (!response.ok) {
      const text = await response.text().catch(() => "");
      throw new Error(text || "Löschen fehlgeschlagen");
    }
  }
}

// Admin counterpart to timeTrackingService.getSessionEdits (which checks
// ownership against the JWT subject). Routes via /api/staff/{id}/.../edits
// so an admin can pull the audit trail for any staff member's session in
// the tenant.
import { parseISODate, toISODate } from "./date-helpers";
import {
  MAX_TARGET_RANGE_DAYS,
  mapBalanceAdjustmentResponse,
  mapDailyTargetsResponse,
  mapMonthCloseResultResponse,
  mapMonthCloseSnapshotResponse,
  mapMonthSummaryResponse,
  mapWorkSessionEditResponse,
  type BackendBalanceAdjustment,
  type BackendDailyTarget,
  type BackendMonthCloseResult,
  type BackendMonthCloseSnapshot,
  type BackendMonthSummary,
  type BackendWorkSessionEdit,
  type BalanceAdjustment,
  type MonthCloseResult,
  type MonthCloseSnapshot,
  type MonthSummary,
  type WorkSessionEdit,
} from "./time-tracking-helpers";

// Payload shape for admin edits and nachgetragene sessions. The backend
// keys (date / check_in_time / ...) match SessionUpdateRequest and
// AdminCreateSessionRequest one-to-one. The route handler decides which
// it is via the HTTP verb (PUT vs POST). Notes is required for both flows.
interface AdminSessionPayload {
  date: string; // YYYY-MM-DD
  check_in_time: string; // ISO 8601
  check_out_time: string; // ISO 8601
  break_minutes: number;
  status: "present" | "home_office";
  notes: string;
}

class StaffSessionEditsService {
  async getEdits(
    staffId: string,
    sessionId: string,
  ): Promise<WorkSessionEdit[]> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/sessions/${sessionId}/edits`,
    );
    if (!response.ok) {
      throw new Error(`Failed to fetch session edits: ${response.statusText}`);
    }
    const json = (await response.json()) as {
      data: BackendWorkSessionEdit[] | null;
    };
    return (json.data ?? []).map(mapWorkSessionEditResponse);
  }
}

// Monatskarte (#1842), live-computed. Admin counterpart to
// timeTrackingService.getMonthSummary.
class StaffMonthSummaryService {
  async getMonthSummary(
    staffId: string,
    year: number,
    month: number,
  ): Promise<MonthSummary> {
    const params = new URLSearchParams({
      year: String(year),
      month: String(month),
    });
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/month-summary?${params}`,
    );
    if (!response.ok) {
      throw new Error(`Failed to fetch month summary: ${response.statusText}`);
    }
    const json = (await response.json()) as { data: BackendMonthSummary };
    return mapMonthSummaryResponse(json.data);
  }

  // Admin counterpart to timeTrackingService.getScheduleTargets — the same
  // date-valid Soll the Monatskarte is computed from, so card and table can
  // never disagree after a schedule change (#1842).
  async getScheduleTargets(
    staffId: string,
    from: string,
    to: string,
  ): Promise<ReadonlyMap<string, number>> {
    const params = new URLSearchParams({ from, to });
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/schedule-targets?${params}`,
    );
    if (!response.ok) {
      throw new Error(
        `Failed to fetch schedule targets: ${response.statusText}`,
      );
    }
    const json = (await response.json()) as {
      data: BackendDailyTarget[] | null;
    };
    return mapDailyTargetsResponse(json.data);
  }

  /**
   * Same date-valid Soll as `getScheduleTargets`, for a range that may be
   * longer than the endpoint's MAX_TARGET_RANGE_DAYS window. The Übersicht
   * charts reach back to the configured account start ("Gesamt"), which can be
   * years — one request would simply 400. The windows are fetched with bounded
   * concurrency and merged; days repeat across no two windows, so the merge is
   * a plain union.
   */
  async getScheduleTargetsRange(
    staffId: string,
    from: string,
    to: string,
  ): Promise<ReadonlyMap<string, number>> {
    // Windows are MAX_TARGET_RANGE_DAYS calendar days INCLUSIVE — one day
    // inside the endpoint's cap, which counts the gap between the bounds
    // rather than the days. Nothing rides on the last day of headroom, and a
    // whole-window stride keeps the arithmetic obvious.
    const windows: { from: string; to: string }[] = [];
    const end = parseISODate(to);
    for (
      let cursor = parseISODate(from);
      cursor <= end;
      cursor = addDays(cursor, MAX_TARGET_RANGE_DAYS)
    ) {
      const windowEnd = addDays(cursor, MAX_TARGET_RANGE_DAYS - 1);
      windows.push({
        from: toISODate(cursor),
        to: toISODate(windowEnd > end ? end : windowEnd),
      });
    }

    // Bound the fan-out: a "Gesamt" range on a years-old account produces one
    // window per ~year, and a plain Promise.all would fire them all at once —
    // one overview render then opens dozens of simultaneous DB-backed requests
    // per staff member. A small pool keeps the endpoint (and its RLS-scoped
    // queries) from being hammered while still overlapping the round-trips.
    const chunks = await mapWithConcurrency(
      windows,
      MAX_TARGET_RANGE_CONCURRENCY,
      (w) => this.getScheduleTargets(staffId, w.from, w.to),
    );
    const merged = new Map<string, number>();
    for (const chunk of chunks) {
      for (const [day, minutes] of chunk) merged.set(day, minutes);
    }
    return merged;
  }
}

// Upper bound on schedule-target windows fetched at once by
// getScheduleTargetsRange. Four keeps the round-trips overlapping without
// letting a long-lived account open a dozen+ concurrent DB-backed requests.
const MAX_TARGET_RANGE_CONCURRENCY = 4;

// Runs `worker` over `items` with at most `limit` in flight at any time,
// preserving input order in the result. A fixed pool of `limit` runners pulls
// from a shared cursor, so the total number of concurrent calls never exceeds
// `limit` regardless of how many items there are.
async function mapWithConcurrency<T, R>(
  items: readonly T[],
  limit: number,
  worker: (item: T) => Promise<R>,
): Promise<R[]> {
  const results = new Array<R>(items.length);
  let next = 0;
  const run = async (): Promise<void> => {
    for (;;) {
      const index = next++;
      if (index >= items.length) return;
      results[index] = await worker(items[index]!);
    }
  };
  const runners = Array.from({ length: Math.min(limit, items.length) }, () =>
    run(),
  );
  await Promise.all(runners);
  return results;
}

function addDays(d: Date, days: number): Date {
  const next = new Date(d);
  next.setDate(next.getDate() + days);
  return next;
}

class StaffSessionService {
  // PUT corrects an existing session on behalf of the named staff member.
  // Backend route: /api/staff/{staffId}/time-tracking/sessions/{sessionId}
  async updateSession(
    staffId: string,
    sessionId: string,
    payload: AdminSessionPayload,
  ): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/sessions/${sessionId}`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      },
    );
    if (!response.ok) {
      const body = await response.text().catch(() => "");
      throw new Error(
        body || `Failed to update session: ${response.statusText}`,
      );
    }
  }

  // POST admin "nachträgt" a session for a staff member who forgot to
  // stamp. Backend route: /api/staff/{staffId}/time-tracking/sessions
  async createSession(
    staffId: string,
    payload: AdminSessionPayload,
  ): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/sessions`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      },
    );
    if (!response.ok) {
      const body = await response.text().catch(() => "");
      throw new Error(
        body || `Failed to create session: ${response.statusText}`,
      );
    }
  }
}

interface StaffAPIError {
  readonly code?: string;
  readonly message: string;
}

async function readStaffAPIError(
  response: Response,
  fallback: string,
): Promise<StaffAPIError> {
  const text = await response.text().catch(() => "");
  if (!text) return { message: fallback };

  try {
    const payload = JSON.parse(text) as {
      code?: unknown;
      error?: unknown;
      message?: unknown;
    };
    return {
      code: typeof payload.code === "string" ? payload.code : undefined,
      message:
        typeof payload.error === "string"
          ? payload.error
          : typeof payload.message === "string"
            ? payload.message
            : fallback,
    };
  } catch {
    return { message: text };
  }
}

// Stundenkonto lifecycle (#1420): payout / comp-time transactions and the
// school-year reset. Admin-only (time_tracking:manage, enforced backend-side).
class StaffBalanceAdjustmentService {
  async list(
    staffId: string,
    from: string,
    to: string,
  ): Promise<BalanceAdjustment[]> {
    const params = new URLSearchParams({ from, to });
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/adjustments?${params}`,
    );
    if (!response.ok) {
      throw new Error(`Failed to fetch adjustments: ${response.statusText}`);
    }
    const json = (await response.json()) as {
      data: BackendBalanceAdjustment[] | null;
    };
    return (json.data ?? []).map(mapBalanceAdjustmentResponse);
  }

  // minutesDelta is signed and negative — payout and comp-time grants only
  // ever reduce the Stundenkonto.
  async create(
    staffId: string,
    payload: {
      type: "payout" | "comp_time";
      minutesDelta: number;
      effectiveDate: string;
      note: string;
    },
  ): Promise<BalanceAdjustment> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/adjustments`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          type: payload.type,
          minutes_delta: payload.minutesDelta,
          effective_date: payload.effectiveDate,
          note: payload.note,
        }),
      },
    );
    if (!response.ok) {
      const error = await readStaffAPIError(response, "Buchung fehlgeschlagen");
      if (error.code === "dependent_balance_reset") {
        throw new Error(
          "Die Buchung liegt vor einem vorhandenen Reset und würde dessen Saldo verfälschen.",
        );
      }
      if (error.code === "balance_adjustment_exceeds_balance") {
        throw new Error(
          "Die Buchung übersteigt die zum gewählten Datum verfügbaren Plus-Stunden.",
        );
      }
      if (error.code === "adjustment_in_closed_month") {
        throw new Error(
          "Der gewählte Monat ist abgeschlossen. Buche die Korrektur mit einem Datum im offenen Monat oder öffne den Monatsabschluss wieder.",
        );
      }
      throw new Error(error.message);
    }
    const json = (await response.json()) as { data: BackendBalanceAdjustment };
    return mapBalanceAdjustmentResponse(json.data);
  }

  async delete(staffId: string, adjustmentId: string): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/adjustments/${adjustmentId}`,
      { method: "DELETE" },
    );
    if (!response.ok) {
      const error = await readStaffAPIError(response, "Löschen fehlgeschlagen");
      if (error.code === "dependent_balance_reset") {
        throw new Error(
          "Die Buchung liegt vor einem vorhandenen Reset und kann deshalb nicht gelöscht werden.",
        );
      }
      if (error.code === "balance_adjustment_exceeds_balance") {
        throw new Error(
          "Die Buchung kann nicht gelöscht werden, weil spätere Abzüge vom dadurch entstehenden Guthaben abhängen.",
        );
      }
      if (error.code === "adjustment_in_closed_month") {
        throw new Error(
          "Der gewählte Monat ist abgeschlossen. Öffne den Monatsabschluss wieder, bevor du die Buchung löschst.",
        );
      }
      throw new Error(error.message);
    }
  }

  // The backend computes the closing balance as of effectiveDate under a
  // per-staff lock and writes the inverting transaction; a repeated reset
  // for the same date returns 409.
  async reset(
    staffId: string,
    payload: {
      effectiveDate: string;
      carryoverMinutes: number;
      note: string;
    },
  ): Promise<BalanceAdjustment> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/reset`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          effective_date: payload.effectiveDate,
          carryover_minutes: payload.carryoverMinutes,
          note: payload.note,
        }),
      },
    );
    if (!response.ok) {
      const error = await readStaffAPIError(response, "Reset fehlgeschlagen");
      if (error.code === "balance_already_reset") {
        throw new Error(
          "Das Stundenkonto wurde für dieses Datum bereits zurückgesetzt.",
        );
      }
      if (error.code === "dependent_balance_reset") {
        throw new Error(
          "Der Reset liegt vor einem späteren Reset und würde dessen Saldo verfälschen.",
        );
      }
      if (error.code === "balance_adjustment_exceeds_balance") {
        throw new Error(
          "Der Reset kann nicht durchgeführt werden, weil spätere Buchungen oder Freizeitausgleichstage vom aktuellen Guthaben abhängen.",
        );
      }
      if (error.code === "adjustment_in_closed_month") {
        throw new Error(
          "Der gewählte Monat ist abgeschlossen. Wähle ein Datum im offenen Monat oder öffne den Monatsabschluss wieder.",
        );
      }
      throw new Error(error.message);
    }
    const json = (await response.json()) as { data: BackendBalanceAdjustment };
    return mapBalanceAdjustmentResponse(json.data);
  }

  // Eröffnungssaldo (#2132): sets the Stundenkonto to a SIGNED target value
  // as of the Stichtag — the only booking that may take the account negative
  // (takeover from the previous system). One opening per person, ever.
  async createOpening(
    staffId: string,
    payload: {
      effectiveDate: string;
      balanceMinutes: number;
      note: string;
    },
  ): Promise<BalanceAdjustment> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/opening`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          effective_date: payload.effectiveDate,
          balance_minutes: payload.balanceMinutes,
          note: payload.note,
        }),
      },
    );
    if (!response.ok) {
      const error = await readStaffAPIError(
        response,
        "Eröffnungssaldo fehlgeschlagen",
      );
      if (error.code === "opening_balance_already_exists") {
        throw new Error(
          "Für diese Person existiert bereits ein Eröffnungssaldo. Lösche zuerst die bestehende Buchung.",
        );
      }
      if (error.code === "dependent_balance_reset") {
        throw new Error(
          "Es existieren bereits spätere Buchungen (Reset), die vom Stichtag abhängen.",
        );
      }
      if (error.code === "adjustment_in_closed_month") {
        throw new Error(
          "Der gewählte Monat ist abgeschlossen. Wähle ein Datum im offenen Monat oder öffne den Monatsabschluss wieder.",
        );
      }
      throw new Error(error.message);
    }
    const json = (await response.json()) as { data: BackendBalanceAdjustment };
    return mapBalanceAdjustmentResponse(json.data);
  }
}

// Monatsabschluss (#1417): school-wide freeze of a month's closing balances,
// per-staff reopen. The German copy for the stable error codes lives here so
// every caller explains the same rules the same way.
class StaffMonthCloseService {
  /** Active snapshots of one month for the whole school; [] = not closed. */
  async getStatus(year: number, month: number): Promise<MonthCloseSnapshot[]> {
    const params = new URLSearchParams({
      year: String(year),
      month: String(month),
    });
    const response = await sessionFetch(
      `/api/staff/time-tracking/month-close?${params}`,
    );
    if (!response.ok) {
      throw new Error(
        `Failed to fetch month close status: ${response.statusText}`,
      );
    }
    const json = (await response.json()) as {
      data: BackendMonthCloseSnapshot[] | null;
    };
    return (json.data ?? []).map(mapMonthCloseSnapshotResponse);
  }

  /** Freezes the month for the whole school in one transaction. */
  async closeMonth(payload: {
    year: number;
    month: number;
    reason: string;
  }): Promise<MonthCloseResult> {
    const response = await sessionFetch(
      "/api/staff/time-tracking/month-close",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          year: payload.year,
          month: payload.month,
          reason: payload.reason,
        }),
      },
    );
    if (!response.ok) {
      const error = await readStaffAPIError(
        response,
        "Monatsabschluss fehlgeschlagen",
      );
      if (error.code === "month_not_closable") {
        throw new Error(
          "Dieser Monat kann noch nicht abgeschlossen werden: Er ist noch nicht vorbei. Der Abschluss friert den Stand zum Monatsende ein; für einen laufenden Monat gibt es diesen Stand noch nicht.",
        );
      }
      if (error.code === "later_month_closed") {
        throw new Error(
          "Ein späterer Monat ist bereits abgeschlossen. Monate werden in Reihenfolge abgeschlossen; öffne zuerst den späteren Abschluss.",
        );
      }
      throw new Error(error.message);
    }
    const json = (await response.json()) as { data: BackendMonthCloseResult };
    return mapMonthCloseResultResponse(json.data);
  }

  /** Reopens one staff member's closed month; reason is mandatory. */
  async reopenMonth(
    staffId: string,
    payload: { year: number; month: number; reason: string },
  ): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/time-tracking/month-close/reopen`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          year: payload.year,
          month: payload.month,
          reason: payload.reason,
        }),
      },
    );
    if (!response.ok) {
      const error = await readStaffAPIError(
        response,
        "Wiedereröffnung fehlgeschlagen",
      );
      if (error.code === "month_not_closed") {
        throw new Error("Dieser Monat ist nicht abgeschlossen.");
      }
      if (error.code === "later_month_closed") {
        throw new Error(
          "Für diese Person ist ein späterer Monat noch abgeschlossen. Abschlüsse werden vom neuesten zum ältesten geöffnet; öffne zuerst den späteren Monat.",
        );
      }
      throw new Error(error.message);
    }
  }
}

// Personalnummer (#1417): payroll identifier maintained on the Stammdaten
// tab. time_tracking:manage backend-side — callers gate rendering on the
// permission so no request fires without it.
class StaffPayrollNumberService {
  async get(staffId: string): Promise<string | null> {
    const response = await sessionFetch(`/api/staff/${staffId}/payroll-number`);
    if (!response.ok) {
      throw new Error(`Failed to fetch payroll number: ${response.statusText}`);
    }
    const json = (await response.json()) as {
      data: { personnel_number: string | null };
    };
    return json.data.personnel_number;
  }

  async update(
    staffId: string,
    personnelNumber: string | null,
    note: string,
  ): Promise<string | null> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/payroll-number`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ personnel_number: personnelNumber, note }),
      },
    );
    if (!response.ok) {
      const error = await readStaffAPIError(
        response,
        "Personalnummer konnte nicht gespeichert werden",
      );
      if (error.code === "personnel_number_taken") {
        throw new Error(
          "Diese Personalnummer ist in dieser Schule bereits vergeben.",
        );
      }
      if (error.code === "personnel_number_invalid") {
        throw new Error(
          "Ungültige Personalnummer: nur Ziffern, höchstens 9 Stellen.",
        );
      }
      throw new Error(error.message);
    }
    const json = (await response.json()) as {
      data: { personnel_number: string | null };
    };
    return json.data.personnel_number;
  }
}

// --- Stammdaten (#1423) -----------------------------------------------------

export type StammdatenGender = "female" | "male" | "diverse";

export interface StammdatenPerson {
  firstName: string;
  lastName: string;
  /** "YYYY-MM-DD" or null */
  birthday: string | null;
  gender: StammdatenGender | null;
}

export interface StammdatenKontakt {
  addressStreet: string | null;
  addressPostalCode: string | null;
  addressCity: string | null;
  phone: string | null;
  email: string | null;
  emergencyContactName: string | null;
  emergencyContactPhone: string | null;
}

export interface StammdatenArbeitsvertrag {
  /** "YYYY-MM-DD" or null */
  entryDate: string | null;
  contractEndDate: string | null;
  probationEndDate: string | null;
  weeklyHours: number | null;
  employmentType: string | null;
}

export interface StammdatenQualifikation {
  id: string | null;
  name: string;
  acquiredOn: string | null;
  expiresOn: string | null;
}

export interface StaffStammdaten {
  staffId: string;
  person: StammdatenPerson;
  kontakt: StammdatenKontakt;
  arbeitsvertrag: StammdatenArbeitsvertrag;
  qualifikationen: StammdatenQualifikation[];
}

interface StaffFinancialMasked {
  ibanMasked: string | null;
  taxIdMasked: string | null;
  socialSecurityNumberMasked: string | null;
}

export interface StaffFinancialPlain {
  iban: string | null;
  taxId: string | null;
  socialSecurityNumber: string | null;
}

interface BackendStammdaten {
  staff_id: number;
  person: {
    first_name: string;
    last_name: string;
    birthday: string | null;
    gender: StammdatenGender | null;
  };
  kontakt: {
    address_street: string | null;
    address_postal_code: string | null;
    address_city: string | null;
    phone: string | null;
    email: string | null;
    emergency_contact_name: string | null;
    emergency_contact_phone: string | null;
  };
  arbeitsvertrag: {
    entry_date: string | null;
    contract_end_date: string | null;
    probation_end_date: string | null;
    weekly_hours: number | null;
    employment_type: string | null;
  };
  qualifikationen: {
    id?: number;
    name: string;
    acquired_on: string | null;
    expires_on: string | null;
  }[];
}

function mapStammdatenResponse(data: BackendStammdaten): StaffStammdaten {
  return {
    staffId: data.staff_id.toString(),
    person: {
      firstName: data.person.first_name,
      lastName: data.person.last_name,
      birthday: data.person.birthday,
      gender: data.person.gender,
    },
    kontakt: {
      addressStreet: data.kontakt.address_street,
      addressPostalCode: data.kontakt.address_postal_code,
      addressCity: data.kontakt.address_city,
      phone: data.kontakt.phone,
      email: data.kontakt.email,
      emergencyContactName: data.kontakt.emergency_contact_name,
      emergencyContactPhone: data.kontakt.emergency_contact_phone,
    },
    arbeitsvertrag: {
      entryDate: data.arbeitsvertrag.entry_date,
      contractEndDate: data.arbeitsvertrag.contract_end_date,
      probationEndDate: data.arbeitsvertrag.probation_end_date,
      weeklyHours: data.arbeitsvertrag.weekly_hours,
      employmentType: data.arbeitsvertrag.employment_type,
    },
    qualifikationen: (data.qualifikationen ?? []).map((q) => ({
      id: q.id != null ? q.id.toString() : null,
      name: q.name,
      acquiredOn: q.acquired_on,
      expiresOn: q.expires_on,
    })),
  };
}

async function throwStammdatenError(
  response: Response,
  fallback: string,
): Promise<never> {
  const error = await readStaffAPIError(response, fallback);
  if (error.code === "stammdaten_invalid") {
    throw new Error(
      "Ungültige Eingabe. Bitte prüfe die Werte und versuche es erneut.",
    );
  }
  throw new Error(error.message);
}

// Stammdaten (#1423): section-scoped master data of one staff member. The
// non-sensitive sections ride on users:read/users:update, the bank & tax
// section is staff:financial only — callers gate rendering on the permission
// so no request fires without it.
class StaffStammdatenService {
  async get(staffId: string): Promise<StaffStammdaten> {
    const response = await sessionFetch(`/api/staff/${staffId}/stammdaten`);
    if (!response.ok) {
      throw new Error(`Failed to fetch stammdaten: ${response.statusText}`);
    }
    const json = (await response.json()) as { data: BackendStammdaten };
    return mapStammdatenResponse(json.data);
  }

  private async putSection(
    staffId: string,
    section: string,
    body: Record<string, unknown>,
  ): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/stammdaten/${section}`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );
    if (!response.ok) {
      await throwStammdatenError(
        response,
        "Stammdaten konnten nicht gespeichert werden",
      );
    }
  }

  async updatePerson(
    staffId: string,
    person: StammdatenPerson,
    note: string,
  ): Promise<void> {
    await this.putSection(staffId, "person", {
      first_name: person.firstName,
      last_name: person.lastName,
      birthday: person.birthday,
      gender: person.gender,
      note,
    });
  }

  async updateKontakt(
    staffId: string,
    kontakt: StammdatenKontakt,
    note: string,
  ): Promise<void> {
    await this.putSection(staffId, "kontakt", {
      address_street: kontakt.addressStreet,
      address_postal_code: kontakt.addressPostalCode,
      address_city: kontakt.addressCity,
      phone: kontakt.phone,
      email: kontakt.email,
      emergency_contact_name: kontakt.emergencyContactName,
      emergency_contact_phone: kontakt.emergencyContactPhone,
      note,
    });
  }

  async updateArbeitsvertrag(
    staffId: string,
    vertrag: StammdatenArbeitsvertrag,
    note: string,
  ): Promise<void> {
    await this.putSection(staffId, "arbeitsvertrag", {
      entry_date: vertrag.entryDate,
      contract_end_date: vertrag.contractEndDate,
      probation_end_date: vertrag.probationEndDate,
      weekly_hours: vertrag.weeklyHours,
      employment_type: vertrag.employmentType,
      note,
    });
  }

  async updateQualifikationen(
    staffId: string,
    qualifikationen: readonly StammdatenQualifikation[],
    note: string,
  ): Promise<void> {
    await this.putSection(staffId, "qualifikationen", {
      qualifikationen: qualifikationen.map((q) => ({
        name: q.name,
        acquired_on: q.acquiredOn,
        expires_on: q.expiresOn,
      })),
      note,
    });
  }

  async getFinancial(staffId: string): Promise<StaffFinancialMasked> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/stammdaten/bank-steuer`,
    );
    if (!response.ok) {
      throw new Error(`Failed to fetch financial data: ${response.statusText}`);
    }
    const json = (await response.json()) as {
      data: {
        iban_masked: string | null;
        tax_id_masked: string | null;
        social_security_number_masked: string | null;
      };
    };
    return {
      ibanMasked: json.data.iban_masked,
      taxIdMasked: json.data.tax_id_masked,
      socialSecurityNumberMasked: json.data.social_security_number_masked,
    };
  }

  // POST: the reveal writes a GDPR access-log row server-side.
  async revealFinancial(staffId: string): Promise<StaffFinancialPlain> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/stammdaten/bank-steuer/reveal`,
      { method: "POST", headers: { "Content-Type": "application/json" } },
    );
    if (!response.ok) {
      throw new Error(
        `Failed to reveal financial data: ${response.statusText}`,
      );
    }
    const json = (await response.json()) as {
      data: {
        iban: string | null;
        tax_id: string | null;
        social_security_number: string | null;
      };
    };
    return {
      iban: json.data.iban,
      taxId: json.data.tax_id,
      socialSecurityNumber: json.data.social_security_number,
    };
  }

  async updateFinancial(
    staffId: string,
    financial: StaffFinancialPlain,
    note: string,
  ): Promise<void> {
    await this.putSection(staffId, "bank-steuer", {
      iban: financial.iban,
      tax_id: financial.taxId,
      social_security_number: financial.socialSecurityNumber,
      note,
    });
  }
}

export const staffService = new StaffService();
export const staffStammdatenService = new StaffStammdatenService();
export const staffPayrollNumberService = new StaffPayrollNumberService();
export const staffScheduleService = new StaffScheduleService();
export const workTimeModelService = new WorkTimeModelService();
export const staffHistoryService = new StaffHistoryService();
export const staffAbsenceService = new StaffAbsenceService();
export const staffSessionEditsService = new StaffSessionEditsService();
export const staffSessionService = new StaffSessionService();
export const staffMonthSummaryService = new StaffMonthSummaryService();
export const staffBalanceAdjustmentService =
  new StaffBalanceAdjustmentService();
export const staffMonthCloseService = new StaffMonthCloseService();
