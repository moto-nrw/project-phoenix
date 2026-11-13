import { proxyPost } from "~/lib/route-wrapper.server";

// Check children in and place them in a running session (#6953). The
// backend checks both rights and writes attendance and room stay together.
export const POST = proxyPost("/api/active/visits/check-in-to-group");
