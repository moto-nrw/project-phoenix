// app/api/settings/system/route.ts
import { createProxyGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/settings/system
 * Returns system-wide settings with resolved values (admin only)
 */
export const GET = createProxyGetHandler("/api/settings/system");
