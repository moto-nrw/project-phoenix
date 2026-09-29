import { proxyGet } from "~/lib/route-proxy.server";

// Einrichtungs-Assistent für neue Schulen (#2832). Der Zustand gilt für die
// Schule, das Ausblenden für das Konto aus dem Token.
export const GET = proxyGet("/api/school-setup");
