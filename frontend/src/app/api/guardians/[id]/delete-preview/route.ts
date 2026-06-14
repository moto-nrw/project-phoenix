import { createGetHandler } from "@/lib/route-wrapper.server";
import { apiGet } from "@/lib/api-helpers.server";

// GET /api/guardians/[id]/delete-preview - Read-only blast radius of a full
// delete (admin-only on the backend). Lets the confirmation modal show the
// affected children without the old destructive "probe DELETE" (#819).
export const GET = createGetHandler(async (request, token, params) => {
  const { id } = params;
  const guardianId = String(id);

  const response = await apiGet(
    `/api/guardians/${guardianId}/delete-preview`,
    token,
  );
  // @ts-expect-error - API helper returns unknown type

  return response.data;
});
