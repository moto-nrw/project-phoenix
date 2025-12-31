// Guardian API Client
// Calls Next.js API routes which proxy to the Go backend

import type {
  Guardian,
  GuardianWithRelationship,
  GuardianFormData,
  StudentGuardianLinkRequest,
  BackendGuardianProfile,
  BackendGuardianWithRelationship,
} from "./guardian-helpers";
import {
  mapGuardianResponse,
  mapGuardianWithRelationshipResponse,
  mapGuardianFormDataToBackend,
  mapStudentGuardianLinkToBackend,
} from "./guardian-helpers";

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
  phone?: string | null;
  mobile_phone?: string | null;
  address_street?: string | null;
  address_city?: string | null;
  address_postal_code?: string | null;
  preferred_contact_method?: string;
  language_preference?: string;
  occupation?: string | null;
  employer?: string | null;
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
  if (data.phone !== undefined) result.phone = data.phone;
  if (data.mobilePhone !== undefined) result.mobile_phone = data.mobilePhone;
  if (data.addressStreet !== undefined)
    result.address_street = data.addressStreet;
  if (data.addressCity !== undefined) result.address_city = data.addressCity;
  if (data.addressPostalCode !== undefined)
    result.address_postal_code = data.addressPostalCode;
  if (data.preferredContactMethod)
    result.preferred_contact_method = data.preferredContactMethod;
  if (data.languagePreference)
    result.language_preference = data.languagePreference;
  if (data.occupation !== undefined) result.occupation = data.occupation;
  if (data.employer !== undefined) result.employer = data.employer;
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
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to fetch guardians" }));
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
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to fetch students" }));
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
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to create guardian" }));
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to create guardian: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendGuardianProfile>;

  if (result.status === "error" || !result.data) {
    throw new Error(result.error ?? "Failed to create guardian");
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
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to update guardian" }));
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to update guardian: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  const result = (await response.json()) as ApiResponse<BackendGuardianProfile>;

  if (result.status === "error" || !result.data) {
    throw new Error(result.error ?? "Failed to update guardian");
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
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to delete guardian" }));
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
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to link guardian" }));
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
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to update relationship" }));
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
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to remove guardian" }));
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

/**
 * Search for existing guardians (for linking)
 */
export async function searchGuardians(query: string): Promise<Guardian[]> {
  const response = await fetch(
    `/api/guardians?search=${encodeURIComponent(query)}`,
  );

  if (!response.ok) {
    const error: unknown = await response
      .json()
      .catch(() => ({ error: "Failed to search guardians" }));
    const errorMessage = isErrorResponse(error)
      ? error.error
      : `Failed to search guardians: ${response.statusText}`;
    throw new Error(errorMessage);
  }

  const result =
    (await response.json()) as PaginatedResponse<BackendGuardianProfile>;

  if (result.status === "error") {
    throw new Error(result.error ?? "Failed to search guardians");
  }

  return (result.data ?? []).map(mapGuardianResponse);
}
