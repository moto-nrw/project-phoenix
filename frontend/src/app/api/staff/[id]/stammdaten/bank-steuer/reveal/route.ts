import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// POST on purpose: revealing the unmasked values writes an audit row, so the
// read has a side effect.
export const POST = proxyPost(
  (p) =>
    `/api/staff/${requirePathSegmentParam(p)}/stammdaten/bank-steuer/reveal`,
);
