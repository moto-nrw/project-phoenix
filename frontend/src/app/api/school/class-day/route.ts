import { proxyGet } from "~/lib/school/route-wrapper.server";

// Klassenansicht im Schul-Portal (#2207): leitet class/date-Query mit dem
// school-Token an das Backend weiter; der Klassen-Scope wird serverseitig
// erzwungen.
export const GET = proxyGet("/school/class-day/");
