import type { NextRequest } from "next/server";
import { apiPut, apiDelete } from "~/lib/api-helpers";
import {
  createPutHandler,
  createDeleteHandler,
  isStringParam,
} from "~/lib/route-wrapper";

interface UpdateAbsenceBody {
  absence_type?: string;
  date_start?: string;
  date_end?: string;
  half_day?: boolean;
  note?: string;
}

/**
 * PUT /api/time-tracking/absences/{id}
 * Update an absence
 */
export const PUT = createPutHandler<unknown, UpdateAbsenceBody>(
  async (
    _request: NextRequest,
    body: UpdateAbsenceBody,
    token: string,
    params,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid absence ID");
    }

    const response = await apiPut<{ data: unknown }>(
      `/api/time-tracking/absences/${params.id}`,
      token,
      body,
    );
    return response.data;
  },
);

/**
 * DELETE /api/time-tracking/absences/{id}
 * Delete an absence
 */
export const DELETE = createDeleteHandler<void>(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid absence ID");
    }

    await apiDelete(`/api/time-tracking/absences/${params.id}`, token);
  },
);
