import { proxyGet, proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendStatusDay {
  id: string;
  student_id: string;
  date: string;
  status: string;
  reported_at: string;
  source: string;
  note?: string;
}

interface SickNoteBody {
  dates: string[];
  reason?: string;
  status?: string;
}

interface BackendExcusedRequest {
  id: string;
  student_id: string;
  status: string;
  dates: string[];
  note: string;
  decision_reason?: string;
  created_at: string;
  reviewed_at?: string;
}

// The POST response is the envelope `{ status_days, pending_request? }` — an
// excused submission behind the approval gate returns an empty status_days and
// a populated pending_request instead of recording the days immediately.
interface SickNoteSubmitResponse {
  status_days: BackendStatusDay[];
  pending_request?: BackendExcusedRequest;
}

/**
 * Proxy GET /api/parent/me/children/{studentId}/sick-note → backend.
 * Returns the child's active sick days (today .. +2 months by default).
 */
export const GET = proxyGet<BackendStatusDay[]>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/sick-note`,
);

/**
 * Proxy POST /api/parent/me/children/{studentId}/sick-note → backend
 * /parent/me/children/{studentId}/sick-note. The route-wrapper injects
 * the parent session token + 401 retry. The backend verifies the account
 * is a guardian of the child (account id from the JWT, never the URL).
 */
export const POST = proxyPost<SickNoteSubmitResponse, SickNoteBody>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/sick-note`,
);
