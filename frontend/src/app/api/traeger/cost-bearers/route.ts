import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Kostenträger des Trägers (#5627).
export const GET = proxyGet("/traeger/cost-bearers");

export const POST = proxyPost("/traeger/cost-bearers");
