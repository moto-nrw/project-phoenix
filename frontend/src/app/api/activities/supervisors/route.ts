// app/api/activities/supervisors/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import { mapSupervisorResponse } from "~/lib/activity-helpers";

/**
 * Handler for GET /api/activities/supervisors
 * Returns a list of available supervisors (teachers/staff)
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    try {
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

      // Try fetching from staff endpoint as a fallback
      try {
        interface StaffMember {
          id: number | string;
          person?: {
            first_name: string;
            last_name: string;
          };
        }
        const response = await apiGet<{ data?: StaffMember[] } | StaffMember[]>(
          "/api/staff?teachers_only=true",
          token,
        );

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
              name: supervisor.person
                ? `${supervisor.person.first_name} ${supervisor.person.last_name}`
                : `Teacher ${supervisor.id}`,
            }));
            return mapped;
          }
          // If response itself is an array
          else if (Array.isArray(response)) {
            const mapped = response.map((supervisor) => ({
              id: String(supervisor.id),
              name: supervisor.person
                ? `${supervisor.person.first_name} ${supervisor.person.last_name}`
                : `Teacher ${supervisor.id}`,
            }));
            return mapped;
          }
        }
      } catch {}

      // If all API calls failed, return empty array
      return [];
    } catch {
      // Return empty array instead of mock data
      return [];
    }
  },
);
