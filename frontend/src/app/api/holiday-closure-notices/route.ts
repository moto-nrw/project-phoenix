import { proxyGet } from "~/lib/route-wrapper.server";

// Die Elternhinweise zu Schließtagen der Schule in gebuchten Ferienwochen
// (ADR 0196): Vorschlag, Versandstand und die Zahl der Familien.
export const GET = proxyGet("/api/holiday-closure-notices");
