import {
  proxyDelete,
  proxyGet,
  proxyPut,
} from "~/lib/school/route-wrapper.server";

// Einen eigenen Elternsprechtag mit Belegung lesen, ändern oder löschen
// (#4801). Ob er der Lehrkraft gehört, prüft das Backend.
const endpoint = (params: Record<string, unknown>) =>
  `/school/calendar/parent-conferences/${encodeURIComponent(params.conferenceId as string)}`;

export const GET = proxyGet(endpoint);
export const PUT = proxyPut(endpoint);
export const DELETE = proxyDelete(endpoint);
