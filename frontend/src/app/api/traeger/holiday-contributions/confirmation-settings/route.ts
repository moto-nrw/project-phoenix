import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Was das Bestätigen einer Ferienanmeldung beim Träger braucht (#5621): ob
// die Debitorennummer der Familie Pflicht ist.
export const GET = proxyGet(
  "/traeger/holiday-contributions/confirmation-settings",
);
