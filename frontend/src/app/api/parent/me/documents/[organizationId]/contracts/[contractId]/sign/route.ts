import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy POST /api/parent/me/documents/{organizationId}/contracts/{contractId}/sign
 * → backend (#4679): the parents sign a care contract of their inbox. The
 * body carries the signature; the account comes from the parent JWT.
 */
export const POST = proxyPost<unknown>(
  (params) =>
    `/parent/me/documents/${requirePathSegmentParam(params, "organizationId")}/contracts/${requirePathSegmentParam(params, "contractId")}/sign`,
);
