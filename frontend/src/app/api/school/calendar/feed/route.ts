import { proxyGet } from "~/lib/school/route-wrapper.server";

// Abo-Link des Kalenders in moto schule (#4798).
export const GET = proxyGet("/school/calendar/feed");
