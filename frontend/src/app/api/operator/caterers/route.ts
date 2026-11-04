import { proxyGet } from "~/lib/operator/route-wrapper.server";

// Die Caterer mit ihrem Konto und dem Stand der Zwei-Faktor-Anmeldung
// (#4497), für das Zurücksetzen durch den Operator.
export const GET = proxyGet("/operator/caterers");
