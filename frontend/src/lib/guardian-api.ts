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
  PhoneNumberCreateRequest,
  PhoneNumberUpdateRequest,
  BackendPhoneNumber,
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
  "student not found": "Schüler/in nicht gefunden",
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
 * Delete a guardian profile
 */
export async function deleteGuardian(guardianId: string): Promise<void> {
  const response = await fetch(`/api/guardians/${guardianId}`, {
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
    throw new Error(errorMessage);
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
export type InviteGuardianOutcome =
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
  if (result.status === "error") {
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
