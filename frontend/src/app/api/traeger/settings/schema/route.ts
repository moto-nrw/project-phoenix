import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Einstellungen des Trägers (#4652, ADR 0086): nur Einstellungen der Ebenen
// `beide` und `traeger`, je mit Träger-Wert oder Standard.
export const GET = proxyGet("/traeger/settings/schema");
