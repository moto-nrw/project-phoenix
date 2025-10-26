// lib/student-helpers.ts
// Type definitions and helper functions for students

import {
    LOCATION_STATUSES,
    parseLocation,
    isHomeLocation,
    isPresentLocation,
    isSchoolyardLocation,
    isTransitLocation,
} from "./location-helper";

// Scheduled checkout information
export interface ScheduledCheckoutInfo {
    id: number;
    scheduled_for: string;
    reason?: string;
    scheduled_by: string;
}

// Backend types (from Go structs)
export interface BackendStudent {
    id: number;
    person_id: number;
    first_name: string;
    last_name: string;
    tag_id?: string;
    school_class: string;
    current_location?: string | null;
    bus?: boolean;
    guardian_name: string;
    guardian_contact: string;
    guardian_email?: string;
    guardian_phone?: string;
    group_id?: number;
    group_name?: string;
    scheduled_checkout?: ScheduledCheckoutInfo;
    extra_info?: string;
    birthday?: string;
    health_info?: string;
    supervisor_notes?: string;
    created_at: string;
    updated_at: string;
}

// Supervisor contact information
export interface SupervisorContact {
    id: number;
    first_name: string;
    last_name: string;
    email?: string;
    phone?: string;
    role: string;
}

// Detailed student response with access control
export interface BackendStudentDetail extends BackendStudent {
    has_full_access: boolean;
    group_supervisors?: SupervisorContact[];
}

// Privacy consent types
export interface BackendPrivacyConsent {
    id: number;
    student_id: number;
    policy_version: string;
    accepted: boolean;
    accepted_at?: string;
    expires_at?: string;
    duration_days?: number;
    renewal_required: boolean;
    data_retention_days: number;
    details?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
}

export interface PrivacyConsent {
    id: string;
    studentId: string;
    policyVersion: string;
    accepted: boolean;
    acceptedAt?: Date;
    expiresAt?: Date;
    durationDays?: number;
    renewalRequired: boolean;
    dataRetentionDays: number;
    details?: Record<string, unknown>;
    createdAt: Date;
    updatedAt: Date;
}

// Student attendance status enum (updated to use attendance-based terminology)
// Now includes specific location details from backend
export type StudentLocation = string;

// Frontend types (mapped from backend)
export interface Student {
    id: string;
    name: string; // Derived from FirstName + SecondName
    first_name?: string;
    second_name?: string;
    school_class: string;
    grade?: string;
    studentId?: string;
    group_name?: string;
    group_id?: string;
    // Current attendance status of student
    current_location: StudentLocation;
    // Transportation method (separate from attendance)
    takes_bus?: boolean;
    bus?: boolean; // Administrative permission flag (Buskind), not attendance status
    name_lg?: string;
    contact_lg?: string;
    guardian_email?: string;
    guardian_phone?: string;
    custom_users_id?: string;
    // Privacy consent data (fetched separately)
    privacy_consent?: PrivacyConsent;
    // Privacy consent fields for form handling
    privacy_consent_accepted?: boolean;
    data_retention_days?: number;
    // Additional fields for access control
    has_full_access?: boolean;
    group_supervisors?: SupervisorContact[];
    // Extra information visible only to supervisors
    extra_info?: string;
    birthday?: string;
    health_info?: string;
    supervisor_notes?: string;
}

