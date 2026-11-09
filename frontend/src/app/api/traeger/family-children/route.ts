import { proxyGet, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Kinder in der Familie (#5918): die Familien mit ihrer Kinderzahl lesen
// und eine Zahl bestätigen oder ändern. Das Backend hält jedes Lesen und
// Ändern je Vertrag fest.
export const GET = proxyGet("/traeger/family-children");

export const PUT = proxyPut("/traeger/family-children");
