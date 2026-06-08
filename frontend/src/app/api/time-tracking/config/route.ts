import { createProxyGetDataHandler } from "~/lib/route-wrapper.server";

export const GET = createProxyGetDataHandler("/api/time-tracking/config");
