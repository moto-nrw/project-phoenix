import { proxyGet } from "~/lib/route-proxy.server";

// Erste Schritte der Betreuungskräfte (#3748). Der Stand gilt für das Konto
// aus dem Token.
export const GET = proxyGet("/api/staff-onboarding");
