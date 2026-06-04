import { createDeleteHandler } from "~/lib/route-wrapper.server";
import { apiDelete } from "~/lib/api-helpers.server";

export const DELETE = createDeleteHandler(async (request, token, params) => {
  const groupId = params.id as string;
  const substitutionId = params.substitutionId as string;
  await apiDelete(`/api/groups/${groupId}/transfer/${substitutionId}`, token);
  return { success: true };
});
