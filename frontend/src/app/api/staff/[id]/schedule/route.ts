import type { NextRequest } from "next/server";
import { apiGet, apiPut } from "~/lib/api-helpers";
import { createGetHandler, createPutHandler } from "~/lib/route-wrapper";

interface ScheduleEntry {
  day_of_week: number;
  target_minutes: number;
}

interface UpdateScheduleBody {
  entries: ScheduleEntry[];
}

interface ScheduleResponse {
  entries: Array<{
    id: number;
    day_of_week: number;
    target_minutes: number;
    valid_from: string;
  }>;
  weekly_total: number;
}

export const GET = createGetHandler<ScheduleResponse>(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const response = await apiGet<{ data: ScheduleResponse }>(
      `/api/staff/${id}/schedule`,
      token,
    );
    return response.data;
  },
);

export const PUT = createPutHandler<ScheduleResponse, UpdateScheduleBody>(
  async (
    _request: NextRequest,
    body: UpdateScheduleBody,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    const response = await apiPut<{ data: ScheduleResponse }>(
      `/api/staff/${id}/schedule`,
      token,
      body,
    );
    return response.data;
  },
);
