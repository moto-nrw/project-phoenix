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
  recipient_guardian_profile_ids?: string[];
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
 * is a guardian of the child (account id from the JWT, never the URL). Without
 * `?envelope=1` the response is a bare status-day array for compatibility (an
 * empty array can mean the backend created an approval request instead); with
 * it, `{status_days, pending_request}`. The flag is forwarded verbatim so old
 * and new clients keep getting the shape they asked for.
 */
export const POST = proxyPost<unknown, SickNoteBody>((params) => {
  const base = `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/sick-note`;
  return params.envelope === "1" ? `${base}?envelope=1` : base;
});
