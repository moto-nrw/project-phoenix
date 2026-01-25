// app/api/settings/user/me/route.ts
import { createProxyGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/settings/user/me
 * Returns the current user's settings with resolved values
 */
export const GET = createProxyGetHandler("/api/settings/user/me");
