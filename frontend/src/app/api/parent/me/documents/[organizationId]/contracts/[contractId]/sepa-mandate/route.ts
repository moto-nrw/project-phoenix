import { proxyGet, proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

const path = (params: Record<string, unknown>) =>
  `/parent/me/documents/${requirePathSegmentParam(params, "organizationId")}/contracts/${requirePathSegmentParam(params, "contractId")}/sepa-mandate`;

/**
 * Proxy GET → backend: the SEPA mandate of the family on a care contract of
 * the inbox (#4696). The account comes from the parent JWT server-side; the
 * backend checks the address, the relationship right and the Träger's switch.
 */
export const GET = proxyGet<unknown>(path);

/** Proxy POST → backend: the parents give the mandate they confirmed. */
export const POST = proxyPost<unknown>(path);
