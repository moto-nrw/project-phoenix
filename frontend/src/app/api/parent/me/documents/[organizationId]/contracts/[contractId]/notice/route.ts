import { proxyGet, proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

const path = (params: Record<string, unknown>) =>
  `/parent/me/documents/${requirePathSegmentParam(params, "organizationId")}/contracts/${requirePathSegmentParam(params, "contractId")}/notice`;

/**
 * Proxy GET → backend: what the parents can declare on a care contract sent
 * to them today, or the declaration made (#4682). The account comes from
 * the parent JWT server-side; the backend checks the relationship right.
 * The wire shape lives in `~/lib/care-contract-notices`.
 */
export const GET = proxyGet<unknown>(path);

/**
 * Proxy POST → backend: the parents withdraw or terminate the contract with
 * the button „jetzt kündigen“ (§ 312k BGB) and get the confirmation.
 */
export const POST = proxyPost<unknown>(path);
