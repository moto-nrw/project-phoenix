import { proxyGet, proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/guardians/students/[studentId]/guardians - Get all guardians for a student
export const GET = proxyGet(
  (p) =>
    `/api/guardians/students/${requirePathSegmentParam(p, "studentId")}/guardians`,
);

// POST /api/guardians/students/[studentId]/guardians - Link guardian to student
export const POST = proxyPost(
  (p) =>
    `/api/guardians/students/${requirePathSegmentParam(p, "studentId")}/guardians`,
);
