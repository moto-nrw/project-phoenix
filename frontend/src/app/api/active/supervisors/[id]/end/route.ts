// app/api/active/supervisors/[id]/end/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === "string";
}

/**
 * Handler for POST /api/active/supervisors/[id]/end
 * Ends supervision for a specific supervisor
 */
export const POST = createPostHandler(
  async (_request: NextRequest, _body: undefined, token: string, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // End supervision via the API
    return await apiPost(`/active/supervisors/${params.id}/end`, token);
  },
);
