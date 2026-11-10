import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy POST → backend (#4696): the parents revoke the mandate of their
 * family; the next run no longer collects with it.
 */
export const POST = proxyPost<unknown>(
  (params) =>
    `/parent/me/documents/${requirePathSegmentParam(params, "organizationId")}/contracts/${requirePathSegmentParam(params, "contractId")}/sepa-mandate/revoke`,
);
