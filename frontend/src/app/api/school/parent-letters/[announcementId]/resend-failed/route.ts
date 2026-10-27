import { proxyPost } from "~/lib/school/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// POST /api/school/parent-letters/[announcementId]/resend-failed (#4800).
export const POST = proxyPost(
  (p) =>
    `/school/parent-letters/${requirePathSegmentParam(p, "announcementId")}/resend-failed`,
);
