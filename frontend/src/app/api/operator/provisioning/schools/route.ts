import { proxyGet, proxyPost } from "~/lib/operator/route-wrapper.server";

export const GET = proxyGet("/operator/schools");

export const POST = proxyPost("/operator/schools");
