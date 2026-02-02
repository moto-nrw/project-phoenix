/**
 * Helper functions for student API request handling
 * Extracted to reduce cognitive complexity in route handlers
 */

import type { Student } from "~/lib/student-helpers";
import { prepareStudentForBackend } from "~/lib/student-helpers";
import { LOCATION_STATUSES } from "~/lib/location-helper";

/**
 * Backend student request structure
 */
export interface BackendStudentRequest {
  first_name: string;
  last_name: string;
  school_class: string;
  current_location?: string;
  notes?: string;
  tag_id?: string;
  group_id?: number;
  bus?: boolean;
  extra_info?: string;
  birthday?: string;
  health_info?: string;
  supervisor_notes?: string;
  pickup_status?: string;
  guardian_name?: string;
  guardian_contact?: string;
  guardian_email?: string;
  guardian_phone?: string;
}

/**
 * Validated student fields
 */
interface ValidatedStudentFields {
  firstName: string;
  lastName: string;
  schoolClass: string;
  guardianName?: string;
  guardianContact?: string;
}

/**
 * Guardian contact information
 */
interface GuardianContact {
  email?: string;
  phone?: string;
}

/**
 * Validates required student fields
 * Throws error if validation fails
 *
 * @param body - Student data from request
 * @returns Validated field values
 */
export function validateStudentFields(
  body: Partial<Student> & {
    guardian_email?: string;
    guardian_phone?: string;
  },
): ValidatedStudentFields {
  const firstName = body.first_name?.trim();
  const lastName = body.second_name?.trim();
  const schoolClass = body.school_class?.trim();
  const guardianName = body.name_lg?.trim();
  const guardianContact = body.contact_lg?.trim();

  if (!firstName) {
    throw new Error("First name is required");
  }

  if (!lastName) {
    throw new Error("Last name is required");
  }

  if (!schoolClass) {
    throw new Error("School class is required");
  }

  return {
    firstName,
    lastName,
    schoolClass,
    guardianName,
    guardianContact,
  };
}

/**
 * Parses guardian contact information from contact string
 * Determines if contact is email or phone number
 *
 * @param guardianEmail - Explicit guardian email
 * @param guardianPhone - Explicit guardian phone
 * @param contactLg - Contact string (may be email or phone)
 * @returns Parsed guardian contact information
 */
export function parseGuardianContact(
  guardianEmail?: string,
  guardianPhone?: string,
  contactLg?: string,
): GuardianContact {
  let email = guardianEmail;
  let phone = guardianPhone;

  if (!email && !phone && contactLg) {
    // Parse guardian contact - check if it's an email or phone
    if (contactLg.includes("@")) {
      email = contactLg;
    } else {
      phone = contactLg;
    }
  }

  return { email, phone };
}

/**
 * Builds backend student request from validated fields and frontend data
 *
 * @param validated - Validated student fields
 * @param body - Original request body
 * @param guardianContact - Parsed guardian contact
 * @returns Backend-formatted student request
 */
export function buildBackendStudentRequest(
  validated: ValidatedStudentFields,
  body: Partial<Student> & {
    guardian_email?: string;
    guardian_phone?: string;
  },
  guardianContact: GuardianContact,
): BackendStudentRequest {
  // Transform frontend format to backend format
  const backendData = prepareStudentForBackend(body);

  // Create a properly typed request object
  const request: BackendStudentRequest = {
    first_name: validated.firstName,
    last_name: validated.lastName,
    school_class: validated.schoolClass,
    current_location: backendData.current_location ?? LOCATION_STATUSES.UNKNOWN,
    notes: undefined, // Not in frontend model
    tag_id: backendData.tag_id,
    group_id: backendData.group_id,
    bus: backendData.bus,
    extra_info: backendData.extra_info,
    birthday: backendData.birthday,
    health_info: backendData.health_info,
    supervisor_notes: backendData.supervisor_notes,
    pickup_status: backendData.pickup_status,
  };

  // Add legacy guardian fields if provided
  if (validated.guardianName) {
    request.guardian_name = validated.guardianName;
  }
  if (validated.guardianContact) {
    request.guardian_contact = validated.guardianContact;
  }
  if (guardianContact.email || backendData.guardian_email) {
    request.guardian_email =
      guardianContact.email ?? backendData.guardian_email;
  }
  if (guardianContact.phone || backendData.guardian_phone) {
    request.guardian_phone =
      guardianContact.phone ?? backendData.guardian_phone;
  }

  return request;
}

