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

// Error carrying the HTTP status so callers can branch on it. The guardian
// delete flow needs to tell a 409 (still linked — show the affected children
// and offer a full delete) apart from a 403 (not allowed to fully delete) and
// a generic failure.
export class GuardianApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "GuardianApiError";
    this.status = status;
  }
}

// Error message translations (English backend -> German frontend)
// Exported for testing
export const errorTranslations: Record<string, string> = {
  "invalid email format": "Ungültiges E-Mail-Format",
  "bereits registriert": "Diese E-Mail-Adresse wird bereits verwendet",
  // Duplicate-email on guardian create (#1513): the backend already sends a
  // German message, but translateApiError replaces any unrecognised string with
  // the generic catch-all — so this pattern is required for the helpful
  // "use the search" guidance to reach the user.
  "bereits vergeben":
    "Diese E-Mail-Adresse ist bereits vergeben. Bitte die vorhandene Person über die Suche auswählen.",
  "guardian not found": "Erziehungsberechtigte/r nicht gefunden",
  "student not found": "Kind nicht gefunden",
  "relationship already exists": "Diese Verknüpfung existiert bereits",
  "validation failed": "Validierung fehlgeschlagen",
  "invalid phone number format":
    "Ungültiges Telefonnummernformat (nur Ziffern, Leerzeichen, +, -, Klammern)",
  "phone number must contain at least 3 digits":
    "Telefonnummer muss mindestens 3 Ziffern enthalten",
  "phone number is required": "Telefonnummer ist erforderlich",
  unauthorized: "Keine Berechtigung",
  forbidden: "Zugriff verweigert",
};

/**
 * Translate backend error messages to user-friendly German messages
 * Exported for testing
 */
export function translateApiError(errorMessage: string): string {
  const lowerError = errorMessage.toLowerCase();

  // Check specific error patterns first (before generic "validation failed")
  // Order matters: more specific patterns must be checked before generic ones
  for (const [pattern, translation] of Object.entries(errorTranslations)) {
    if (pattern === "validation failed") continue; // Handle last
    if (lowerError.includes(pattern)) {
      return translation;
    }
  }

  // Handle "validation failed: <specific reason>" — extract and translate the reason
  if (lowerError.includes("validation failed")) {
    const colonIndex = lowerError.indexOf("validation failed:");
    if (colonIndex !== -1) {
      const reason = errorMessage
        .substring(colonIndex + "validation failed:".length)
        .trim();
      if (reason) {
        // Try to translate the extracted reason
        const translatedReason = translateApiError(reason);
        // If the reason itself translates to something specific, use it
        if (
          translatedReason !==
          "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut."
        ) {
          return translatedReason;
        }
      }
    }
    return errorTranslations["validation failed"]!;
  }

  // Return generic message for unknown errors
  return "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.";
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
  const response = await fetch(
    `/api/guardians/students/${studentId}/guardians`,
  );

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "fetch_student_guardians",
      });
      return { error: "Failed to fetch guardians" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to fetch guardians: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<
    BackendGuardianWithRelationship[]
  >;

  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to fetch guardians");
  }

  return (result.data ?? []).map(mapGuardianWithRelationshipResponse);
}

/**
 * Fetch all students for a guardian
 */
