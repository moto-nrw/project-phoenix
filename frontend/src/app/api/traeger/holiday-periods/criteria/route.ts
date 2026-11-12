import { proxyGet, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Kriterien der Platzvergabe des Trägers (#4700): lesen und als ganze Liste
// in ihrer Reihenfolge speichern.
export const GET = proxyGet("/traeger/holiday-periods/criteria");

export const PUT = proxyPut("/traeger/holiday-periods/criteria");
