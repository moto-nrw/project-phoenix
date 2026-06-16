import {
  createParentDeleteHandler,
  createParentGetHandler,
  createParentPostHandler,
  parentApiDelete,
  parentApiGet,
  parentApiPost,
} from "~/lib/parent/route-wrapper.server";

interface BackendCareException {
  date: string;
  pickup_time?: string;
  arrival_time?: string;
  source: string;
  updated_at: string;
}

interface CareExceptionBody {
  date: string;
  pickup_time?: string | null;
  arrival_time?: string | null;
}

/**
 * Proxy GET /api/parent/me/children/{studentId}/care-exception → backend.
 * Returns the child's pickup/arrival overrides (today .. +2 months by default).
 */
export const GET = createParentGetHandler<BackendCareException[]>(
  async (request, token, params) => {
    const studentId = String(params.studentId);
    const search = new URL(request.url).search;
    return parentApiGet<BackendCareException[]>(
      `/parent/me/children/${encodeURIComponent(studentId)}/care-exception${search}`,
      token,
    );
  },
);

/**
 * Proxy POST /api/parent/me/children/{studentId}/care-exception → backend.
 * The route-wrapper injects the parent session token + 401 retry; the backend
 * verifies the account is a guardian of the child (account id from the JWT).
 */
export const POST = createParentPostHandler<
  BackendCareException,
  CareExceptionBody
>(async (_request, body, token, params) => {
  const studentId = String(params.studentId);
  return parentApiPost<BackendCareException, CareExceptionBody>(
    `/parent/me/children/${encodeURIComponent(studentId)}/care-exception`,
    token,
    body,
  );
});

/**
 * Proxy DELETE /api/parent/me/children/{studentId}/care-exception?date=… →
 * backend. Reverts the day to the standard weekly plan.
 */
export const DELETE = createParentDeleteHandler<null>(
  async (request, token, params) => {
    const studentId = String(params.studentId);
    const search = new URL(request.url).search;
    return parentApiDelete<null>(
      `/parent/me/children/${encodeURIComponent(studentId)}/care-exception${search}`,
      token,
    );
  },
);
