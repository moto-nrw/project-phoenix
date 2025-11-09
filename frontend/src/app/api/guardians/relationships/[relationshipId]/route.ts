import { createPutHandler } from "@/lib/route-wrapper";
import { apiPut } from "@/lib/api-helpers";

// PUT /api/guardians/relationships/[relationshipId] - Update student-guardian relationship
export const PUT = createPutHandler(async (request, body, token, params) => {
  const { relationshipId } = params;
  const relId = String(relationshipId);

  const response = await apiPut(
    `/api/guardians/relationships/${relId}`,
    token,
    body,
  );
  // @ts-expect-error - API helper returns unknown type
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  return response.data;
});
