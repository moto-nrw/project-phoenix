import { proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

// POST /api/students/[id]/team-notices/read - Die geteilten Notizen dieses
// Kindes sind gelesen; antwortet mit den noch ungelesenen (#6854)
export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/team-notices/read`,
);
