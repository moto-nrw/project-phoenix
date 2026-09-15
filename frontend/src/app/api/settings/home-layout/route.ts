import { proxyGet, proxyPut, proxyDelete } from "~/lib/route-proxy.server";

// Persönliche Zusammenstellung der Startseite (#2875). Die Zeile hängt am
// Konto aus dem Token, deshalb trägt der Pfad keine Kennung.
export const GET = proxyGet(() => "/api/settings/home-layout");
export const PUT = proxyPut(() => "/api/settings/home-layout");
export const DELETE = proxyDelete(() => "/api/settings/home-layout");
