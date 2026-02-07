// Settings definitions API route
import { createProxyGetHandler } from "~/lib/route-wrapper";

export const GET = createProxyGetHandler("/api/settings/definitions");
