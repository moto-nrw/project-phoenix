import { proxyGet } from "@/lib/route-proxy.server";

/**
 * GET /api/students/ended-care
 *
 * The "Beendete Betreuungen" archive (#2487): every child whose care interval
 * has run out, with the recorded exit reason where there is one. Behind
 * users:delete in the backend, like the reason itself.
 */
export const GET = proxyGet("/api/students/ended-care");
