import { proxyPost } from "~/lib/route-proxy.server";

export const POST = proxyPost("/api/active/visits/move-to-group");
