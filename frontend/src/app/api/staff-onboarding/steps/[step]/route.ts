import { proxyPut } from "~/lib/route-proxy.server";

// Einen Schritt als erledigt, übersprungen oder wieder offen markieren (#3748).
export const PUT = proxyPut(
  (params) =>
    `/api/staff-onboarding/steps/${encodeURIComponent(String(params.step))}`,
);
