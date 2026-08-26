import { proxyGet } from "@/lib/route-proxy.server";

// GET /api/guardians/payment-overview - the Bankverbindungen list, masked
export const GET = proxyGet(() => `/api/guardians/payment-overview`);
