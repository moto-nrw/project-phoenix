import { proxyGet } from "~/lib/operator/route-wrapper.server";

export const GET = proxyGet("/operator/suggestions/unviewed-count");
