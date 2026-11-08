import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Jedes Schreiben eines Monatslaufs mit Adresse und ohne Mail senden (#4684).
export const POST = proxyPost(
  (params) =>
    `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/letters/mail`,
);
