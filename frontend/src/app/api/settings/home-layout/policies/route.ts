import { proxyPut } from "~/lib/route-wrapper.server";

// Vorgabe der Einrichtung für alle (#2875). Die Berechtigung prüft das
// Backend; hier wird nur weitergereicht.
export const PUT = proxyPut(() => "/api/settings/home-layout/policies");