export async function fetchGuardianStudents(
  guardianId: string,
): Promise<BackendStudent[]> {
  const response = await fetch(`/api/guardians/${guardianId}/students`);

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "fetch_guardian_students",
      });
      return { error: "Failed to fetch students" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to fetch students: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendStudent[]>;

  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to fetch students");
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

  const response = await fetch("/api/guardians", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "create_guardian",
      });
      return { error: "Failed to create guardian" };
    });
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(`Failed to create guardian: ${response.statusText}`);
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendGuardianProfile>;

  if (result.status === "error" || !result.data) {
    throw new Error(
      translateApiError(result.error ?? "Failed to create guardian"),
    );
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

  const response = await fetch(`/api/guardians/${guardianId}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "update_guardian",
      });
      return { error: "Failed to update guardian" };
    });
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(`Failed to update guardian: ${response.statusText}`);
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendGuardianProfile>;

  if (result.status === "error" || !result.data) {
    throw new Error(
      translateApiError(result.error ?? "Failed to update guardian"),
    );
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
  const response = await fetch(url, {
    method: "DELETE",
  });

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "delete_guardian",
      });
      return { error: "Failed to delete guardian" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to delete guardian: ${response.statusText}`;
    throw new GuardianApiError(errorMessage, response.status);
  }

  // 204 No Content means successful deletion with no response body
  if (response.status === 204) {
    return;
  }

  // If there's a response body, parse it
  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to delete guardian");
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
  const response = await fetch(`/api/guardians/${guardianId}/delete-preview`);

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "fetch_guardian_delete_preview",
      });
      return { error: "Failed to load delete preview" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to load delete preview: ${response.statusText}`;
    throw new GuardianApiError(errorMessage, response.status);
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
    throw new Error(result.error ?? "Failed to load delete preview");
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

  const response = await fetch(
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
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "link_guardian_to_student",
      });
      return { error: "Failed to link guardian" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to link guardian: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to link guardian");
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
 * a 400 with a German message, translated for display.
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

  const response = await fetch(
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
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "create_student_guardians",
      });
      return { error: "Failed to create guardians" };
    });
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(`Failed to create guardians: ${response.statusText}`);
    throw new Error(errorMessage);
  }

  // 201 Created with no meaningful body — the caller reloads the guardian list.
  if (response.status === 204) {
    return;
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw new Error(
      translateApiError(result.error ?? "Failed to create guardians"),
    );
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

  const response = await fetch(
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
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "update_student_guardian_relationship",
      });
      return { error: "Failed to update relationship" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to update relationship: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to update relationship");
  }
}

/**
 * Remove a guardian from a student
 */
export async function removeGuardianFromStudent(
  studentId: string,
  guardianId: string,
): Promise<void> {
  const response = await fetch(
    `/api/guardians/students/${studentId}/guardians/${guardianId}`,
    {
      method: "DELETE",
    },
  );

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "remove_guardian_from_student",
      });
      return { error: "Failed to remove guardian" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to remove guardian: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  // 204 No Content means successful deletion with no response body
  if (response.status === 204) {
    return;
  }

  // If there's a response body, parse it
  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to remove guardian");
  }
}

// Outcome returned by the unified invite resolve, mirroring the backend.
type InviteGuardianOutcome =
  | "linked_existing_account"
  | "already_linked"
  | "invited"
  | "pending_approval";

export interface InviteGuardianResult {
  outcome: InviteGuardianOutcome;
  guardian_profile_id: string;
  invitation_id?: string;
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
  },
): Promise<InviteGuardianResult> {
  const response = await fetch(`/api/guardians/students/${studentId}/invite`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      email,
      first_name: options?.firstName ?? "",
      last_name: options?.lastName ?? "",
      relationship_type: options?.relationshipType ?? "",
    }),
  });

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "invite_guardian_to_student",
      });
      return { error: "Failed to invite guardian" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to invite guardian: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<InviteGuardianResult>;
  if (result.status === "error" || !result.data) {
    throw new Error(result.error ?? "Failed to invite guardian");
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
  const response = await fetch(
    `/api/guardians/search?q=${encodeURIComponent(query)}&page_size=${GUARDIAN_PICKER_RESULT_LIMIT}`,
  );

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "search_guardians",
      });
      return { error: "Failed to search guardians" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to search guardians: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  const result =
    (await response.json()) as PaginatedResponse<BackendGuardianPickerResponse>;

  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to search guardians");
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
  const response = await fetch(`/api/guardians/${guardianId}/phone-numbers`);

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "fetch_guardian_phone_numbers",
      });
      return { error: "Failed to fetch phone numbers" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to fetch phone numbers: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendPhoneNumber[]>;

  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to fetch phone numbers");
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

  const response = await fetch(`/api/guardians/${guardianId}/phone-numbers`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "add_guardian_phone_number",
      });
      return { error: "Failed to add phone number" };
    });
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(`Failed to add phone number: ${response.statusText}`);
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendPhoneNumber>;

  if (result.status === "error" || !result.data) {
    throw new Error(
      translateApiError(result.error ?? "Failed to add phone number"),
    );
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

  const response = await fetch(
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
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "update_guardian_phone_number",
      });
      return { error: "Failed to update phone number" };
    });
    const errorMessage = isErrorResponse(error)
      ? translateApiError(error.error)
      : translateApiError(
          `Failed to update phone number: ${response.statusText}`,
        );
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw new Error(
      translateApiError(result.error ?? "Failed to update phone number"),
    );
  }
}

/**
 * Delete a guardian's phone number
 */
export async function deleteGuardianPhoneNumber(
  guardianId: string,
  phoneId: string,
): Promise<void> {
  const response = await fetch(
    `/api/guardians/${guardianId}/phone-numbers/${phoneId}`,
    {
      method: "DELETE",
    },
  );

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "delete_guardian_phone_number",
      });
      return { error: "Failed to delete phone number" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to delete phone number: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  // 204 No Content means successful deletion with no response body
  if (response.status === 204) {
    return;
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to delete phone number");
  }
}

/**
 * Set a phone number as primary for a guardian
 */
export async function setGuardianPrimaryPhone(
  guardianId: string,
  phoneId: string,
): Promise<void> {
  const response = await fetch(
    `/api/guardians/${guardianId}/phone-numbers/${phoneId}/set-primary`,
    {
      method: "POST",
    },
  );

  if (!response.ok) {
    const error: unknown = await response.json().catch((err) => {
      logger.debug("json_parse_failed", {
        error: err instanceof Error ? err.message : String(err),
        status: response.status,
        operation: "set_guardian_primary_phone",
      });
      return { error: "Failed to set primary phone" };
    });
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to set primary phone: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<null>;

  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to set primary phone");
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
  };
}

/** List parent-initiated guardian invitations awaiting staff approval. */
export async function listPendingApprovals(): Promise<PendingApproval[]> {
  const response = await fetch("/api/guardians/invitations/pending-approval");
  if (!response.ok) {
    const error: unknown = await response.json().catch(() => ({
      error: "Failed to load approvals",
    }));
    throw new Error(
      isErrorResponse(error)
        ? error.error
        : `Failed to load approvals: ${response.statusText}`,
    );
  }
  const result = (await response.json()) as ApiResponse<
    BackendPendingApproval[]
  >;
  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to load approvals");
  }
  return (result.data ?? []).map(mapPendingApproval);
}

async function postInvitationAction(
  invitationId: string,
  action: "approve" | "reject",
): Promise<void> {
  const response = await fetch(
    `/api/guardians/invitations/${invitationId}/${action}`,
    { method: "POST" },
  );
  if (!response.ok) {
    const error: unknown = await response.json().catch(() => ({
      error: `Failed to ${action} invitation`,
    }));
    throw new Error(
      isErrorResponse(error)
        ? error.error
        : `Failed to ${action} invitation: ${response.statusText}`,
    );
  }
  if (response.status === 204) return;
  const result = (await response.json()) as ApiResponse<null>;
  if (result.status === "error") {
    throw new Error(result.error ?? `Failed to ${action} invitation`);
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
