import { proxyGet } from "~/lib/route-proxy.server";

// Bestellungen der Schule (#3873): Zahlen je Tag und Klasse, Nachmeldungen.
// meals:read erzwingt das Backend.
export const GET = proxyGet("/api/meal-plan/orders");
