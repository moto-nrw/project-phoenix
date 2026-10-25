import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Stammdaten des Trägers (#4653): Ansprechpartner, Logo und Bankverbindungen.
export const GET = proxyGet("/traeger/master-data/");
