import { proxyGet } from "~/lib/route-wrapper.server";

// GET /api/guardians/payment-overview - the Bankverbindungen list, masked
export const GET = proxyGet(() => `/api/guardians/payment-overview`);
