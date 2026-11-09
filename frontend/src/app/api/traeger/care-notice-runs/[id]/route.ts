import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Ein Lauf der Jahresbescheide (#6219): Vorschau oder ausgestellte Bescheide.
export const GET = proxyGet(
  (params) => `/traeger/care-notice-runs/${requirePathSegmentParam(params)}`,
);
