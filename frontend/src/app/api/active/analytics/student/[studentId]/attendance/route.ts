// app/api/active/analytics/student/[studentId]/attendance/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Type guard to check if parameter exists and is a string
 */
function isStringParam(param: unknown): param is string {
  return typeof param === 'string';
}

/**
 * Handler for GET /api/active/analytics/student/[studentId]/attendance
 * Returns attendance data for a specific student
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  if (!isStringParam(params.studentId)) {
    throw new Error('Invalid studentId parameter');
  }
  
  // Fetch student attendance data from the API
  return await apiGet(`/active/analytics/student/${params.studentId}/attendance`, token);
});