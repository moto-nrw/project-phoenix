import type { NextRequest } from "next/server";

import { apiGet, apiPut } from "~/lib/api-helpers.server";
import { createGetHandler, createPutHandler } from "~/lib/route-wrapper.server";

// Backend response shape; kept loose because the schedule grew rotation
// metadata (mode, model, rotation_length, weekly_totals, anchor) and the
// proxy just forwards whatever the server returns.
type ScheduleResponse = Record<string, unknown>;

// Both modes are accepted: { mode: "template", model_id }
// or { mode: "custom", rotation_length, entries: [...], save_as_template }.
type UpdateScheduleBody = Record<string, unknown>;

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
