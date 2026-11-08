import { proxyGet } from "~/lib/route-proxy.server";

// Mensa-Monatsfreigabe (#4686): die letzten zwölf Monate mit ihrem Stand.
// meals:manage und meals:orders erzwingt das Backend.
export const GET = proxyGet("/api/meal-billing-months");
