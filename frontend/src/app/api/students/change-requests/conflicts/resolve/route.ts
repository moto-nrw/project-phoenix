import { proxyPost } from "~/lib/route-proxy.server";

/**
 * Proxy POST /api/students/change-requests/conflicts/resolve → backend
 * (#2267). Legt für eine Gruppe sich widersprechender Anfragen EIN Ergebnis
 * fest: eine freigeben, einen eigenen Wert eintragen oder alles ablehnen.
 * Entweder klappt alles zusammen oder nichts.
 */
export const POST = proxyPost(
  "/api/students/change-requests/conflicts/resolve",
);
