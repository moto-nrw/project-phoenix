import { proxyGet } from "~/lib/route-proxy.server";

// Lehrkraft-Tagesansicht (#1772): leitet class/date-Query an das Backend
// weiter; der Klassen-Scope wird serverseitig erzwungen.
export const GET = proxyGet("/api/class-day/");
