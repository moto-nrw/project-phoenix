import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Nachweise der Verträge für Einkommensstufe und Alleinerziehende (#4685).
// Das Backend hält jedes Lesen je Vertrag fest.
export const GET = proxyGet("/traeger/care-evidences");

export const POST = proxyPost("/traeger/care-evidences");
