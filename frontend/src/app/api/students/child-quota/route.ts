import { proxyGet } from "~/lib/route-wrapper.server";

// Kinderkontingent der Schule (#3569); das Backend verlangt users:delete.
export const GET = proxyGet("/api/students/child-quota");
