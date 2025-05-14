// app/api/active/analytics/student/[studentId]/attendance/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/active/analytics/student/[studentId]/attendance
 * Returns attendance data for a specific student
 */
export const GET = createGetHandler(async (_request: NextRequest, token: string, params) => {
  const studentId = params.studentId as string;
  
  // Fetch student attendance data from the API
  return await apiGet(`/active/analytics/student/${studentId}/attendance`, token);
});