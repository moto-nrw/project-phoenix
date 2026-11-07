import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Länger als gebucht: die Zahlen je Schule ohne Namen (#4683).
export const GET = proxyGet("/traeger/care-time-overruns");
