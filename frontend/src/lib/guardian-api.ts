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
}

// Guardian API Client Functions

/**
 * Fetch all guardians for a student
 */
export async function fetchStudentGuardians(
  studentId: string
): Promise<GuardianWithRelationship[]> {
  const response = await fetch(`/api/guardians/students/${studentId}/guardians`);

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "Failed to fetch guardians" }));
    throw new Error(error.error || `Failed to fetch guardians: ${response.statusText}`);
  }

  const result: ApiResponse<BackendGuardianWithRelationship[]> = await response.json();

  if (result.status === "error") {
    throw new Error(result.error || "Failed to fetch guardians");
  }

  return (result.data || []).map(mapGuardianWithRelationshipResponse);
}

/**
 * Fetch all students for a guardian
 */
export async function fetchGuardianStudents(guardianId: string): Promise<any[]> {
  const response = await fetch(`/api/guardians/${guardianId}/students`);

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "Failed to fetch students" }));
    throw new Error(error.error || `Failed to fetch students: ${response.statusText}`);
  }

  const result: ApiResponse<any[]> = await response.json();

  if (result.status === "error") {
    throw new Error(result.error || "Failed to fetch students");
  }

  return result.data || [];
}

/**
 * Create a new guardian profile
 */
export async function createGuardian(data: GuardianFormData): Promise<Guardian> {
  const backendData = mapGuardianFormDataToBackend(data);

  const response = await fetch("/api/guardians", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "Failed to create guardian" }));
    throw new Error(error.error || `Failed to create guardian: ${response.statusText}`);
  }

  const result: ApiResponse<BackendGuardianProfile> = await response.json();

  if (result.status === "error" || !result.data) {
    throw new Error(result.error || "Failed to create guardian");
  }

  return mapGuardianResponse(result.data);
}

/**
 * Update a guardian profile
 */
export async function updateGuardian(
  guardianId: string,
  data: Partial<GuardianFormData>
): Promise<Guardian> {
  const backendData: any = {};

  if (data.firstName) backendData.first_name = data.firstName;
  if (data.lastName) backendData.last_name = data.lastName;
  if (data.email !== undefined) backendData.email = data.email;
  if (data.phone !== undefined) backendData.phone = data.phone;
  if (data.mobilePhone !== undefined) backendData.mobile_phone = data.mobilePhone;
  if (data.addressStreet !== undefined) backendData.address_street = data.addressStreet;
  if (data.addressCity !== undefined) backendData.address_city = data.addressCity;
  if (data.addressPostalCode !== undefined) backendData.address_postal_code = data.addressPostalCode;
  if (data.preferredContactMethod) backendData.preferred_contact_method = data.preferredContactMethod;
  if (data.languagePreference) backendData.language_preference = data.languagePreference;
  if (data.occupation !== undefined) backendData.occupation = data.occupation;
  if (data.employer !== undefined) backendData.employer = data.employer;
  if (data.notes !== undefined) backendData.notes = data.notes;

  const response = await fetch(`/api/guardians/${guardianId}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "Failed to update guardian" }));
    throw new Error(error.error || `Failed to update guardian: ${response.statusText}`);
  }

  const result: ApiResponse<BackendGuardianProfile> = await response.json();

  if (result.status === "error" || !result.data) {
    throw new Error(result.error || "Failed to update guardian");
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
    const error = await response.json().catch(() => ({ error: "Failed to delete guardian" }));
    throw new Error(error.error || `Failed to delete guardian: ${response.statusText}`);
  }

  // 204 No Content means successful deletion with no response body
  if (response.status === 204) {
    return;
  }

  // If there's a response body, parse it
  const result: ApiResponse<null> = await response.json();

  if (result.status === "error") {
    throw new Error(result.error || "Failed to delete guardian");
  }
}

/**
 * Link a guardian to a student
 */
export async function linkGuardianToStudent(
  studentId: string,
  linkData: StudentGuardianLinkRequest
): Promise<void> {
  const backendData = mapStudentGuardianLinkToBackend(linkData);

  const response = await fetch(`/api/guardians/students/${studentId}/guardians`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "Failed to link guardian" }));
    throw new Error(error.error || `Failed to link guardian: ${response.statusText}`);
  }

  const result: ApiResponse<any> = await response.json();

  if (result.status === "error") {
    throw new Error(result.error || "Failed to link guardian");
  }
}

/**
 * Update student-guardian relationship
 */
export async function updateStudentGuardianRelationship(
  relationshipId: string,
  updates: Partial<StudentGuardianLinkRequest>
): Promise<void> {
  const backendData: any = {};

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

  const response = await fetch(`/api/guardians/relationships/${relationshipId}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(backendData),
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "Failed to update relationship" }));
    throw new Error(error.error || `Failed to update relationship: ${response.statusText}`);
  }

  const result: ApiResponse<null> = await response.json();

  if (result.status === "error") {
    throw new Error(result.error || "Failed to update relationship");
  }
}

/**
 * Remove a guardian from a student
 */
export async function removeGuardianFromStudent(
  studentId: string,
  guardianId: string
): Promise<void> {
  const response = await fetch(
    `/api/guardians/students/${studentId}/guardians/${guardianId}`,
    {
      method: "DELETE",
    }
  );

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "Failed to remove guardian" }));
    throw new Error(error.error || `Failed to remove guardian: ${response.statusText}`);
  }

  // 204 No Content means successful deletion with no response body
  if (response.status === 204) {
    return;
  }

  // If there's a response body, parse it
  const result: ApiResponse<null> = await response.json();

  if (result.status === "error") {
    throw new Error(result.error || "Failed to remove guardian");
  }
}

/**
 * Search for existing guardians (for linking)
 */
export async function searchGuardians(query: string): Promise<Guardian[]> {
  const response = await fetch(`/api/guardians?search=${encodeURIComponent(query)}`);

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: "Failed to search guardians" }));
    throw new Error(error.error || `Failed to search guardians: ${response.statusText}`);
  }

  const result: PaginatedResponse<BackendGuardianProfile> = await response.json();

  if (result.status === "error") {
    throw new Error(result.error || "Failed to search guardians");
  }

  return (result.data || []).map(mapGuardianResponse);
}
