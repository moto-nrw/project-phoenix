import { createPostHandler } from "@/lib/route-wrapper.server";
import { apiPost } from "@/lib/api-helpers.server";

// POST /api/guardians/students/[studentId]/invite
// Invite a guardian to a student by email (staff). The backend resolves an
// existing profile/account or creates a fresh invite.
export const POST = createPostHandler(async (request, body, token, params) => {
  const { studentId } = params;

  const response = await apiPost(
    `/api/guardians/students/${String(studentId)}/invite`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type

  return response.data;
});
