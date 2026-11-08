import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Regeln des Monatsbeitrags (#4685): Einkommensstufen, Geschwister,
// Alleinerziehende und Höchstbetrag.
export const GET = proxyGet("/traeger/care-rules");

export const POST = proxyPost("/traeger/care-rules");
