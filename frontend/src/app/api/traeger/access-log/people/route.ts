import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Personen des Zugriffsprotokolls (#4655) für den Filter nach Person.
export const GET = proxyGet("/traeger/access-log/people");
