import { proxyPut } from "~/lib/route-proxy.server";

// Einen Schritt überspringen oder das Überspringen zurücknehmen (#2832).
export const PUT = proxyPut(
  (params) =>
    `/api/school-setup/steps/${encodeURIComponent(String(params.step))}`,
);
