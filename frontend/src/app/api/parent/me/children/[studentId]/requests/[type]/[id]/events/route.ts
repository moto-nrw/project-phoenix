import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/parent/me/children/{studentId}/requests/{type}/{id}/events →
 * backend. Returns the append-only history of one request; its newest entry
 * carries the version a guardian edit has to send back.
 */
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/requests/${requirePathSegmentParam(params, "type")}/${requirePathSegmentParam(params, "id")}/events`,
);
