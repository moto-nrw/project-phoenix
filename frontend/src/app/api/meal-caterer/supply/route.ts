import { proxyDelete } from "~/lib/route-proxy.server";

// Die Belieferung beenden: der Caterer sieht die Schule danach nicht mehr
// (ADR 0075). meals:manage erzwingt das Backend.
export const DELETE = proxyDelete("/api/meal-caterer/supply");
