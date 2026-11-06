import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Vertragsvorlagen je Angebot der Schulen des Trägers (#4678).
export const GET = proxyGet("/traeger/contract-templates");
