import { proxyGet } from "~/lib/route-proxy.server";

// Der Caterer der Schule (ADR 0075): die laufende Belieferung und die
// Einladungen. meals:manage erzwingt das Backend.
export const GET = proxyGet("/api/meal-caterer");
