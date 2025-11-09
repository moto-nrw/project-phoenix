import { NextRequest } from "next/server";
import { createPutHandler } from "@/lib/route-wrapper";
import { apiPut } from "@/lib/api-helpers";

// PUT /api/guardians/relationships/[relationshipId] - Update student-guardian relationship
export const PUT = createPutHandler(async (request, token, params) => {
  const { relationshipId } = await params;
  const body = await request.json();

  const response = await apiPut(
    `/api/guardians/relationships/${relationshipId}`,
    body,
    token
  );
  return response.data;
});
