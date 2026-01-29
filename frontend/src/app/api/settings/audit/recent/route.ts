// Settings audit recent API route
import { createProxyGetHandler } from "~/lib/route-wrapper";

export const GET = createProxyGetHandler("/api/settings/audit/recent");
