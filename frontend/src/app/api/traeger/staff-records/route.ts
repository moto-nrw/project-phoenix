import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Personalakten des Trägers (#4705): die Übersicht mit Vorschlägen und das
// Verknüpfen von Beschäftigungen zu einer Akte.
export const GET = proxyGet("/traeger/staff-records/");

export const POST = proxyPost("/traeger/staff-records/");
