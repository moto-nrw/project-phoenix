import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Wie die Verträge jeder Schule entstehen (#4680).
export const GET = proxyGet("/traeger/contracts/conclusion-modes");
