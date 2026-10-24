import { proxyGet } from "~/lib/school/route-wrapper.server";

// Kalender der Lehrkraft in moto schule (#4798): Termine, Schließtage,
// Ferien und die eigenen Klassen für den from/to-Zeitraum.
export const GET = proxyGet("/school/calendar/my");
