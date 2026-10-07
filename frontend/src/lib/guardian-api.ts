import { ApiError, enrichApiError, transportFetch } from "./api-error";
// Guardian API Client
// Calls Next.js API routes which proxy to the Go backend

import { createLogger } from "~/lib/logger";
import type {
  Guardian,
  GuardianWithRelationship,
  GuardianFormData,
  StudentGuardianLinkRequest,
  BackendGuardianProfile,
  BackendGuardianPickerResponse,
  BackendGuardianWithRelationship,
  PhoneNumber,
  PhoneType,
  PhoneNumberCreateRequest,
  PhoneNumberUpdateRequest,
  BackendPhoneNumber,
  GuardianRole,
} from "./guardian-helpers";
import {
  mapGuardianResponse,
  mapGuardianPickerResponse,
  mapGuardianWithRelationshipResponse,
  mapGuardianFormDataToBackend,
  mapStudentGuardianLinkToBackend,
  mapPhoneNumberResponse,
  mapPhoneNumberCreateToBackend,
  mapPhoneNumberUpdateToBackend,
} from "./guardian-helpers";

const logger = createLogger({ component: "GuardianAPI" });

// API Response Types
interface ApiResponse<T> {
  status: string;
  data?: T;
  message?: string;
  error?: string;
}

interface PaginatedResponse<T> {
  status: string;
  data: T[];
  pagination: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
  message?: string;
  error?: string;
}

// Error response from failed JSON parsing
interface ErrorResponse {
  error: string;
}

// Type guard for error responses
function isErrorResponse(value: unknown): value is ErrorResponse {
  return (
    typeof value === "object" &&
    value !== null &&
    "error" in value &&
    typeof (value as ErrorResponse).error === "string"
  );
}

// Every failure of this client is a GuardianApiError: the wire code, field
// errors and request ID travel to the shared error display (#2517); the
// message is a diagnostic for the logs and is never shown.
class GuardianApiError extends ApiError {
  status: number;

  constructor(message: string, status: number) {
    super(message, status);
    this.name = "GuardianApiError";
    this.status = status;
  }
}

/** The failed response as a GuardianApiError carrying the envelope. */
async function guardianApiError(
  response: Response,
  operation: string,
  fallback: string,
): Promise<GuardianApiError> {
  const body: unknown = await response.json().catch((err: unknown) => {
    logger.debug("json_parse_failed", {
      error: err instanceof Error ? err.message : String(err),
      status: response.status,
      operation,
    });
    return undefined;
  });
  const message = isErrorResponse(body) ? body.error : fallback;
  return enrichApiError(new GuardianApiError(message, response.status), body);
}

/**
 * A 2xx answer whose body still says `status: "error"`, or lacks the data
 * the call needs. Nothing about it is the user's input, so it counts as a
 * server failure.
 */
function bodyError(body: unknown, fallback: string) {
  const message = isErrorResponse(body) ? body.error : fallback;
  return enrichApiError(new GuardianApiError(message, 500), body);
}

// Backend student type (minimal representation for guardian relationships)
interface BackendStudent {
  id: number;
  first_name: string;
  last_name: string;
  date_of_birth: string;
}

// Partial update request type for guardian profile
interface PartialGuardianUpdateRequest {
  first_name?: string;
  last_name?: string;
  email?: string | null;
  address_street?: string | null;
  address_city?: string | null;
  address_postal_code?: string | null;
  preferred_contact_method?: string;
  language_preference?: string;
  notes?: string | null;
}

/**
 * Map frontend guardian form data to backend request format.
 * Only includes fields that are defined in the input.
 */
