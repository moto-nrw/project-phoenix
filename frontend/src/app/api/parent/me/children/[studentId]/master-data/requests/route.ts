import { proxyGet, proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface ChangeRequestBody {
  changes: { target: string; field_key: string; value: unknown }[];
  /** Guardians the request is shared with, saved with the request itself. */
  recipient_guardian_profile_ids?: string[];
}

/**
 * Proxy GET /api/parent/me/children/{studentId}/master-data/requests → backend.
 * Lists the child's Track B change requests (any status).
 */
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/master-data/requests`,
);

/**
 * Proxy POST /api/parent/me/children/{studentId}/master-data/requests → backend.
 * Submits Track B change requests for staff approval.
 */
export const POST = proxyPost<unknown, ChangeRequestBody>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/master-data/requests`,
);
