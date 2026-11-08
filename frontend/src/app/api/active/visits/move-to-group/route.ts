import { proxyPost } from "~/lib/route-wrapper.server";

export const POST = proxyPost("/api/active/visits/move-to-group");
