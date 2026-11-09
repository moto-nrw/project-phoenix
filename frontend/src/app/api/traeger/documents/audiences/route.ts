import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Die Schulen des Trägers mit ihren Gruppen, für eine Freigabe (#4692).
export const GET = proxyGet("/traeger/documents/audiences");
