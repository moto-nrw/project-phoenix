import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/guardians/students/[studentId]/invite
// Invite a guardian to a student by email (staff). The backend resolves an
// existing profile/account or creates a fresh invite.
export const POST = proxyPost(
  (p) =>
    `/api/guardians/students/${requirePathSegmentParam(p, "studentId")}/invite`,
);
