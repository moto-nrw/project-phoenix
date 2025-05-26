// lib/student-helpers.ts
// Type definitions and helper functions for students

// Backend types (from Go structs)
export interface BackendStudent {
    id: number;
    person_id: number;
    first_name: string;
    last_name: string;
    tag_id?: string;
    school_class: string;
    location: string;
    guardian_name: string;
    guardian_contact: string;
    guardian_email?: string;
    guardian_phone?: string;
    group_id?: number;
    group_name?: string;
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
    in_house: boolean;
    wc?: boolean;
    school_yard?: boolean;
    bus?: boolean;
    name_lg?: string;
    contact_lg?: string;
    custom_users_id?: string;
    // Additional fields for access control
    has_full_access?: boolean;
    group_supervisors?: SupervisorContact[];
}

// Mapping functions
export function mapStudentResponse(backendStudent: BackendStudent): Student {
    // Construct the full name from first and last name
    const firstName = backendStudent.first_name || '';
    const lastName = backendStudent.last_name || '';
    const name = `${firstName} ${lastName}`.trim();
    
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
        in_house: backendStudent.location === "In House",
        wc: backendStudent.location === "WC",
        school_yard: backendStudent.location === "School Yard",
        bus: backendStudent.location === "Bus",
        name_lg: backendStudent.guardian_name,
        contact_lg: backendStudent.guardian_contact,
        custom_users_id: undefined, // Not provided by backend
    };
    
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
}): Partial<BackendStudent> {
    // Calculate location string from boolean flags
    let location = "Unknown";
    if (student.in_house) location = "In House";
    else if (student.wc) location = "WC";
    else if (student.school_yard) location = "School Yard";
    else if (student.bus) location = "Bus";

    return {
        id: student.id ? parseInt(student.id, 10) : undefined,
        first_name: student.first_name,
        last_name: student.second_name, // Map second_name to last_name for backend
        school_class: student.school_class,
        location: location,
        guardian_name: student.name_lg,
        guardian_contact: student.contact_lg,
        group_id: student.group_id ? parseInt(student.group_id, 10) : undefined,
        tag_id: student.tag_id,
        guardian_email: student.guardian_email,
        guardian_phone: student.guardian_phone,
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
    if (student.in_house) return 'Anwesend';
    if (student.wc) return 'Toilette';
    if (student.school_yard) return 'Schulhof';
    if (student.bus) return 'Bus';
    return 'Abwesend';
}

export function getStatusColor(student: Student): string {
    if (student.in_house) return 'green';
    if (student.wc) return 'blue';
    if (student.school_yard) return 'yellow';
    if (student.bus) return 'purple';
    return 'red';
}