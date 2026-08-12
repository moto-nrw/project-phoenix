import { proxyGet } from "~/lib/route-proxy.server";

// Zugewiesene Klassen der angemeldeten Lehrkraft (#1772).
export const GET = proxyGet("/api/class-day/classes");
