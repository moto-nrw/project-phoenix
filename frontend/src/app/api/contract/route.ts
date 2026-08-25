import { proxyGet } from "~/lib/route-proxy.server";

/**
 * GET /api/contract — Vertragsdaten und Zahlungsplan der Schule (#1459).
 * Nur Lesen: geschrieben wird ausschließlich im Operator-Portal.
 */
export const GET = proxyGet(() => "/api/contract");
