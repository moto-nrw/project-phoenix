import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet("/api/enrollment/phases/expiry-warnings");
