// app/api/settings/history/route.ts
import { createProxyGetHandler } from "~/lib/route-wrapper";

/**
 * Handler for GET /api/settings/history
 * Returns setting change history with optional filters
 */
export const GET = createProxyGetHandler("/api/settings/history");
