import { proxyPost } from "@/lib/route-proxy.server";

/**
 * POST /api/students/care-end/cancel
 *
 * Withdraws an end of care that has not taken effect yet (#2487).
 */
export const POST = proxyPost("/api/students/care-end/cancel");
