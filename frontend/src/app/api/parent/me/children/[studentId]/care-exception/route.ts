import {
  createParentDeleteHandler,
  parentApiDelete,
  proxyGet,
  proxyPost,
} from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendCareException {
  date: string;
  pickup_time?: string;
  arrival_time?: string;
  source: string;
  updated_at: string;
}

interface BackendPickupChangeRequest {
  id: string;
  date: string;
  pickup_time: string;
  reason: string;
  status: string;
  decision_reason?: string;
  created_at: string;
  reviewed_at?: string;
}

interface CareExceptionBody {
  date: string;
  pickup_time?: string | null;
  arrival_time?: string | null;
  reason?: string;
  /** Guardians the request is shared with, saved with the request itself. */
  recipient_guardian_profile_ids?: string[];
}

/**
 * Proxy GET /api/parent/me/children/{studentId}/care-exception → backend.
 * Returns the child's pickup/arrival overrides (today .. +2 months by default).
 */
export const GET = proxyGet<BackendCareException[]>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/care-exception`,
);

/**
 * Proxy POST /api/parent/me/children/{studentId}/care-exception → backend.
 * The route-wrapper injects the parent session token + 401 retry; the backend
 * verifies the account is a guardian of the child (account id from the JWT).
 */
export const POST = proxyPost<BackendPickupChangeRequest, CareExceptionBody>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/care-exception`,
);

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
