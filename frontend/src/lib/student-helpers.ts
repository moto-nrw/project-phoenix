import type { Student } from './api';

interface BackendStudent {
  id: number;
  custom_user?: {
    first_name: string;
    second_name: string;
  };
  school_class: string;
  group?: { name: string };
  in_house: boolean;
  wc: boolean;
  school_yard: boolean;
  bus: boolean;
  name_lg: string;
  contact_lg: string;
  custom_users_id?: number;
  group_id: number;
}

/**
 * Maps a list of backend student responses to frontend student models
 */
export function mapStudentResponse(data: BackendStudent[]): Student[] {
  return data.map(student => mapSingleStudentResponse(student));
}

/**
 * Maps a single backend student response to frontend student model
 */
export function mapSingleStudentResponse(student: BackendStudent): Student {
  return {
    id: student.id.toString(),
    name: student.custom_user 
      ? `${student.custom_user.first_name} ${student.custom_user.second_name}` 
      : 'Unknown',
    // Store first_name and second_name separately
    first_name: student.custom_user?.first_name || '',
    second_name: student.custom_user?.second_name || '',
    school_class: student.school_class,
    // Use school_class as grade for display consistency
    grade: student.school_class,
    // Use student ID as studentId for clarity
    studentId: student.id.toString(),
    group_name: student.group?.name,
    group_id: student.group_id?.toString(),
    in_house: student.in_house,
    wc: student.wc || false,
    school_yard: student.school_yard || false,
    bus: student.bus || false,
    name_lg: student.name_lg || '',
    contact_lg: student.contact_lg || '',
    custom_users_id: student.custom_users_id?.toString(),
    custom_user_id: student.custom_users_id?.toString() // Include for backward compatibility
  };
}

/**
 * Prepares a student object for sending to the backend API
 */
export function prepareStudentForBackend(student: Partial<Student>): Record<string, unknown> {
  const backendStudent: Record<string, unknown> = {};
  
  // Map fields to backend model
  // School class is required - prioritize school_class, fall back to grade
  backendStudent.school_class = student.school_class || student.grade || '';
  
  // Legal guardian info is required
  backendStudent.name_lg = student.name_lg || '';
  backendStudent.contact_lg = student.contact_lg || '';
  
  // Status fields
  if (student.bus !== undefined) backendStudent.bus = student.bus;
  if (student.in_house !== undefined) backendStudent.in_house = student.in_house;
  if (student.wc !== undefined) backendStudent.wc = student.wc;
  if (student.school_yard !== undefined) backendStudent.school_yard = student.school_yard;
  
  // For updates, custom_users_id is required
  // For new students (create), the backend will create a new user
  if (student.custom_users_id) {
    backendStudent.custom_users_id = parseInt(student.custom_users_id, 10);
  } else if (student.custom_user_id) {
    // Backward compatibility
    backendStudent.custom_users_id = parseInt(student.custom_user_id, 10);
  }
  
  // IMPORTANT: Remove any reference to custom_user_id as the column doesn't exist anymore
  if ('custom_user_id' in backendStudent) {
    delete backendStudent.custom_user_id;
  }
  
  // Group ID is required
  if (student.group_id) {
    backendStudent.group_id = parseInt(student.group_id, 10);
  } else {
    // Default to group ID 1 if not provided (this should be updated based on your application logic)
    backendStudent.group_id = 1;
  }
  
  // Handle name fields - these will be used to update the CustomUser record
  if (student.first_name) {
    backendStudent.first_name = student.first_name;
  }
  
  if (student.second_name) {
    backendStudent.second_name = student.second_name;
  }
  
  return backendStudent;
}