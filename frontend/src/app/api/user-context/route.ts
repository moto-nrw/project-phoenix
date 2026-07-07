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
 * This eliminates 3+ separate auth() calls by making one auth() call
 * and then fetching data in parallel from the Go backend.
 *
 * Used by: /students/search, and potentially other pages that need user context
 *
 * Performance improvement: ~900ms → ~350ms (60% faster)
 */
export const GET = createGetHandler<UserContextResponse>(
  async (_request: NextRequest, token: string) => {
    // Fetch all three endpoints in parallel
    const [groupsResult, supervisedResult, staffResult] = await Promise.all([
      // Fetch user's educational groups (OGS groups)
      apiGet<{ data: BackendEducationalGroup[] }>(
        "/api/me/groups",
        token,
      ).catch(() => ({ data: [] as BackendEducationalGroup[] })),

      // Fetch user's supervised groups (active sessions)
      apiGet<{ data: BackendActiveGroup[] | null }>(
        "/api/me/groups/supervised",
        token,
      ).catch(() => ({ data: null as BackendActiveGroup[] | null })),

      // Fetch current staff info
      apiGet<{ data: BackendStaff }>("/api/me/staff", token).catch(() => ({
        data: null as BackendStaff | null,
      })),
    ]);

    // Transform backend data to frontend format
    const educationalGroups = (groupsResult.data ?? []).map(
      mapEducationalGroup,
    );
    const supervisedGroups = Array.isArray(supervisedResult.data)
      ? supervisedResult.data.map(mapSupervisedGroup)
      : [];
    const currentStaff = staffResult.data
      ? {
          id: staffResult.data.id.toString(),
          personId: staffResult.data.person_id.toString(),
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
