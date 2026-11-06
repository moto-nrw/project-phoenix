import { proxyGet } from "~/lib/route-proxy.server";

// The school's consent defaults for a child that does not exist yet.
export const GET = proxyGet("/api/students/privacy-consent/defaults");
