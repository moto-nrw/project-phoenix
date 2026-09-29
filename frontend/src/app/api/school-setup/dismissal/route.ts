import { proxyPut } from "~/lib/route-proxy.server";

// Blendet den Assistenten für das eigene Konto aus oder wieder ein (#2832).
export const PUT = proxyPut("/api/school-setup/dismissal");
