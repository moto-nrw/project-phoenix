import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Jeden gespeicherten Jahresbescheid mit Adresse und ohne Mail senden (#6347).
export const POST = proxyPost(
  (params) =>
    `/traeger/care-notice-runs/${requirePathSegmentParam(params)}/letters/mail`,
);
