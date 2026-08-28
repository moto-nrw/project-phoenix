import { proxyGet } from "~/lib/school/route-wrapper.server";

// Wen eine Lehrkraft anschreiben darf: aktive Personen der Schule, Name und
// Rollenart, keine Kontaktdaten (#2208).
export const GET = proxyGet("/school/staff-messages/recipients");
