import { proxyPost } from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Ein Klick der Schule beauftragt den Elternhinweis (ADR 0196).
export const POST = proxyPost(
  (p) => `/api/holiday-closure-notices/${requirePathSegmentParam(p)}/notify`,
);