function mapGuardianFormToBackend(
  data: Partial<GuardianFormData>,
): PartialGuardianUpdateRequest {
  const result: PartialGuardianUpdateRequest = {};

  if (data.firstName) result.first_name = data.firstName;
  if (data.lastName) result.last_name = data.lastName;
  if (data.email !== undefined) result.email = data.email;
  if (data.addressStreet !== undefined)
    result.address_street = data.addressStreet;
  if (data.addressCity !== undefined) result.address_city = data.addressCity;
  if (data.addressPostalCode !== undefined)
    result.address_postal_code = data.addressPostalCode;
  if (data.preferredContactMethod)
    result.preferred_contact_method = data.preferredContactMethod;
  if (data.languagePreference)
    result.language_preference = data.languagePreference;
  if (data.notes !== undefined) result.notes = data.notes;

  return result;
}

// Partial update request for student-guardian relationship
interface PartialRelationshipUpdateRequest {
  relationship_type?: string;
  guardian_role?: GuardianRole;
  is_primary?: boolean;
  is_emergency_contact?: boolean;
  can_pickup?: boolean;
  pickup_notes?: string | null;
  emergency_priority?: number;
}

// Guardian API Client Functions

/**
 * Fetch all guardians for a student
 */
export async function fetchStudentGuardians(
  studentId: string,
): Promise<GuardianWithRelationship[]> {
  const response = await transportFetch(
    `/api/guardians/students/${studentId}/guardians`,
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "fetch_student_guardians",
      "Failed to fetch guardians",
    );
  }

  const result = (await response.json()) as ApiResponse<
    BackendGuardianWithRelationship[]
  >;

  if (result.status === "error") {
    throw bodyError(result, "Failed to fetch guardians");
  }

  return (result.data ?? []).map(mapGuardianWithRelationshipResponse);
}

/**
 * Fetch all students for a guardian
 */
export async function fetchGuardianStudents(
  guardianId: string,
): Promise<BackendStudent[]> {
  const response = await transportFetch(
    `/api/guardians/${guardianId}/students`,
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "fetch_guardian_students",
      "Failed to fetch students",
    );
  }

  const result = (await response.json()) as ApiResponse<BackendStudent[]>;

  if (result.status === "error") {
    throw bodyError(result, "Failed to fetch students");
  }

  return result.data ?? [];
}

/**
 * Create a new guardian profile
 */
export async function createGuardian(
  data: GuardianFormData,
): Promise<Guardian> {
  const backendData = mapGuardianFormDataToBackend(data);

  const response = await transportFetch("/api/guardians", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "create_guardian",
      "Failed to create guardian",
    );
  }

  const result = (await response.json()) as ApiResponse<BackendGuardianProfile>;

  if (result.status === "error" || !result.data) {
    throw bodyError(result, "Failed to create guardian");
  }

  return mapGuardianResponse(result.data);
}

/**
 * Update a guardian profile
 */
