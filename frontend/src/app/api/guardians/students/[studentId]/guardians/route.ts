import { NextRequest } from "next/server";
import { createGetHandler, createPostHandler } from "@/lib/route-wrapper";
import { apiGet, apiPost } from "@/lib/api-helpers";

// GET /api/guardians/students/[studentId]/guardians - Get all guardians for a student
export const GET = createGetHandler(async (request, token, params) => {
  const { studentId } = await params;

  const response = await apiGet(`/api/guardians/students/${studentId}/guardians`, token);
  return response.data;
});

// POST /api/guardians/students/[studentId]/guardians - Link guardian to student
export const POST = createPostHandler(async (request, token, params) => {
  const { studentId } = await params;
  const body = await request.json();

  const response = await apiPost(
    `/api/guardians/students/${studentId}/guardians`,
    body,
    token
  );
  return response.data;
});
