import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Das eigene Konto im Träger-Büro (#4693): Anmelde-Adresse und Name. Jede
// angemeldete Person darf es lesen.
export const GET = proxyGet("/traeger/profile");
