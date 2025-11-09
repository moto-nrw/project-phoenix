import { NextRequest } from "next/server";
import { createPutHandler } from "@/lib/route-wrapper";
import { apiPut } from "@/lib/api-helpers";

// PUT /api/guardians/relationships/[relationshipId] - Update student-guardian relationship
export const PUT = createPutHandler(async (request, body, token, params) => {
  const { relationshipId } = params;

  const response = await apiPut(
    `/api/guardians/relationships/${relationshipId}`,
    token,
    body
  );
  return response.data;
});
