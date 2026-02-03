// Staff API service for fetching all staff members and their supervision status

import { getCachedSession } from "./session-cache";

// Backend response types (already mapped by the API route handler)
export interface BackendStaffResponse {
  id: string;
  name: string;
  firstName: string;
  lastName: string;
  specialization?: string | null;
  role: string | null;
  qualifications: string | null;
  tag_id: string | null;
  staff_notes: string | null;
  created_at: string;
  updated_at: string;
  staff_id?: string;
  teacher_id?: string;
  was_present_today?: boolean;
  work_status?: string;
  absence_type?: string;
}

export interface ActiveSupervisionResponse {
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
export interface StaffSupervision {
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
  role?: string; // Display role (Admin/Betreuer/Extern)
  specialization?: string;
  qualifications?: string;
  staffNotes?: string;
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
}

export interface StaffFilters {
  search?: string;
  status?: "all" | "supervising" | "available";
  type?: "all" | "teachers" | "staff";
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
  other: "Abwesend",
};

/**
 * Determines location and supervision info for a staff member.
 *
 * Badge shows time clock status (Anwesend/Abwesend/Homeoffice/Krank/etc.)
 * Supervisions are returned separately as an array of rooms.
 *
 * Priority for currentLocation (badge):
 * 1. Absence today → "Krank" / "Urlaub" / "Fortbildung" / "Abwesend"
 * 2. Time clock present → "Anwesend"
 * 3. Time clock home_office → "Homeoffice"
 * 4. Checked out or never clocked in → "Abwesend"
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

  // Priority 1: Absence today → overrides everything
  if (absenceType && absenceLabels[absenceType]) {
    currentLocation = absenceLabels[absenceType];
  }
  // Priority 2: Time clock present
  else if (workStatus === "present") {
    currentLocation = "Anwesend";
  }
  // Priority 3: Time clock home office
  else if (workStatus === "home_office") {
    currentLocation = "Homeoffice";
  }
  // Priority 4: Explicitly checked out → Abwesend
  else if (workStatus === "checked_out") {
    currentLocation = "Abwesend";
  }
  // Priority 5: Legacy fallback (only if NO work_status data exists)
  else if (wasPresentToday && !workStatus) {
    currentLocation = "Anwesend";
  }
  // Priority 6: Not present
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
 * Builds fetch options with authorization header
 */
function buildFetchOptions(token: string): RequestInit {
  return {
    credentials: "include",
    headers: {
      Authorization: `Bearer ${token}`,
      "Content-Type": "application/json",
    },
  };
}

/**
 * Fetches active groups data, returning empty array on failure
 */
async function fetchActiveGroups(token: string): Promise<ActiveGroupWithId[]> {
  try {
    const response = await fetch(
      "/api/active/groups?active=true",
      buildFetchOptions(token),
    );
    if (!response.ok) return [];
    const data = (await response.json()) as unknown;
    return extractActiveGroups(data);
  } catch {
    return [];
  }
}

// Staff service
class StaffService {
  // Get all staff members with their current supervision status
  async getAllStaff(filters?: StaffFilters): Promise<Staff[]> {
    const session = await getCachedSession();
    const token = session?.user?.token;

    if (!token) {
      throw new Error("No authentication token available");
    }

    // Build staff URL with search filter
    const staffUrl = filters?.search
      ? `/api/staff?search=${encodeURIComponent(filters.search)}`
      : "/api/staff";

    // Fetch staff and active groups in parallel
    const [staffResponse, activeGroups] = await Promise.all([
      fetch(staffUrl, buildFetchOptions(token)),
      fetchActiveGroups(token),
    ]);

    if (!staffResponse.ok) {
      throw new Error(`Failed to fetch staff: ${staffResponse.statusText}`);
    }

    const staffData = (await staffResponse.json()) as
      | BackendStaffResponse[]
      | { data: BackendStaffResponse[] };
    const staffList = extractStaffList(staffData);
    const staffGroupsMap = buildStaffGroupsMap(activeGroups);

    const mappedStaff = staffList.map((staff) =>
      mapStaffMember(staff, staffGroupsMap),
    );

    return applyStaffFilters(mappedStaff, filters);
  }

  // Get active supervisions for a specific staff member
  async getStaffSupervisions(
    staffId: string,
  ): Promise<ActiveSupervisionResponse[]> {
    try {
      const session = await getCachedSession();
      const token = session?.user?.token;

      if (!token) {
        throw new Error("No authentication token available");
      }

      const response = await fetch(
        `/api/active/supervisors/staff/${staffId}/active`,
        {
          credentials: "include",
          headers: {
            Authorization: `Bearer ${token}`,
            "Content-Type": "application/json",
          },
        },
      );

      if (!response.ok) {
        throw new Error(
          `Failed to fetch staff supervisions: ${response.statusText}`,
        );
      }

      const data = (await response.json()) as
        | ActiveSupervisionResponse[]
        | { data: ActiveSupervisionResponse[] };

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
      console.error(`Error fetching supervisions for staff ${staffId}:`, error);
      return [];
    }
  }
}

export const staffService = new StaffService();
