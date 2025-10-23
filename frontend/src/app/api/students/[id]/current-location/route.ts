// app/api/students/[id]/current-location/route.ts
import type { NextRequest } from "next/server";
import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-client";
import type {
  BackendStudentLocationStatus,
  StudentLocationStatus,
} from "~/lib/student-location-helpers";
import { mapLocationStatus } from "~/lib/student-location-helpers";

interface BackendLocationResponse {
  location_status?: BackendStudentLocationStatus | null;
  current_room?: string;
  scheduled_checkout?: {
    id: number;
    scheduled_for: string;
    reason?: string;
    scheduled_by: string;
  } | null;
}

interface FrontendLocationResponse {
  location_status: StudentLocationStatus | null;
  current_room?: string;
  scheduled_checkout?: BackendLocationResponse["scheduled_checkout"];
}

export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ): Promise<FrontendLocationResponse> => {
    const studentId = params.id as string | undefined;

    if (!studentId) {
      throw new Error("Student ID is required");
    }

    const response = await apiGet<BackendLocationResponse>(
      `/api/students/${studentId}/current-location`,
      token,
    );

    const backendLocation = response?.data ?? response;

    const locationStatus = mapLocationStatus(
      backendLocation?.location_status ?? null,
    );

    return {
      location_status: locationStatus,
      current_room: backendLocation?.current_room,
      scheduled_checkout: backendLocation?.scheduled_checkout ?? undefined,
    };
  },
);
