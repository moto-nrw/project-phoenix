import { proxyGet } from "~/lib/school/route-wrapper.server";

// Fehlzeitenliste einer Klasse im Schul-Portal (#4793): leitet class/from/to
// mit dem school-Token weiter; welche Klasse die Person sehen darf, prüft das
// Backend.
export const GET = proxyGet("/school/class-day/absence-list");
