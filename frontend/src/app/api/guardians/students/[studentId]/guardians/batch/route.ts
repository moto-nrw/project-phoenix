import { createPostHandler } from "@/lib/route-wrapper.server";
import { apiPost } from "@/lib/api-helpers.server";

// POST /api/guardians/students/[studentId]/guardians/batch
// Atomically creates (or links) one or more guardians for an existing student
// in a single backend transaction (#819).
export const POST = createPostHandler(async (request, body, token, params) => {
  const { studentId } = params;

  const response = await apiPost(
    `/api/guardians/students/${String(studentId)}/guardians/batch`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type

  return response.data;
});
