import { proxyGet } from "~/lib/school/route-wrapper.server";

// VAPID-Schlüssel für das Push-Abo eines Schul-Geräts (#2208).
export const GET = proxyGet("/school/notifications/push/public-key");
