import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy GET /api/parent/me/documents/{organizationId}/contracts/{contractId}/signing
 * → backend (#4679): a care contract of the inbox to read and sign. The
 * account comes from the parent JWT server-side.
 */
export const GET = proxyGet<unknown>(
  (params) =>
    `/parent/me/documents/${requirePathSegmentParam(params, "organizationId")}/contracts/${requirePathSegmentParam(params, "contractId")}/signing`,
);
