import { proxyGet, proxyPost } from "~/lib/operator/route-wrapper.server";

export const GET = proxyGet("/operator/organizations");

export const POST = proxyPost("/operator/organizations");
