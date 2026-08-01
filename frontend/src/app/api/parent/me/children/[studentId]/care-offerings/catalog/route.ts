import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/parent/me/children/{studentId}/care-offerings/catalog →
 * backend. Returns the offerings the guardian may pick from, prefilled with the
 * current booking, plus the allowed effective-date window.
 */
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/care-offerings/catalog`,
);
