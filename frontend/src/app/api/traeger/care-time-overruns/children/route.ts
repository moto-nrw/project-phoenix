import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Länger als gebucht: die Kinder mit Namen, nur mit „Verträge verwalten“ (#4683).
export const GET = proxyGet("/traeger/care-time-overruns/children");
