import { NextRequest } from "next/server";
import { createDeleteHandler } from "@/lib/route-wrapper";
import { apiDelete } from "@/lib/api-helpers";

// DELETE /api/guardians/students/[studentId]/guardians/[guardianId] - Remove guardian from student
export const DELETE = createDeleteHandler(async (request, token, params) => {
  const { studentId, guardianId } = await params;

  await apiDelete(
    `/api/guardians/students/${studentId}/guardians/${guardianId}`,
    token
  );
  return null;
});
