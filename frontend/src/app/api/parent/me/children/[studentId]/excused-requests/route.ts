import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendExcusedRequest {
  id: string;
  student_id: string;
  status: string;
  dates: string[];
  note: string;
  decision_reason?: string;
  created_at: string;
  reviewed_at?: string;
  is_self: boolean;
}

/**
 * Proxy GET /api/parent/me/children/{studentId}/excused-requests → backend.
 * Returns the child's "entschuldigt" absence requests that went through the OGS
 * approval gate (pending + recently-decided), newest first. The route-wrapper
 * injects the parent session token; the backend verifies the account is a
 * guardian of the child (account id from the JWT, never the URL).
 */
export const GET = proxyGet<BackendExcusedRequest[]>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/excused-requests`,
);
