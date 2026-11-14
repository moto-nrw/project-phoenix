import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Bank- und Steuerdaten einer Akte im Klartext zeigen; der Zugriff steht im
// Protokoll jedes Einsatzorts (#4705).
export const POST = proxyPost(
  (params) =>
    `/traeger/staff-records/${requirePathSegmentParam(params)}/financial/reveal`,
);
