import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/guardians/students/[studentId]/guardians/batch
// Atomically creates (or links) one or more guardians for an existing student
// in a single backend transaction (#819).
export const POST = proxyPost(
  (p) =>
    `/api/guardians/students/${requirePathSegmentParam(p, "studentId")}/guardians/batch`,
);
