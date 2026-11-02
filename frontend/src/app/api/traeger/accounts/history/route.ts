import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Verlauf des Träger-Büros (#5474): eine Seite der Änderungen an Personen
// und Einladungen. Das Backend verlangt das Recht „Konten verwalten“.
export const GET = proxyGet("/traeger/accounts/history");
