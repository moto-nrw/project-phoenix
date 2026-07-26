import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/me/groups");
