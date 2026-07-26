import { proxyPost } from "~/lib/route-proxy.server";

/**
 * POST /api/staff/shift-types/defaults
 * Seed example shift types for the current tenant (idempotent).
 */
export const POST = proxyPost("/api/shift-types/defaults");