export async function updateGuardian(
  guardianId: string,
  data: Partial<GuardianFormData>,
): Promise<Guardian> {
  const backendData = mapGuardianFormToBackend(data);

  const response = await transportFetch(`/api/guardians/${guardianId}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "update_guardian",
      "Failed to update guardian",
    );
  }

  const result = (await response.json()) as ApiResponse<BackendGuardianProfile>;

  if (result.status === "error" || !result.data) {
    throw bodyError(result, "Failed to update guardian");
  }

  return mapGuardianResponse(result.data);
}

/**
 * Delete a guardian profile.
 *
 * By default this is a guarded delete: the backend refuses (409) a guardian
 * that is still linked to any student. Pass `{ force: true }` to perform the
 * deliberate full delete that also removes every student link — the backend
 * restricts that to administrators (403 otherwise). On any non-OK response a
 * {@link GuardianApiError} carrying the HTTP status is thrown so the caller can
 * branch on 409 / 403.
 */
export async function deleteGuardian(
  guardianId: string,
  opts: { force?: boolean; expectedAffectedLinkIds?: readonly string[] } = {},
): Promise<void> {
  const searchParams = new URLSearchParams();
  if (opts.force) {
    searchParams.set("force", "true");
  }
  if (opts.expectedAffectedLinkIds?.length) {
    searchParams.set(
      "expected_link_ids",
      opts.expectedAffectedLinkIds.join(","),
    );
  }
  const queryString = searchParams.toString();
  const url = queryString
    ? `/api/guardians/${guardianId}?${queryString}`
    : `/api/guardians/${guardianId}`;
  const response = await transportFetch(url, {
    method: "DELETE",
  });

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "delete_guardian",
      "Failed to delete guardian",
    );
  }

  // 204 No Content means successful deletion with no response body
  if (response.status === 204) {
    return;
  }

  // If there's a response body, parse it
  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw bodyError(result, "Failed to delete guardian");
  }
}

/** Read-only preview of what a full guardian delete would affect. */
export interface GuardianDeletePreview {
  /** How many students are still linked to the guardian. */
  linkedCount: number;
  /** Full names of the affected children. */
  affectedNames: string[];
  /** Relationship row IDs the server will compare on confirm. */
  affectedLinkIds: string[];
  /** Ready-to-show German warning describing the blast radius. */
  warning: string;
}

/**
 * Fetch the blast radius of a full guardian delete WITHOUT deleting anything.
 *
 * Backs the admin "Komplett löschen" confirmation: the modal shows
 * {@link GuardianDeletePreview.warning} (which children would lose the
 * guardian) before the user confirms the force delete. Admin-only on the
 * backend (403 otherwise), exposed via {@link GuardianApiError}.
 */
export async function fetchGuardianDeletePreview(
  guardianId: string,
): Promise<GuardianDeletePreview> {
  const response = await transportFetch(
    `/api/guardians/${guardianId}/delete-preview`,
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "fetch_guardian_delete_preview",
      "Failed to load delete preview",
    );
  }

  const result = (await response.json()) as ApiResponse<{
    linked_count: number;
    affected_names: string[];
    // The backend sends int64 link IDs as strings (lossless across the JS
    // safe-integer boundary); tolerate legacy numbers defensively.
    affected_link_ids: Array<string | number>;
    warning: string;
  }>;

  if (result.status === "error" || !result.data) {
    throw bodyError(result, "Failed to load delete preview");
  }

  return {
    linkedCount: result.data.linked_count,
    affectedNames: result.data.affected_names ?? [],
    affectedLinkIds: (result.data.affected_link_ids ?? []).map((id) =>
      String(id),
    ),
    warning: result.data.warning,
  };
}

/**
 * Link a guardian to a student
 */
export async function linkGuardianToStudent(
  studentId: string,
  linkData: StudentGuardianLinkRequest,
): Promise<void> {
  const backendData = mapStudentGuardianLinkToBackend(linkData);

  const response = await transportFetch(
    `/api/guardians/students/${studentId}/guardians`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(backendData),
    },
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "link_guardian_to_student",
      "Failed to link guardian",
    );
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw bodyError(result, "Failed to link guardian");
  }
}

/**
 * One guardian to create and link to an existing student in a single atomic
 * request. This flow always creates a NEW profile; the sibling/existing-guardian
 * case is handled separately by {@link linkGuardianToStudent} via the picker.
 */
export interface NewStudentGuardianInput {
  firstName: string;
  lastName: string;
  email?: string;
  addressStreet?: string;
  addressCity?: string;
  addressPostalCode?: string;
  languagePreference?: string;
  notes?: string;
  relationshipType: string;
  guardianRole?: GuardianRole;
  isPrimary: boolean;
  isEmergencyContact: boolean;
  canPickup: boolean;
  pickupNotes?: string;
  emergencyPriority: number;
  phoneNumbers?: Array<{
    phoneNumber: string;
    phoneType: PhoneType;
    label?: string;
    isPrimary: boolean;
  }>;
}

/**
 * Atomically create one or more guardians for an existing student.
 *
 * The backend creates every guardian profile, links it to the student, and adds
 * its phone numbers inside ONE transaction (#819). Any failure rolls the whole
 * batch back server-side, so there are no orphaned profiles and the client needs
 * no compensating delete (which a non-admin supervisor could not perform once a
 * guardian had lost its links). Bad input (e.g. a duplicate email) comes back as
 * a 400; the shared error display shows the catalog text for its code.
 */
export async function createStudentGuardians(
  studentId: string,
  guardians: NewStudentGuardianInput[],
): Promise<void> {
  const body = {
    guardians: guardians.map((g) => ({
      first_name: g.firstName,
      last_name: g.lastName,
      email: g.email,
      address_street: g.addressStreet,
      address_city: g.addressCity,
      address_postal_code: g.addressPostalCode,
      language_preference: g.languagePreference,
      notes: g.notes,
      relationship_type: g.relationshipType,
      guardian_role: g.guardianRole,
      is_primary: g.isPrimary,
      is_emergency_contact: g.isEmergencyContact,
      can_pickup: g.canPickup,
      pickup_notes: g.pickupNotes,
      emergency_priority: g.emergencyPriority,
      phone_numbers: g.phoneNumbers?.map((p) => ({
        phone_number: p.phoneNumber,
        phone_type: p.phoneType,
        label: p.label,
        is_primary: p.isPrimary,
      })),
    })),
  };

  const response = await transportFetch(
    `/api/guardians/students/${studentId}/guardians/batch`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(body),
    },
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "create_student_guardians",
      "Failed to create guardians",
    );
  }

  // 201 Created with no meaningful body — the caller reloads the guardian list.
  if (response.status === 204) {
    return;
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw bodyError(result, "Failed to create guardians");
  }
}

/**
 * Update student-guardian relationship
 */
export async function updateStudentGuardianRelationship(
  relationshipId: string,
  updates: Partial<StudentGuardianLinkRequest>,
): Promise<void> {
  const backendData: PartialRelationshipUpdateRequest = {};

  if (updates.relationshipType !== undefined) {
    backendData.relationship_type = updates.relationshipType;
  }
  if (updates.guardianRole !== undefined) {
    backendData.guardian_role = updates.guardianRole;
  }
  if (updates.isPrimary !== undefined) {
    backendData.is_primary = updates.isPrimary;
  }
  if (updates.isEmergencyContact !== undefined) {
    backendData.is_emergency_contact = updates.isEmergencyContact;
  }
  if (updates.canPickup !== undefined) {
    backendData.can_pickup = updates.canPickup;
  }
  if (updates.pickupNotes !== undefined) {
    backendData.pickup_notes = updates.pickupNotes;
  }
  if (updates.emergencyPriority !== undefined) {
    backendData.emergency_priority = updates.emergencyPriority;
  }

  const response = await transportFetch(
    `/api/guardians/relationships/${relationshipId}`,
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(backendData),
    },
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "update_student_guardian_relationship",
      "Failed to update relationship",
    );
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw bodyError(result, "Failed to update relationship");
  }
}

/**
 * Remove a guardian from a student
 */
export async function removeGuardianFromStudent(
  studentId: string,
  guardianId: string,
): Promise<void> {
  const response = await transportFetch(
    `/api/guardians/students/${studentId}/guardians/${guardianId}`,
    {
      method: "DELETE",
    },
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "remove_guardian_from_student",
      "Failed to remove guardian",
    );
  }

  // 204 No Content means successful deletion with no response body
  if (response.status === 204) {
    return;
  }

  // If there's a response body, parse it
  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw bodyError(result, "Failed to remove guardian");
  }
}

// Outcome returned by the unified invite resolve, mirroring the backend.
type InviteGuardianOutcome =
  | "linked_existing_account"
  | "already_linked"
  | "invited"
  | "pending_approval"
  | "existing_contact_restricted";

export interface InviteGuardianResult {
  outcome: InviteGuardianOutcome;
  guardian_profile_id: string;
  invitation_id?: string;
  // Current guardian role for the existing_contact_restricted outcome, so the
  // UI can name it in the upgrade confirmation (#2172).
  existing_role?: string;
}

/**
 * Invite a guardian to a student by email. For a guardian whose info is already
 * on file (no account yet), pass their on-file email — the backend resolves the
 * existing profile and sends the invite without creating a duplicate. Staff
 * path: invites act immediately (no approval queue).
 */
export async function inviteGuardianToStudent(
  studentId: string,
  email: string,
  options?: {
    firstName?: string;
    lastName?: string;
    relationshipType?: string;
    confirmRoleUpgrade?: boolean;
  },
): Promise<InviteGuardianResult> {
  const response = await transportFetch(
    `/api/guardians/students/${studentId}/invite`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email,
        first_name: options?.firstName ?? "",
        last_name: options?.lastName ?? "",
        relationship_type: options?.relationshipType ?? "",
        // Only sent when confirming an upgrade; a plain invite keeps the
        // historical four-field body.
        ...(options?.confirmRoleUpgrade ? { confirm_role_upgrade: true } : {}),
      }),
    },
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "invite_guardian_to_student",
      "Failed to invite guardian",
    );
  }

  const result = (await response.json()) as ApiResponse<InviteGuardianResult>;
  if (result.status === "error" || !result.data) {
    throw bodyError(result, "Failed to invite guardian");
  }
  return result.data;
}

// Hard ceiling on guardian picker results, requested explicitly so the picker
// doesn't silently ride on whatever the backend's default page size happens to
// be. The backend clamps to the same value (maxGuardianPickerResults); keep the
// two in sync. The picker uses it to detect "the list was capped" — when a
// search returns exactly this many rows, more may exist and the user should
// narrow their query rather than assume the person isn't there (#1513).
export const GUARDIAN_PICKER_RESULT_LIMIT = 50;

/**
 * Search for existing guardians (for linking)
 */
export async function searchGuardians(query: string): Promise<Guardian[]> {
  const response = await transportFetch(
    `/api/guardians/search?q=${encodeURIComponent(query)}&page_size=${GUARDIAN_PICKER_RESULT_LIMIT}`,
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "search_guardians",
      "Failed to search guardians",
    );
  }

  const result =
    (await response.json()) as PaginatedResponse<BackendGuardianPickerResponse>;

  if (result.status === "error") {
    throw bodyError(result, "Failed to search guardians");
  }

  return (result.data ?? []).map(mapGuardianPickerResponse);
}

// =============================================================================
// Phone Number API Functions
// =============================================================================

/**
 * Fetch all phone numbers for a guardian
 */
export async function fetchGuardianPhoneNumbers(
  guardianId: string,
): Promise<PhoneNumber[]> {
  const response = await transportFetch(
    `/api/guardians/${guardianId}/phone-numbers`,
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "fetch_guardian_phone_numbers",
      "Failed to fetch phone numbers",
    );
  }

  const result = (await response.json()) as ApiResponse<BackendPhoneNumber[]>;

  if (result.status === "error") {
    throw bodyError(result, "Failed to fetch phone numbers");
  }

  return (result.data ?? []).map(mapPhoneNumberResponse);
}

/**
 * Add a phone number to a guardian
 */
export async function addGuardianPhoneNumber(
  guardianId: string,
  data: PhoneNumberCreateRequest,
): Promise<PhoneNumber> {
  const backendData = mapPhoneNumberCreateToBackend(data);

  const response = await transportFetch(
    `/api/guardians/${guardianId}/phone-numbers`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(backendData),
    },
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "add_guardian_phone_number",
      "Failed to add phone number",
    );
  }

  const result = (await response.json()) as ApiResponse<BackendPhoneNumber>;

  if (result.status === "error" || !result.data) {
    throw bodyError(result, "Failed to add phone number");
  }

  return mapPhoneNumberResponse(result.data);
}

/**
 * Update a guardian's phone number
 */
export async function updateGuardianPhoneNumber(
  guardianId: string,
  phoneId: string,
  data: PhoneNumberUpdateRequest,
): Promise<void> {
  const backendData = mapPhoneNumberUpdateToBackend(data);

  const response = await transportFetch(
    `/api/guardians/${guardianId}/phone-numbers/${phoneId}`,
    {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(backendData),
    },
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "update_guardian_phone_number",
      "Failed to update phone number",
    );
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw bodyError(result, "Failed to update phone number");
  }
}

/**
 * Delete a guardian's phone number
 */
export async function deleteGuardianPhoneNumber(
  guardianId: string,
  phoneId: string,
): Promise<void> {
  const response = await transportFetch(
    `/api/guardians/${guardianId}/phone-numbers/${phoneId}`,
    {
      method: "DELETE",
    },
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "delete_guardian_phone_number",
      "Failed to delete phone number",
    );
  }

  // 204 No Content means successful deletion with no response body
  if (response.status === 204) {
    return;
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw bodyError(result, "Failed to delete phone number");
  }
}

/**
 * Set a phone number as primary for a guardian
 */
export async function setGuardianPrimaryPhone(
  guardianId: string,
  phoneId: string,
): Promise<void> {
  const response = await transportFetch(
    `/api/guardians/${guardianId}/phone-numbers/${phoneId}/set-primary`,
    {
      method: "POST",
    },
  );

  if (!response.ok) {
    throw await guardianApiError(
      response,
      "set_guardian_primary_phone",
      "Failed to set primary phone",
    );
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw bodyError(result, "Failed to set primary phone");
  }
}

// ============================================================================
// Guardian invitation approval queue (parent-initiated invites)
// ============================================================================

// Staff-facing approval-queue row.
export interface PendingApproval {
  id: string;
  guardianProfileId: string;
  guardianName: string;
  guardianEmail?: string;
  studentId?: string;
  studentName?: string;
  requestedByEmail?: string;
  createdAt: string;
  expiresAt: string;
  // Approving this request also upgrades an existing restrictive contact
  // link to full portal access (#2172).
  roleUpgrade: boolean;
}

interface BackendPendingApproval {
  id: string;
  guardian_profile_id: string;
  guardian_name: string;
  guardian_email?: string;
  student_id?: string;
  student_name?: string;
  requested_by_email?: string;
  created_at: string;
  expires_at: string;
  role_upgrade?: boolean;
}

function mapPendingApproval(data: BackendPendingApproval): PendingApproval {
  return {
    id: data.id,
    guardianProfileId: data.guardian_profile_id,
    guardianName: data.guardian_name,
    guardianEmail: data.guardian_email,
    studentId: data.student_id,
    studentName: data.student_name,
    requestedByEmail: data.requested_by_email,
    createdAt: data.created_at,
    expiresAt: data.expires_at,
    roleUpgrade: data.role_upgrade ?? false,
  };
}

/** List parent-initiated guardian invitations awaiting staff approval. */
export async function listPendingApprovals(): Promise<PendingApproval[]> {
  const response = await transportFetch(
    "/api/guardians/invitations/pending-approval",
  );
  if (!response.ok) {
    throw await guardianApiError(
      response,
      "list_pending_approvals",
      "Failed to load approvals",
    );
  }
  const result = (await response.json()) as ApiResponse<
    BackendPendingApproval[]
  >;
  if (result.status === "error") {
    throw bodyError(result, "Failed to load approvals");
  }
  return (result.data ?? []).map(mapPendingApproval);
}

async function postInvitationAction(
  invitationId: string,
  action: "approve" | "reject",
): Promise<void> {
  const response = await transportFetch(
    `/api/guardians/invitations/${invitationId}/${action}`,
    { method: "POST" },
  );
  if (!response.ok) {
    throw await guardianApiError(
      response,
      `${action}_invitation`,
      `Failed to ${action} invitation`,
    );
  }
  if (response.status === 204) return;
  const result = (await response.json()) as ApiResponse<null>;
  if (result.status === "error") {
    throw bodyError(result, `Failed to ${action} invitation`);
  }
}

/** Approve a pending parent-initiated invitation. */
export async function approveGuardianInvitation(
  invitationId: string,
): Promise<void> {
  return postInvitationAction(invitationId, "approve");
}

/** Reject a pending parent-initiated invitation. */
export async function rejectGuardianInvitation(
  invitationId: string,
): Promise<void> {
  return postInvitationAction(invitationId, "reject");
}
