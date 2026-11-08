import { proxyGet } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// GET /api/guardians/students/[studentId]/email-status - What the school knows
// about mail to the addresses of a child's guardians (#5934).
export const GET = proxyGet(
  (p) =>
    `/api/guardians/students/${requirePathSegmentParam(p, "studentId")}/email-status`,
);
