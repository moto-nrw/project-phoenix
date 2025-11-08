// app/api/active/visits/student/[studentId]/current/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === "string";
}

/**
 * Handler for GET /api/active/visits/student/[studentId]/current
 * Returns the current visit for a specific student
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.studentId)) {
      throw new Error("Invalid studentId parameter");
    }

    // Fetch student's current visit from the API
    return await apiGet(
      `/active/visits/student/${params.studentId}/current`,
      token,
    );
  },
);
