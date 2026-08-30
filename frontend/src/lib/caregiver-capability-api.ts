import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CaregiverCapabilityAPI" });

export interface BlockerSupervision {
  id: string;
  groupName: string;
  startDate: string;
}

export interface BlockerSubstitution {
  id: string;
  groupName: string;
  startDate: string;
  endDate: string;
}

export interface BlockerActivity {
  id: string;
  activityId: string;
  activityName: string;
  isPrimary: boolean;
}

export interface BlockerGroup {
  id: string;
  groupId: string;
  groupName: string;
  teacherId: string;
  teacherIds: string[];
}

export interface CaregiverCapabilityState {
  accountId: string;
  email: string;
  firstName: string;
  lastName: string;
  personId: string | null;
  staffId: string | null;
  teacherId: string | null;
  hasAdminRole: boolean;
  hasUserRole: boolean;
  hasPerson: boolean;
  hasStaff: boolean;
  hasTeacher: boolean;
  hasCaregiverProfile: boolean;
  isActiveCaregiver: boolean;
  disableBlocked: boolean;
  disableBlockers: string[];
  activeSupervisions: BlockerSupervision[];
  activeSubstitutions: BlockerSubstitution[];
  activitySupervisions: BlockerActivity[];
  groupAssignments: BlockerGroup[];
}

function formatCount(count: number, singular: string, plural: string): string {
  return `${count} ${count === 1 ? singular : plural}`;
}

function translateCapabilityBlocker(
  blocker: string,
  state?: BackendCaregiverCapabilityState,
): string {
  switch (blocker) {
    case "missing_usable_role":
      return "Ohne Betreuerfähigkeit bliebe dem Konto keine nutzbare Systemrolle.";
    case "active_group_supervisions":
      if (!state) {
        return "Aktive Gruppenaufsichten";
      }
      return formatCount(
        state?.active_supervisions?.length ?? 0,
        "aktive Gruppenaufsicht",
        "aktive Gruppenaufsichten",
      );
    case "active_group_substitutions":
      if (!state) {
        return "Aktive Gruppenübergaben";
      }
      return formatCount(
        state?.active_substitutions?.length ?? 0,
        "aktive Gruppenübergabe",
        "aktive Gruppenübergaben",
      );
    case "activity_supervisions":
      if (!state) {
        return "Aktivitätsleitungen";
      }
      return formatCount(
        state?.activity_supervisions?.length ?? 0,
        "Aktivitätsleitung",
        "Aktivitätsleitungen",
      );
    case "group_assignments":
      if (!state) {
        return "Stammgruppen-Zuordnungen";
      }
      return formatCount(
        state?.group_assignments?.length ?? 0,
        "Stammgruppen-Zuordnung",
        "Stammgruppen-Zuordnungen",
      );
    default:
      return blocker;
  }
}

interface BackendBlockerSupervision {
  id: number;
  group_name: string;
  start_date: string;
}

interface BackendBlockerSubstitution {
  id: number;
  group_name: string;
  start_date: string;
  end_date: string;
}

interface BackendBlockerActivity {
  id: number;
  activity_id: number;
  activity_name: string;
  is_primary: boolean;
}

interface BackendBlockerGroup {
  id: number;
  group_id: number;
  group_name: string;
  teacher_id: number;
  teacher_ids?: number[];
}

interface BackendCaregiverCapabilityState {
  account_id: number;
  email: string;
  first_name: string;
  last_name: string;
  person_id?: number;
  staff_id?: number;
  teacher_id?: number;
  has_admin_role: boolean;
  has_user_role: boolean;
  has_person: boolean;
  has_staff: boolean;
  has_teacher: boolean;
  has_caregiver_profile: boolean;
  is_active_caregiver: boolean;
  disable_blocked: boolean;
  disable_blockers?: string[];
  active_supervisions?: BackendBlockerSupervision[];
  active_substitutions?: BackendBlockerSubstitution[];
  activity_supervisions?: BackendBlockerActivity[];
  group_assignments?: BackendBlockerGroup[];
}

interface EnableCaregiverCapabilityRequest {
  first_name?: string;
  last_name?: string;
  position?: string;
}

interface ApiEnvelope<T> {
  data?: T;
  success?: boolean;
  status?: string;
  message?: string;
}

interface CapabilityErrorPayload {
  error?: string;
  message?: string;
  blockers?: string[];
}

export class CaregiverCapabilityApiError extends Error {
  status: number;
  blockers: string[];

  constructor(message: string, status: number, blockers: string[] = []) {
    super(message);
    this.name = "CaregiverCapabilityApiError";
    this.status = status;
    this.blockers = blockers;
  }
}

