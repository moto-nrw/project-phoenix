import { createPostHandler } from "~/lib/route-wrapper";
import { apiPost } from "~/lib/api-helpers";

export const POST = createPostHandler<
  { success: boolean },
  { target_user_id: number }
>(async (request, body, token, params) => {
  const groupId = params.id as string;
  await apiPost(`/api/groups/${groupId}/transfer`, token, body);
  return { success: true };
});
