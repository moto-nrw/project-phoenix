import { proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Section writes (person, kontakt, arbeitsvertrag, qualifikationen). The
// static sibling folder `bank-steuer/` wins over this dynamic segment, so the
// financial section never routes through here.
export const PUT = proxyPut(
  (p) =>
    `/api/staff/${requirePathSegmentParam(p)}/stammdaten/${requirePathSegmentParam(p, "section")}`,
);
