import type {
  SchoolSearchMatch,
  SchoolSearchMatchField,
  SchoolSearchResult,
} from "~/lib/school-search";

/** The backend's search answer (snake_case, IDs as strings). */
export interface BackendSchoolSearchResult {
  children: Array<{
    id: string;
    first_name: string;
    last_name: string;
    school_class: string;
    group_name?: string;
    match: BackendMatch;
  }>;
  contacts: Array<{
    id: string;
    first_name: string;
    last_name: string;
    match: BackendMatch;
    children: Array<{
      id: string;
      first_name: string;
      last_name: string;
      school_class: string;
      relationship_type: string;
    }>;
  }>;
  staff: Array<{ id: string; first_name: string; last_name: string }>;
  rooms: Array<{ id: string; name: string; building?: string }>;
  groups: Array<{ id: string; name: string }>;
}

interface BackendMatch {
  field: SchoolSearchMatchField;
  label?: string;
  text?: string;
}

function mapMatch(match: BackendMatch): SchoolSearchMatch {
  return {
    field: match.field,
    ...(match.label ? { label: match.label } : {}),
    ...(match.text ? { text: match.text } : {}),
  };
}

/** Maps the backend answer onto the frontend shape. */
export function mapSchoolSearchResult(
  result: BackendSchoolSearchResult,
): SchoolSearchResult {
  return {
    children: (result.children ?? []).map((child) => ({
      id: child.id,
      firstName: child.first_name,
      lastName: child.last_name,
      schoolClass: child.school_class,
      ...(child.group_name ? { groupName: child.group_name } : {}),
      match: mapMatch(child.match),
    })),
    contacts: (result.contacts ?? []).map((contact) => ({
      id: contact.id,
      firstName: contact.first_name,
      lastName: contact.last_name,
      match: mapMatch(contact.match),
      children: (contact.children ?? []).map((child) => ({
        id: child.id,
        firstName: child.first_name,
        lastName: child.last_name,
        schoolClass: child.school_class,
        relationshipType: child.relationship_type,
      })),
    })),
    staff: (result.staff ?? []).map((member) => ({
      id: member.id,
      firstName: member.first_name,
      lastName: member.last_name,
    })),
    rooms: (result.rooms ?? []).map((room) => ({
      id: room.id,
      name: room.name,
      ...(room.building ? { building: room.building } : {}),
    })),
    groups: (result.groups ?? []).map((group) => ({
      id: group.id,
      name: group.name,
    })),
  };
}
