import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Widerruf und Kündigung eines Betreuungsvertrags (#4682): was der Vertrag
// heute erlaubt, und eine Erklärung, die das Träger-Büro erhalten hat.
export const GET = proxyGet(
  (params) => `/traeger/contract-notices/${requirePathSegmentParam(params)}`,
);

export const POST = proxyPost(
  (params) => `/traeger/contract-notices/${requirePathSegmentParam(params)}`,
);
