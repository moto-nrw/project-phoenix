import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CaregiverCapabilityAPI" });

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
  disableBlockersCount: number;
  disableBlockers: string[];
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
  disable_blockers_count: number;
  disable_blockers?: string[];
}

export interface EnableCaregiverCapabilityRequest {
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
    disableBlockersCount: state.disable_blockers_count,
    disableBlockers: state.disable_blockers ?? [],
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
    blockers = payload.blockers ?? [];
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
