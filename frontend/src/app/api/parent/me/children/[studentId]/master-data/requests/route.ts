import {
  createParentGetHandler,
  createParentPostHandler,
  parentApiGet,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

interface ChangeRequestBody {
  changes: { target: string; field_key: string; value: unknown }[];
}

/**
 * Proxy GET /api/parent/me/children/{studentId}/master-data/requests → backend.
 * Lists the child's Track B change requests (any status).
 */
export const GET = createParentGetHandler<unknown>(
  async (_request, token, params) => {
    const studentId = String(params.studentId);
    return parentApiGet<unknown>(
      `/parent/me/children/${encodeURIComponent(studentId)}/master-data/requests`,
      token,
    );
  },
);

/**
 * Proxy POST /api/parent/me/children/{studentId}/master-data/requests → backend.
 * Submits Track B change requests for staff approval.
 */
export const POST = createParentPostHandler<unknown, ChangeRequestBody>(
  async (_request, body, token, params) => {
    const studentId = String(params.studentId);
    return parentApiPost<unknown, ChangeRequestBody>(
      `/parent/me/children/${encodeURIComponent(studentId)}/master-data/requests`,
      token,
      body,
    );
  },
);
