import { proxyGet } from "~/lib/route-proxy.server";

// Kinderkontingent der Schule (#3569); das Backend verlangt users:delete.
export const GET = proxyGet("/api/students/child-quota");
