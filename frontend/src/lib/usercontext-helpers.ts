// lib/usercontext-helpers.ts

// Frontend types for UserContext
export interface EducationalGroup {
  id: string;
  name: string;
  room_id?: string;
  room?: {
    id: string;
    name: string;
  };
  viaSubstitution?: boolean; // True if access is through temporary transfer
}

export interface ActivityGroup {
  id: string;
  name: string;
  room_id?: string;
  room?: {
    id: string;
    name: string;
  };
}

export interface ActiveGroup {
  id: string;
  name: string;
  room_id?: string;
  room?: {
    id: string;
    name: string;
  };
}

export interface UserProfile {
  id: string;
  email: string;
  username: string;
  name: string;
  active: boolean;
}

export interface Person {
  id: string;
  first_name: string;
  last_name: string;
  date_of_birth?: string;
}

export interface Staff {
  id: string;
  person_id: string;
  phone?: string;
  email?: string;
  emergency_contact?: string;
  emergency_phone?: string;
  person?: Person;
}

export interface Teacher {
  id: string;
  staff_id: string;
  staff?: Staff;
}

// Backend types (what comes from the API)
export interface BackendEducationalGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
  via_substitution?: boolean;
}

export interface BackendActivityGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
}

export interface BackendActiveGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
}

export interface BackendUserProfile {
  id: number;
  email: string;
  username: string;
  name: string;
  active: boolean;
}

export interface BackendPerson {
  id: number;
  first_name: string;
  last_name: string;
  date_of_birth?: string;
}

export interface BackendStaff {
  id: number;
  person_id: number;
  phone?: string;
  email?: string;
  emergency_contact?: string;
  emergency_phone?: string;
  person?: BackendPerson;
}

export interface BackendTeacher {
  id: number;
  staff_id: number;
  staff?: BackendStaff;
}

// Helper functions to transform backend data to frontend types
export function mapEducationalGroupResponse(
  data: BackendEducationalGroup,
): EducationalGroup {
  return {
    id: data.id.toString(),
    name: data.name,
    room_id: data.room_id?.toString(),
    room: data.room
      ? {
          id: data.room.id.toString(),
          name: data.room.name,
        }
      : undefined,
    viaSubstitution: data.via_substitution ?? false,
  };
}

export function mapActivityGroupResponse(
  data: BackendActivityGroup,
): ActivityGroup {
  return {
    id: data.id.toString(),
    name: data.name,
    room_id: data.room_id?.toString(),
    room: data.room
      ? {
          id: data.room.id.toString(),
          name: data.room.name,
        }
      : undefined,
  };
}

export function mapActiveGroupResponse(data: BackendActiveGroup): ActiveGroup {
  return {
    id: data.id.toString(),
    name: data.name,
    room_id: data.room_id?.toString(),
    room: data.room
      ? {
          id: data.room.id.toString(),
          name: data.room.name,
        }
      : undefined,
  };
}

export function mapUserProfileResponse(data: BackendUserProfile): UserProfile {
  return {
    id: data.id.toString(),
    email: data.email,
    username: data.username,
    name: data.name,
    active: data.active,
  };
}

export function mapPersonResponse(data: BackendPerson): Person {
  return {
    id: data.id.toString(),
    first_name: data.first_name,
    last_name: data.last_name,
    date_of_birth: data.date_of_birth,
  };
}

export function mapStaffResponse(data: BackendStaff): Staff {
  return {
    id: data.id.toString(),
    person_id: data.person_id.toString(),
    phone: data.phone,
    email: data.email,
    emergency_contact: data.emergency_contact,
    emergency_phone: data.emergency_phone,
    person: data.person ? mapPersonResponse(data.person) : undefined,
  };
}

export function mapTeacherResponse(data: BackendTeacher): Teacher {
  return {
    id: data.id.toString(),
    staff_id: data.staff_id.toString(),
    staff: data.staff ? mapStaffResponse(data.staff) : undefined,
  };
}
