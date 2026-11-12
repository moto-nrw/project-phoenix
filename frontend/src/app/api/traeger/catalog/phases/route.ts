import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Anmeldephasen des Trägers (#4704, ADR 0195): alle lesen und eine neue
// anlegen. Das Backend schreibt in der Träger-Transaktion der Sitzung.
export const GET = proxyGet("/traeger/catalog/phases");

export const POST = proxyPost("/traeger/catalog/phases");