/**
 * Handles privacy consent creation for a new student
 * Logs errors but doesn't throw (privacy consent is non-critical)
 *
 * @param studentId - Created student ID
 * @param privacyAccepted - Privacy consent acceptance status
 * @param retentionDays - Data retention days
 * @param apiPut - API PUT function
 * @param token - Authentication token
 * @param shouldCreate - Privacy consent helper
 * @param updateConsent - Privacy consent update function
 */
export async function handlePrivacyConsentCreation(
  studentId: number,
  privacyAccepted: boolean | undefined,
  retentionDays: number | undefined,
  apiPut: (endpoint: string, token: string, data: unknown) => Promise<unknown>,
  token: string,
  shouldCreate: (accepted?: boolean, retentionDays?: number) => boolean,
  updateConsent: (
    id: number | string,
    apiPut: (
      endpoint: string,
      token: string,
      data: unknown,
    ) => Promise<unknown>,
    token: string,
    accepted?: boolean,
    retentionDays?: number,
    operationName?: string,
  ) => Promise<void>,
): Promise<void> {
  if (!shouldCreate(privacyAccepted, retentionDays)) {
    return;
  }

  try {
    await updateConsent(
      studentId,
      apiPut,
      token,
      privacyAccepted,
      retentionDays,
      "POST Student",
    );
  } catch (consentError) {
    console.error(
      `[POST Student] Error creating privacy consent for student ${studentId}:`,
      consentError,
    );
    console.warn(
      "[POST Student] Student created but privacy consent failed. Admin can update later.",
    );
  }
}

/**
 * Builds final student response with privacy consent data
 *
 * @param mappedStudent - Mapped student from backend
 * @param studentId - Student ID
 * @param apiGet - API GET function
 * @param token - Authentication token
 * @param fetchConsent - Privacy consent fetch function
 * @returns Student with privacy consent data
 */
export async function buildStudentResponse(
  mappedStudent: Student,
  studentId: number | null | undefined,
  apiGet: (endpoint: string, token: string) => Promise<unknown>,
  token: string,
  fetchConsent: (
    studentId: string,
    apiGet: (endpoint: string, token: string) => Promise<unknown>,
    token: string,
  ) => Promise<{
    privacy_consent_accepted: boolean;
    data_retention_days: number;
  }>,
): Promise<
  Student & { privacy_consent_accepted: boolean; data_retention_days: number }
> {
  if (studentId) {
    const consentData = await fetchConsent(studentId.toString(), apiGet, token);
    return {
      ...mappedStudent,
      ...consentData,
    };
  }

  // Return with default privacy consent values if no ID
  return {
    ...mappedStudent,
    privacy_consent_accepted: false,
    data_retention_days: 30,
  };
}

/**
 * Handles student creation errors and throws appropriate messages
 *
 * @param error - Error from student creation
 */
export function handleStudentCreationError(error: unknown): never {
  // Check for permission errors (403 Forbidden)
  if (error instanceof Error && error.message.includes("403")) {
    console.error("Permission denied when creating student:", error);
    throw new Error(
      "Permission denied: You need the 'users:create' permission to create students.",
    );
  }

  // Check for validation errors (400)
  if (error instanceof Error && error.message.includes("400")) {
    const errorMessage = error.message;
    console.error("Validation error when creating student:", errorMessage);

    // Extract specific error messages
    const validationErrors: Record<string, string> = {
      "first name is required": "First name is required",
      "school class is required": "School class is required",
      "guardian name is required": "Guardian name is required",
      "guardian contact is required": "Guardian contact is required",
    };

    for (const [key, message] of Object.entries(validationErrors)) {
      if (errorMessage.includes(key)) {
        throw new Error(message);
      }
    }
  }

  // Re-throw other errors
  throw error;
}
