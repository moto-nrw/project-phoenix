import { createGetHandler, createPostHandler } from "@/lib/route-wrapper";
import { apiGet, apiPost } from "@/lib/api-helpers";

// GET /api/guardians/students/[studentId]/guardians - Get all guardians for a student
export const GET = createGetHandler(async (request, token, params) => {
  const { studentId } = params;

  const response = await apiGet(
    `/api/guardians/students/${String(studentId)}/guardians`,
    token,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});

// POST /api/guardians/students/[studentId]/guardians - Link guardian to student
export const POST = createPostHandler(async (request, body, token, params) => {
  const { studentId } = params;

  const response = await apiPost(
    `/api/guardians/students/${String(studentId)}/guardians`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});
