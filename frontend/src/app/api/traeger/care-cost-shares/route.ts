import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Kostenträger-Anteile der Verträge am Monatsbeitrag (#4687). Das Backend
// hält jedes Lesen je Vertrag fest.
export const GET = proxyGet("/traeger/care-cost-shares");

export const POST = proxyPost("/traeger/care-cost-shares");
