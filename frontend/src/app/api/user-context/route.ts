// app/api/user-context/route.ts
// BFF (Backend-for-Frontend) endpoint for shared user context data
// Consolidates 3 API calls into 1 to eliminate redundant auth() overhead
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
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
  supervised_groups: BackendActiveGroup[];
  current_staff: BackendStaff | null;
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
 * GET /api/user-context
 *
 * BFF endpoint that fetches user context data needed across multiple pages.
 * The Go backend returns one complete projection, so one browser request maps
 * to one backend request and partial access data cannot be hidden as empty.
 *
 * Used by: /students/search, and potentially other pages that need user context
 *
 */
export const GET = createGetHandler<UserContextResponse>(
  async (_request: NextRequest, token: string) => {
    const result = await apiGet<{ data: BackendNavigationContext }>(
      "/api/me/navigation",
      token,
    );

    // Transform backend data to frontend format
    const educationalGroups =
      result.data.educational_groups.map(mapEducationalGroup);
    const supervisedGroups =
      result.data.supervised_groups.map(mapSupervisedGroup);
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
    };
  },
);
