import { proxyGet } from "~/lib/school/route-wrapper.server";

// Zugewiesene Klassen der angemeldeten Lehrkraft im Schul-Portal (#2207).
export const GET = proxyGet("/school/class-day/classes");
