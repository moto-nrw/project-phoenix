import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Ein Klick des Träger-Büros beauftragt einen Elternhinweis (ADR 0196).
export const POST = proxyPost(
  (p) =>
    `/traeger/holiday-closures/pending/${requirePathSegmentParam(p)}/notify`,
);
