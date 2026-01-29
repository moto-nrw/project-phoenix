// Settings tabs API route
import { createProxyGetHandler } from "~/lib/route-wrapper";

export const GET = createProxyGetHandler("/api/settings/tabs");
