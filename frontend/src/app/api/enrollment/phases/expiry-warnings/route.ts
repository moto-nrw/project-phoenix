import { proxyGet } from "~/lib/route-wrapper.server";

export const GET = proxyGet("/api/enrollment/phases/expiry-warnings");
