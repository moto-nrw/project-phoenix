import { proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

/**
 * Proxy POST → backend (#4696): the mandate text the parents read and
 * confirm next. It writes nothing; the body carries the account.
 */
export const POST = proxyPost<unknown>(
  (params) =>
    `/parent/me/documents/${requirePathSegmentParam(params, "organizationId")}/contracts/${requirePathSegmentParam(params, "contractId")}/sepa-mandate/preview`,
);
