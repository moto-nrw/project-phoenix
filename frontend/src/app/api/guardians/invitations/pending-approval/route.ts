import { createGetHandler } from "@/lib/route-wrapper.server";
import { apiGet } from "@/lib/api-helpers.server";

// GET /api/guardians/invitations/pending-approval
// Staff approval queue: parent-initiated invites awaiting approval.
export const GET = createGetHandler(async (request, token) => {
  const response = await apiGet(
    `/api/guardians/invitations/pending-approval`,
    token,
  );
  // @ts-expect-error - API helper returns unknown type

  return response.data;
});
