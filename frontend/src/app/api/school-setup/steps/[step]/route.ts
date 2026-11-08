import { proxyPut } from "~/lib/route-wrapper.server";

// Einen Schritt überspringen oder das Überspringen zurücknehmen (#2832).
export const PUT = proxyPut(
  (params) =>
    `/api/school-setup/steps/${encodeURIComponent(String(params.step))}`,
);