// Mapping functions
export function mapStudentResponse(backendStudent: BackendStudent): Student & { scheduled_checkout?: ScheduledCheckoutInfo } {
    // Construct the full name from first and last name
    const firstName = backendStudent.first_name || '';
    const lastName = backendStudent.last_name || '';
    const name = `${firstName} ${lastName}`.trim();
    
    // Map backend attendance status - backend now provides current_location directly
    const rawLocation = backendStudent.current_location?.trim();
    const current_location: StudentLocation = rawLocation && rawLocation.length > 0
        ? rawLocation
        : LOCATION_STATUSES.UNKNOWN;
    
    const mapped = {
        id: String(backendStudent.id),
        name: name,
        first_name: backendStudent.first_name,
        second_name: backendStudent.last_name, // Map last_name to second_name for frontend compatibility
        school_class: backendStudent.school_class,
        grade: undefined, // Not provided by backend
        studentId: backendStudent.tag_id,
        group_name: backendStudent.group_name,
        group_id: backendStudent.group_id ? String(backendStudent.group_id) : undefined,
        // New attendance-based system
        current_location: current_location,
        takes_bus: undefined, // TODO: Map from backend when available
        bus: backendStudent.bus ?? false, // Administrative permission flag (Buskind)
        name_lg: backendStudent.guardian_name,
        contact_lg: backendStudent.guardian_contact,
        guardian_email: backendStudent.guardian_email,
        guardian_phone: backendStudent.guardian_phone,
        custom_users_id: undefined, // Not provided by backend
        extra_info: backendStudent.extra_info,
        birthday: backendStudent.birthday,
        health_info: backendStudent.health_info,
        supervisor_notes: backendStudent.supervisor_notes,
    };

    // Add scheduled checkout info if present
    if (backendStudent.scheduled_checkout) {
        return {
            ...mapped,
            scheduled_checkout: backendStudent.scheduled_checkout
        };
    }
    
    return mapped;
}

// Map array of students
export function mapStudentsResponse(backendStudents: BackendStudent[]): Student[] {
    return backendStudents.map(mapStudentResponse);
}

// Map a single student
export function mapSingleStudentResponse(response: { data: BackendStudent }): Student {
    return mapStudentResponse(response.data);
}

// Map student detail response (includes access control info)
export function mapStudentDetailResponse(backendStudent: BackendStudentDetail): Student {
    // First map the basic student data
    const student = mapStudentResponse(backendStudent);
    
    // Then add the additional fields
    student.has_full_access = backendStudent.has_full_access;
    student.group_supervisors = backendStudent.group_supervisors;
    
    return student;
}

// Prepare frontend student for backend
export function prepareStudentForBackend(student: Partial<Student> & {
    tag_id?: string;
    guardian_email?: string;
    guardian_phone?: string;
    extra_info?: string;
    birthday?: string;
    health_info?: string;
    supervisor_notes?: string;
}): Partial<BackendStudent> {
    return {
        id: student.id ? parseInt(student.id, 10) : undefined,
        first_name: student.first_name,
        last_name: student.second_name, // Map second_name to last_name for backend
        school_class: student.school_class,
        current_location: student.current_location,
        bus: student.bus ?? false, // Send bus as a separate field
        guardian_name: student.name_lg,
        guardian_contact: student.contact_lg,
        group_id: student.group_id ? parseInt(student.group_id, 10) : undefined,
        tag_id: student.tag_id,
        guardian_email: student.guardian_email,
        guardian_phone: student.guardian_phone,
        extra_info: student.extra_info,
        birthday: student.birthday,
        health_info: student.health_info,
        supervisor_notes: student.supervisor_notes,
    };
}

// Request/Response types
export interface CreateStudentRequest {
    first_name?: string;
    second_name?: string; // Will be mapped to last_name for backend
    school_class?: string;
    group_id?: number;
    name_lg?: string; // Guardian name
    contact_lg?: string; // Guardian contact
    tag_id?: string; // Optional RFID
    guardian_email?: string;
    guardian_phone?: string;
    extra_info?: string;
}

export interface UpdateStudentRequest {
    first_name?: string;
    second_name?: string; // Will be mapped to last_name for backend
    school_class?: string;
    group_id?: string;
    name_lg?: string; // Guardian name
    contact_lg?: string; // Guardian contact
    tag_id?: string;
    guardian_email?: string;
    guardian_phone?: string;
    extra_info?: string;
    birthday?: string;
    health_info?: string;
    supervisor_notes?: string;
}

// Backend request type (for actual API calls)
export interface BackendUpdateRequest {
    first_name?: string;
    last_name?: string;
    tag_id?: string;
    school_class?: string;
    guardian_name?: string;
    guardian_contact?: string;
    guardian_email?: string;
    guardian_phone?: string;
    group_id?: number;
    extra_info?: string;
    birthday?: string;
    health_info?: string;
    supervisor_notes?: string;
}

