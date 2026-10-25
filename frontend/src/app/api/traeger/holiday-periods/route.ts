import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Ferienzeiträume des Trägers (#4659): alle lesen und einen neuen anlegen.
// Das Backend liest und schreibt in der Träger-Transaktion der Sitzung.
export const GET = proxyGet("/traeger/holiday-periods");

export const POST = proxyPost("/traeger/holiday-periods");
