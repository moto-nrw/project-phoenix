// app/api/active/supervisors/staff/[staffId]/active/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === "string";
}

/**
 * Handler for GET /api/active/supervisors/staff/[staffId]/active
 * Returns active supervisions for a specific staff member
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.staffId)) {
      throw new Error("Invalid staffId parameter");
    }

    // The backend wraps the list in { status, data, message } — unwrap it so
    // clients receive the plain array (coreFetch reads response.data once).
    const response = await apiGet<{ data: unknown }>(
      `/api/active/supervisors/staff/${params.staffId}/active`,
      token,
    );
    return response.data;
  },
);
