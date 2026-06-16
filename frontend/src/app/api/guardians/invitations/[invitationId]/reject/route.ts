import { createPostHandler } from "@/lib/route-wrapper.server";
import { apiPost } from "@/lib/api-helpers.server";

// POST /api/guardians/invitations/[invitationId]/reject
export const POST = createPostHandler(async (request, body, token, params) => {
  const { invitationId } = params;

  const response = await apiPost(
    `/api/guardians/invitations/${String(invitationId)}/reject`,
    token,
    {},
  );
  // @ts-expect-error - API helper returns unknown type

  return response.data;
});
