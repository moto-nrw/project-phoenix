import {
  createParentGetHandler,
  createParentPostHandler,
  parentApiGet,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

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
}

/**
 * Proxy GET /api/parent/me/children/{studentId}/sick-note → backend.
 * Returns the child's active sick days (today .. +2 months by default).
 */
export const GET = createParentGetHandler<BackendStatusDay[]>(
  async (request, token, params) => {
    const studentId = String(params.studentId);
    const search = new URL(request.url).search;
    return parentApiGet<BackendStatusDay[]>(
      `/parent/me/children/${encodeURIComponent(studentId)}/sick-note${search}`,
      token,
    );
  },
);

/**
 * Proxy POST /api/parent/me/children/{studentId}/sick-note → backend
 * /parent/me/children/{studentId}/sick-note. The route-wrapper injects
 * the parent session token + 401 retry. The backend verifies the account
 * is a guardian of the child (account id from the JWT, never the URL).
 */
export const POST = createParentPostHandler<BackendStatusDay[], SickNoteBody>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    return parentApiPost<BackendStatusDay[], SickNoteBody>(
      `/parent/me/children/${encodeURIComponent(studentId)}/sick-note`,
      token,
      body,
    );
  },
);
