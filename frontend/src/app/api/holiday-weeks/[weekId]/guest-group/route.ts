import type { NextRequest } from "next/server";
import { apiPut } from "~/lib/api-helpers.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { createPutHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy PUT /api/holiday-weeks/{weekId}/guest-group → backend (#4666): die
 * ausrichtende OGS wählt die Gruppe für die Feriengäste einer Woche. Das
 * Backend übernimmt die bestätigten Feriengäste sofort in die Gruppe und
 * schreibt die Übernahme ins Zugriffsprotokoll.
 */
export const PUT = createPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const weekId = requirePathSegmentParam(params, "weekId");
    const response = await apiPut<{ data: unknown }>(
      `/api/holiday-weeks/${weekId}/guest-group`,
      token,
      body,
    );
    return response.data;
  },
);