function mapCapabilityState(
  state: BackendCaregiverCapabilityState,
): CaregiverCapabilityState {
  return {
    accountId: state.account_id.toString(),
    email: state.email,
    firstName: state.first_name,
    lastName: state.last_name,
    personId: state.person_id?.toString() ?? null,
    staffId: state.staff_id?.toString() ?? null,
    teacherId: state.teacher_id?.toString() ?? null,
    hasAdminRole: state.has_admin_role,
    hasUserRole: state.has_user_role,
    hasPerson: state.has_person,
    hasStaff: state.has_staff,
    hasTeacher: state.has_teacher,
    hasCaregiverProfile: state.has_caregiver_profile,
    isActiveCaregiver: state.is_active_caregiver,
    disableBlocked: state.disable_blocked,
    disableBlockers: (state.disable_blockers ?? []).map((blocker) =>
      translateCapabilityBlocker(blocker, state),
    ),
    activeSupervisions: (state.active_supervisions ?? []).map((s) => ({
      id: s.id.toString(),
      groupName: s.group_name,
      startDate: s.start_date,
    })),
    activeSubstitutions: (state.active_substitutions ?? []).map((s) => ({
      id: s.id.toString(),
      groupName: s.group_name,
      startDate: s.start_date,
      endDate: s.end_date,
    })),
    activitySupervisions: (state.activity_supervisions ?? []).map((a) => ({
      id: a.id.toString(),
      activityId: a.activity_id.toString(),
      activityName: a.activity_name,
      isPrimary: a.is_primary,
    })),
    groupAssignments: (state.group_assignments ?? []).map((g) => ({
      id: g.id.toString(),
      groupId: g.group_id.toString(),
      groupName: g.group_name,
      teacherId: g.teacher_id.toString(),
      teacherIds: (g.teacher_ids ?? []).map((teacherId) =>
        teacherId.toString(),
      ),
    })),
  };
}

async function parseCapabilityResponse(
  response: Response,
): Promise<CaregiverCapabilityState> {
  const raw = (await response.json()) as
    | BackendCaregiverCapabilityState
    | ApiEnvelope<BackendCaregiverCapabilityState>;

  const payload =
    typeof raw === "object" &&
    raw !== null &&
    "data" in raw &&
    raw.data !== undefined
      ? raw.data
      : (raw as BackendCaregiverCapabilityState);

  return mapCapabilityState(payload);
}

async function handleCapabilityError(response: Response): Promise<never> {
  let message = response.statusText || "Unbekannter Fehler";
  let blockers: string[] = [];

  try {
    const payload = (await response.json()) as CapabilityErrorPayload;
    message = payload.message ?? payload.error ?? message;
    blockers = (payload.blockers ?? []).map((blocker) =>
      translateCapabilityBlocker(blocker),
    );
  } catch (error) {
    logger.warn("failed to parse caregiver capability error response", {
      error: error instanceof Error ? error.message : String(error),
      status: response.status,
    });
  }

  throw new CaregiverCapabilityApiError(message, response.status, blockers);
}

async function requestCapability(
  endpoint: string,
  options: RequestInit = {},
): Promise<CaregiverCapabilityState> {
  const response = await fetch(endpoint, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(options.headers ?? {}),
    },
    ...options,
  });

  if (!response.ok) {
    await handleCapabilityError(response);
  }

  return parseCapabilityResponse(response);
}

class CaregiverCapabilityService {
  getTenantAccountCapability(accountId: string) {
    return requestCapability(
      `/api/auth/accounts/${encodeURIComponent(accountId)}/caregiver-capability`,
    );
  }

  enableTenantAccountCapability(
    accountId: string,
    body: EnableCaregiverCapabilityRequest,
  ) {
    return requestCapability(
      `/api/auth/accounts/${encodeURIComponent(accountId)}/caregiver-capability`,
      {
        method: "POST",
        body: JSON.stringify(body),
      },
    );
  }

  disableTenantAccountCapability(accountId: string) {
    return requestCapability(
      `/api/auth/accounts/${encodeURIComponent(accountId)}/caregiver-capability`,
      {
        method: "DELETE",
      },
    );
  }

  getOperatorAccountCapability(schoolId: string, accountId: string) {
    return requestCapability(
      `/api/operator/provisioning/schools/${encodeURIComponent(schoolId)}/accounts/${encodeURIComponent(accountId)}/caregiver-capability`,
    );
  }

  enableOperatorAccountCapability(
    schoolId: string,
    accountId: string,
    body: EnableCaregiverCapabilityRequest,
  ) {
    return requestCapability(
      `/api/operator/provisioning/schools/${encodeURIComponent(schoolId)}/accounts/${encodeURIComponent(accountId)}/caregiver-capability`,
      {
        method: "POST",
        body: JSON.stringify(body),
      },
    );
  }

  disableOperatorAccountCapability(schoolId: string, accountId: string) {
    return requestCapability(
      `/api/operator/provisioning/schools/${encodeURIComponent(schoolId)}/accounts/${encodeURIComponent(accountId)}/caregiver-capability`,
      {
        method: "DELETE",
      },
    );
  }
}

export const caregiverCapabilityService = new CaregiverCapabilityService();
