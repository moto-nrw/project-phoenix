import { proxyDelete, proxyPut } from "~/lib/school/route-wrapper.server";

// Eigenen Schultermin ändern oder löschen (#4798). Ob die Lehrkraft den
// Termin angelegt hat, prüft das Backend.
const endpoint = (params: Record<string, unknown>) =>
  `/school/calendar/appointments/${encodeURIComponent(params.appointmentId as string)}`;

export const PUT = proxyPut(endpoint);
export const DELETE = proxyDelete(endpoint);
