import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Grund nicht anerkannt (#4682): das Träger-Büro erkennt den Grund einer
// außerordentlichen Kündigung nicht an; sie gilt als ordentliche Kündigung.
export const POST = proxyPost(
  (params) =>
    `/traeger/contract-notices/${requirePathSegmentParam(params)}/reason-rejection`,
);
