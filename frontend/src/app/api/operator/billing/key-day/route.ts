import { proxyGet, proxyPut } from "~/lib/operator/route-wrapper.server";

export const GET = proxyGet("/operator/billing/key-day");

export const PUT = proxyPut("/operator/billing/key-day");
