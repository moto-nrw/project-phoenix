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
    request.guardian_email = guardianContact.email ?? backendData.guardian_email;
  }
  if (guardianContact.phone || backendData.guardian_phone) {
    request.guardian_phone = guardianContact.phone ?? backendData.guardian_phone;
  }

  return request;
}
