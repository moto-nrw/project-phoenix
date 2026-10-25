import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Personen des Träger-Büros (#4650): aktive Personen mit Rollen und offene
// Einladungen. Das Backend verlangt das Recht „Konten verwalten“.
export const GET = proxyGet("/traeger/accounts/");
