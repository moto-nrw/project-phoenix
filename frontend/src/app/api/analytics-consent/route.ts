import { proxyGet, proxyPut } from "~/lib/route-proxy.server";

// Einwilligung des eigenen Kontos zur Nutzungsanalyse (#4901). Die Antwort
// hängt am Konto aus dem Token, deshalb trägt der Pfad keine Kennung. Der
// abschließende Schrägstrich ist Teil der Backend-Route.
export const GET = proxyGet<{ granted: boolean | null }>("/analytics-consent/");
export const PUT = proxyPut<{ granted: boolean | null }>("/analytics-consent/");
