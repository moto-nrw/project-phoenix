import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Zugriffsprotokoll des Trägers (#4655): eine Seite der Einträge an den
// Schulen des Trägers der Sitzung, nur mit dem Recht „Zugriffsprotokoll lesen“.
export const GET = proxyGet("/traeger/access-log");
