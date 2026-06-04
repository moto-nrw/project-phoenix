// app/api/activities/supervisors/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import { mapSupervisorResponse } from "~/lib/activity-helpers";

interface FallbackSupervisor {
  id: number | string;
  first_name?: string;
  last_name?: string;
  person?: {
    first_name?: string;
    last_name?: string;
  };
}

function getFallbackSupervisorName(supervisor: FallbackSupervisor): string {
  const firstName = supervisor.first_name ?? supervisor.person?.first_name;
  const lastName = supervisor.last_name ?? supervisor.person?.last_name;

  if (firstName && lastName) {
    return `${firstName} ${lastName}`;
  }

  return `Teacher ${supervisor.id}`;
}

/**
 * Handler for GET /api/activities/supervisors
 * Returns a list of available supervisors (teachers/staff)
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    // Try fetching from the backend activities API endpoint first
    try {
      const response = await apiGet<{ data?: unknown[] } | unknown[]>(
        "/api/activities/supervisors/available",
        token,
      );

      // Handle response structure with more flexible error checking
      if (response) {
        // If response has a data property that is an array
        if (
          typeof response === "object" &&
          "data" in response &&
          Array.isArray(response.data)
        ) {
          const mapped = response.data.map((item: unknown) =>
            mapSupervisorResponse(item),
          );
          return mapped;
        }
        // If response itself is an array
        else if (Array.isArray(response)) {
          const mapped = response.map((item: unknown) =>
            mapSupervisorResponse(item),
          );
          return mapped;
        }
      }
    } catch {
      // Fall through to try the staff endpoint
    }

    // Try fetching from the canonical caregiver pool as a fallback
    try {
      const response = await apiGet<
        { data?: FallbackSupervisor[] } | FallbackSupervisor[]
      >("/api/staff/by-role?role=user", token);

      // Handle response structure with more flexible checking
      if (response) {
        // If response has a data property that is an array
        if (
          typeof response === "object" &&
          "data" in response &&
          Array.isArray(response.data)
        ) {
          const mapped = response.data.map((supervisor) => ({
            id: String(supervisor.id),
            name: getFallbackSupervisorName(supervisor),
          }));
          return mapped;
        }
        // If response itself is an array
        else if (Array.isArray(response)) {
          const mapped = response.map((supervisor) => ({
            id: String(supervisor.id),
            name: getFallbackSupervisorName(supervisor),
          }));
          return mapped;
        }
      }
    } catch {
      // Fall through to return empty array
    }

    // If all API calls failed, return empty array
    return [];
  },
);
