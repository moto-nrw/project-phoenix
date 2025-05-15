// lib/student-helpers.ts
// Type definitions and helper functions for students

// Backend types (from Go structs)
export interface BackendStudent {
    id: number;
    first_name?: string;
    second_name?: string;
    school_class?: string;
    grade?: string;
    student_id?: string;
    group_id?: number;
    group_name?: string;
    created_at: string;
    updated_at: string;
    in_house: boolean;
    wc?: boolean;
    school_yard?: boolean;
    bus?: boolean;
    name_lg?: string; // Legal Guardian name
    contact_lg?: string; // Legal Guardian contact
    custom_users_id?: number;
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
}

// Mapping functions
export function mapStudentResponse(backendStudent: BackendStudent): Student {
    // Construct the full name from first and second name
    const name = [backendStudent.first_name, backendStudent.second_name]
        .filter(Boolean)
        .join(' ');
    
    return {
        id: String(backendStudent.id),
        name: name ?? 'Unnamed Student',
        first_name: backendStudent.first_name,
        second_name: backendStudent.second_name,
        school_class: backendStudent.school_class ?? '',
        grade: backendStudent.grade,
        studentId: backendStudent.student_id,
        group_name: backendStudent.group_name,
        group_id: backendStudent.group_id ? String(backendStudent.group_id) : undefined,
        in_house: backendStudent.in_house,
        wc: backendStudent.wc,
        school_yard: backendStudent.school_yard,
        bus: backendStudent.bus,
        name_lg: backendStudent.name_lg,
        contact_lg: backendStudent.contact_lg,
        custom_users_id: backendStudent.custom_users_id ? String(backendStudent.custom_users_id) : undefined,
    };
}

// Map array of students
export function mapStudentsResponse(backendStudents: BackendStudent[]): Student[] {
    return backendStudents.map(mapStudentResponse);
}

// Map a single student
export function mapSingleStudentResponse(response: { data: BackendStudent }): Student {
    return mapStudentResponse(response.data);
}

// Prepare frontend student for backend
export function prepareStudentForBackend(student: Partial<Student>): Partial<BackendStudent> {
    return {
        id: student.id ? parseInt(student.id, 10) : undefined,
        first_name: student.first_name,
        second_name: student.second_name,
        school_class: student.school_class,
        grade: student.grade,
        student_id: student.studentId,
        group_id: student.group_id ? parseInt(student.group_id, 10) : undefined,
        in_house: student.in_house,
        wc: student.wc,
        school_yard: student.school_yard,
        bus: student.bus,
        name_lg: student.name_lg,
        contact_lg: student.contact_lg,
        custom_users_id: student.custom_users_id ? parseInt(student.custom_users_id, 10) : undefined,
    };
}

// Request/Response types
export interface CreateStudentRequest {
    first_name?: string;
    second_name?: string;
    school_class?: string;
    grade?: string;
    student_id?: string;
    group_id?: number;
    in_house?: boolean;
    name_lg?: string;
    contact_lg?: string;
    custom_users_id?: number;
}

export interface UpdateStudentRequest {
    first_name?: string;
    second_name?: string;
    school_class?: string;
    grade?: string;
    student_id?: string;
    group_id?: number;
    in_house?: boolean;
    name_lg?: string;
    contact_lg?: string;
    custom_users_id?: number;
}

// Helper functions
export function formatStudentName(student: Student): string {
    if (student.name) return student.name;
    
    return [student.first_name, student.second_name]
        .filter(Boolean)
        .join(' ') || 'Unnamed Student';
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