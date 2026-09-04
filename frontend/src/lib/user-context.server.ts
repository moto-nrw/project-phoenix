import { apiGet } from "~/lib/api-helpers.server";
import type {
  EducationalGroup,
  SupervisedGroup,
  UserContextResponse,
} from "~/lib/user-context-types";

// Backend response types
interface BackendEducationalGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
  via_substitution?: boolean;
}

interface BackendActiveGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    name: string;
  };
}

interface BackendStaff {
  id: number;
  person_id: number;
  phone?: string;
  email?: string;
}

interface BackendNavigationContext {
  educational_groups: BackendEducationalGroup[];
  supervised_groups: BackendActiveGroup[] | null;
  current_staff: BackendStaff | null;
  incomplete: boolean;
  unavailable_sections: string[];
}

/**
 * Transform backend educational group to frontend format
 */
function mapEducationalGroup(data: BackendEducationalGroup): EducationalGroup {
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

/**
 * Transform backend active group to frontend format
 */
function mapSupervisedGroup(data: BackendActiveGroup): SupervisedGroup {
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
  };
}

/**
 * Loads the navigation projection (GET /api/me/navigation) and maps it to the
 * shape the browser caches under the "user-context" SWR key. Shared by the
 * /api/user-context route and the server-side shell bootstrap (#2973).
 */
export async function loadUserContext(
  token: string,
  options?: { signal?: AbortSignal },
): Promise<UserContextResponse> {
  const result = options
    ? await apiGet<{ data: BackendNavigationContext }>(
        "/api/me/navigation",
        token,
        options,
      )
    : await apiGet<{ data: BackendNavigationContext }>(
        "/api/me/navigation",
        token,
      );

  // Transform backend data to frontend format
  const educationalGroups =
    result.data.educational_groups.map(mapEducationalGroup);
  const supervisedGroups = (result.data.supervised_groups ?? []).map(
    mapSupervisedGroup,
  );
  const currentStaff = result.data.current_staff
    ? {
        id: result.data.current_staff.id.toString(),
        personId: result.data.current_staff.person_id.toString(),
      }
    : null;

  // Pre-compute derived data for convenience
  const educationalGroupIds = educationalGroups.map((g) => g.id);
  const educationalGroupRoomNames = educationalGroups
    .map((g) => g.room?.name)
    .filter((name): name is string => !!name);
  const supervisedRoomNames = supervisedGroups
    .map((g) => g.room?.name)
    .filter((name): name is string => !!name);

  return {
    educationalGroups,
    supervisedGroups,
    currentStaff,
    educationalGroupIds,
    educationalGroupRoomNames,
    supervisedRoomNames,
    incomplete: result.data.incomplete,
    unavailableSections: result.data.unavailable_sections,
  };
}
