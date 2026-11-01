import { proxyGet } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * GET /api/staff/{id}/qualifications
 * Die Nachweise einer Person mit Status und fehlenden Pflichtnachweisen.
 */
export const GET = proxyGet(
  (p) => `/api/staff/${requirePathSegmentParam(p)}/qualifications`,
);
