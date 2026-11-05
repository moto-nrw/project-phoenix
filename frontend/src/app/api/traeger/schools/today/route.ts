import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Zahlen von jetzt für alle Schulen des Trägers und ihre Summe (#4674).
export const GET = proxyGet("/traeger/schools/today");
