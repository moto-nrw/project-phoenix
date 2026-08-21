import { proxyPost } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Klassenlisteneintrag bewusst einem regulären Kind zuordnen (#2382): löscht
// die Dublette aus der Klassenliste. users:delete — erzwingt das Backend.
export const POST = proxyPost(
  (p) => `/api/class-list-entries/${requirePathSegmentParam(p)}/assign`,
);
