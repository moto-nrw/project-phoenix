// app/api/settings/definitions/route.ts
import { createProxyGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/settings/definitions
 * Returns all setting definitions
 */
export const GET = createProxyGetHandler("/api/settings/definitions");
