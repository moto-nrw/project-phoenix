// app/api/active/visits/student/[studentId]/route.ts
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
 * Handler for GET /api/active/visits/student/[studentId]
 * Returns visits for a specific student
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string, params) => {
    if (!isStringParam(params.studentId)) {
      throw new Error("Invalid studentId parameter");
    }

    // Fetch student visits from the API
    return await apiGet(`/api/active/visits/student/${params.studentId}`, token);
  },
);
