// app/api/active/combined/[id]/end/route.ts
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
 * Handler for POST /api/active/combined/[id]/end
 * Ends an active combined group
 */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    _body: Record<string, never>,
    token: string,
    params,
  ) => {
    if (!isStringParam(params.id)) {
      throw new Error("Invalid id parameter");
    }

    // End the combined group via the API
    return await apiPost(`/api/active/combined/${params.id}/end`, token);
  },
);
