// lib/usercontext-helpers.ts

import type { EducationalGroup } from "./user-context-types";

// Frontend types for UserContext (EducationalGroup is shared with the
// user-context BFF route — single definition in user-context-types.ts)
export type { EducationalGroup };

export interface ActiveGroup {
  id: string;
  name: string;
  room_id?: string;
  room?: {
    id: string;
    name: string;
  };
}

interface Person {
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

// Backend types (what comes from the API)
export interface BackendEducationalGroup {
  id: number;
  name: string;
  via_substitution?: boolean;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
}

export interface NavigationEducationalGroup {
  id: string;
  name: string;
  via_substitution?: boolean;
  is_personal?: boolean;
  room_id?: string;
  room_name?: string;
  /** Namen der Gruppenleitung, nur wenn eine zugeordnet ist (#4351). */
  leaders?: string[];
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

// Helper functions to transform backend data to frontend types
export function mapEducationalGroupResponse(
  data: BackendEducationalGroup,
): EducationalGroup {
  return {
    id: data.id.toString(),
    name: data.name,
    roomId: data.room_id?.toString(),
    room: data.room
      ? {
          id: data.room.id.toString(),
          name: data.room.name,
        }
      : undefined,
    viaSubstitution: data.via_substitution ?? false,
  };
}

export function mapNavigationEducationalGroupResponse(
  data: NavigationEducationalGroup,
): EducationalGroup {
  return {
    id: data.id,
    name: data.name,
    roomId: data.room_id,
    room:
      data.room_id && data.room_name
        ? { id: data.room_id, name: data.room_name }
        : undefined,
    viaSubstitution: data.via_substitution ?? false,
  };
}

// Shared mapper for group responses with room information
function mapGroupWithRoom<
  T extends {
    id: number;
    name: string;
    room_id?: number | null;
    room?: { id: number; name: string } | null;
  },
>(
  data: T,
): {
  id: string;
  name: string;
  room_id?: string;
  room?: { id: string; name: string };
} {
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
  return mapGroupWithRoom(data);
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
