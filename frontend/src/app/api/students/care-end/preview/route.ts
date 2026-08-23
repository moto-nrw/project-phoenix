import { proxyPost } from "@/lib/route-proxy.server";

/**
 * POST /api/students/care-end/preview
 *
 * Asks what ending the care of the selected children would do, without
 * changing anything (#2487).
 */
export const POST = proxyPost("/api/students/care-end/preview");
