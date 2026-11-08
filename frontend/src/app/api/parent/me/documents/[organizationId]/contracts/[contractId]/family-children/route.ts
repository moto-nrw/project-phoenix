import { proxyGet, proxyPut } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

const path = (params: Record<string, unknown>) =>
  `/parent/me/documents/${requirePathSegmentParam(params, "organizationId")}/contracts/${requirePathSegmentParam(params, "contractId")}/family-children`;

/**
 * Proxy GET → backend: how many children the family of a care contract in
 * the inbox has for the sibling rule (#5918). The account comes from the
 * parent JWT server-side; the backend checks the address and the
 * relationship right.
 */
export const GET = proxyGet<unknown>(path);

/** Proxy PUT → backend: the parents state the number of their children. */
export const PUT = proxyPut<unknown>(path);