// Map privacy consent from backend to frontend
export function mapPrivacyConsentResponse(backendConsent: BackendPrivacyConsent): PrivacyConsent {
    return {
        id: String(backendConsent.id),
        studentId: String(backendConsent.student_id),
        policyVersion: backendConsent.policy_version,
        accepted: backendConsent.accepted,
        acceptedAt: backendConsent.accepted_at ? new Date(backendConsent.accepted_at) : undefined,
        expiresAt: backendConsent.expires_at ? new Date(backendConsent.expires_at) : undefined,
        durationDays: backendConsent.duration_days,
        renewalRequired: backendConsent.renewal_required,
        dataRetentionDays: backendConsent.data_retention_days,
        details: backendConsent.details,
        createdAt: new Date(backendConsent.created_at),
        updatedAt: new Date(backendConsent.updated_at),
    };
}

// Map frontend update request to backend format
export function mapUpdateRequestToBackend(request: UpdateStudentRequest): BackendUpdateRequest {
    const backendRequest: BackendUpdateRequest = {};

    if (request.first_name !== undefined) {
        backendRequest.first_name = request.first_name;
    }
    if (request.second_name !== undefined) {
        backendRequest.last_name = request.second_name;
    }
    if (request.tag_id !== undefined) {
        backendRequest.tag_id = request.tag_id;
    }
    if (request.school_class !== undefined) {
        backendRequest.school_class = request.school_class;
    }
    if (request.name_lg !== undefined) {
        backendRequest.guardian_name = request.name_lg;
    }
    if (request.contact_lg !== undefined) {
        backendRequest.guardian_contact = request.contact_lg;
    }
    if (request.guardian_email !== undefined) {
        backendRequest.guardian_email = request.guardian_email;
    }
    if (request.guardian_phone !== undefined) {
        backendRequest.guardian_phone = request.guardian_phone;
    }
    if (request.group_id !== undefined) {
        backendRequest.group_id = parseInt(request.group_id, 10);
    }
    if (request.extra_info !== undefined) {
        backendRequest.extra_info = request.extra_info;
    }
    if (request.birthday !== undefined) {
        backendRequest.birthday = request.birthday;
    }
    if (request.health_info !== undefined) {
        backendRequest.health_info = request.health_info;
    }
    if (request.supervisor_notes !== undefined) {
        backendRequest.supervisor_notes = request.supervisor_notes;
    }

    return backendRequest;
}

// Helper functions
export function formatStudentName(student: Student): string {
    if (student.name) return student.name;
    
    const fallback = [student.first_name, student.second_name]
        .filter(Boolean)
        .join(' ') || 'Unnamed Student';
    
    return fallback;
}

export function formatStudentStatus(student: Student): string {
    const parsed = parseLocation(student.current_location);
    return parsed.room ?? parsed.status ?? LOCATION_STATUSES.UNKNOWN;
}

/**
 * Extracts guardian contact information with clear precedence rules.
 * This function provides a consistent way to determine which contact
 * information to display for a student's guardian.
 * 
 * Precedence order:
 * 1. guardian_email - Primary contact method (if available)
 * 2. contact_lg - Legacy contact field (fallback)
 * 3. Empty string - Final fallback when no contact info is available
 * 
 * @param studentData - Object containing guardian contact fields
 * @returns The most appropriate guardian contact information
 */
export function extractGuardianContact(studentData: {
    guardian_email?: string;
    contact_lg?: string;
}): string {
    // First priority: Use guardian_email if available
    if (studentData.guardian_email) {
        return studentData.guardian_email;
    }
    
    // Second priority: Fall back to legacy contact_lg field
    if (studentData.contact_lg) {
        return studentData.contact_lg;
    }
    
    // Final fallback: Return empty string when no contact info available
    return "";
}

export function getStatusColor(student: Student): string {
    if (isPresentLocation(student.current_location)) return 'green';
    if (isSchoolyardLocation(student.current_location)) return 'yellow';
    if (isTransitLocation(student.current_location)) return 'purple';
    if (isHomeLocation(student.current_location)) return 'red';
    return 'gray';
}
