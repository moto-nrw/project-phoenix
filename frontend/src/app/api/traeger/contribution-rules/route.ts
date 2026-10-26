import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Beitragsregeln des Trägers (#4664).
export const GET = proxyGet("/traeger/contribution-rules");

export const POST = proxyPost("/traeger/contribution-rules");
