import { proxyGet, proxyPost } from "~/lib/operator/route-wrapper.server";

export const GET = proxyGet("/operator/devices");

export const POST = proxyPost("/operator/devices");
