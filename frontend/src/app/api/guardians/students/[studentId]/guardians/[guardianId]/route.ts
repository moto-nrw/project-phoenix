import { proxyDelete } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// DELETE /api/guardians/students/[studentId]/guardians/[guardianId] - Remove guardian from student
export const DELETE = proxyDelete(
  (p) =>
    `/api/guardians/students/${requirePathSegmentParam(p, "studentId")}/guardians/${requirePathSegmentParam(p, "guardianId")}`,
);
