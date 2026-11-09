import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Feldzuordnung der Sollstellung und Dateinummern des Trägers (#4693), nur
// mit dem Recht „Finanz-Export“.
export const GET = proxyGet("/traeger/finance-exports/settings");
