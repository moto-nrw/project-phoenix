import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Träger der Sitzung (#4658): Name und Kennzeichen „Test-Träger“ für den
// Rahmen des Träger-Portals.
export const GET = proxyGet("/traeger/organization");
