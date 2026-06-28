import { proxyGet, proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendParentNote {
  id: string;
  student_id: string;
  body: string;
  created_at: string;
}

interface AddNoteBody {
  body: string;
}

/**
 * Proxy GET /api/parent/me/children/{studentId}/notes → backend. Returns
 * the newest notes the parent left for the team (newest first).
 */
export const GET = proxyGet<BackendParentNote[]>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/notes`,
);

/**
 * Proxy POST /api/parent/me/children/{studentId}/notes → backend. Appends
 * a note and returns the newest few. Guardian check + feature gate happen
 * server-side.
 */
export const POST = proxyPost<BackendParentNote[], AddNoteBody>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/notes`,
);
