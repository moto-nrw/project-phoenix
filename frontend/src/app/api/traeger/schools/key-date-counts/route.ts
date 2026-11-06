import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Die festgehaltenen Stichtagszahlen aller Schulen des Trägers (#4673).
export const GET = proxyGet("/traeger/schools/key-date-counts");
