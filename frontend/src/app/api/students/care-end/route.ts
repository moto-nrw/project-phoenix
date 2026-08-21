import { proxyPost } from "@/lib/route-proxy.server";

/**
 * POST /api/students/care-end
 *
 * Confirms "Betreuung beenden" for exactly the state a preview described
 * (#2487). Pure pass-through: the backend owns the users:delete gate, the
 * preview-token comparison and the atomic write.
 */
export const POST = proxyPost("/api/students/care-end");
