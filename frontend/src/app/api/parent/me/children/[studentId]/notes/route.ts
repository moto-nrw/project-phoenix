import {
  createParentGetHandler,
  createParentPostHandler,
  parentApiGet,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

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
export const GET = createParentGetHandler<BackendParentNote[]>(
  async (_request, token, params) => {
    const studentId = String(params.studentId);
    return parentApiGet<BackendParentNote[]>(
      `/parent/me/children/${encodeURIComponent(studentId)}/notes`,
      token,
    );
  },
);

/**
 * Proxy POST /api/parent/me/children/{studentId}/notes → backend. Appends
 * a note and returns the newest few. Guardian check + feature gate happen
 * server-side.
 */
export const POST = createParentPostHandler<BackendParentNote[], AddNoteBody>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    return parentApiPost<BackendParentNote[], AddNoteBody>(
      `/parent/me/children/${encodeURIComponent(studentId)}/notes`,
      token,
      body,
    );
  },
);
