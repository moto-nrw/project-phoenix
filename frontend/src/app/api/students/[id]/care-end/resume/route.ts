import { proxyPost } from "@/lib/route-proxy.server";
import { requirePathSegmentParam } from "@/lib/route-wrapper-utils.server";

/**
 * POST /api/students/[id]/care-end/resume
 *
 * Reopens one child's care from a new start day (#2487). Nothing is switched
 * back on automatically — the backend requires the caller to confirm that
 * group, offerings, weekly plan and arrival/pickup times were reviewed.
 */
export const POST = proxyPost(
  (p) => `/api/students/${requirePathSegmentParam(p)}/care-end/resume`,
);
