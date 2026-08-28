import { proxyDelete, proxyGet } from "~/lib/school/route-wrapper.server";

// Benachrichtigungs-Entscheidungen der Lehrkraft im Schul-Portal (#2208).
// Der abschließende Schrägstrich ist Teil der Backend-Route.
export const GET = proxyGet("/school/notifications/preferences/");
export const DELETE = proxyDelete("/school/notifications/preferences/");
