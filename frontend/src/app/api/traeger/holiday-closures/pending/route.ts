import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Schließtage in gebuchten Ferienwochen, über die noch niemand die Eltern
// informiert hat (#6434).
export const GET = proxyGet("/traeger/holiday-closures/pending");
