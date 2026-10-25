import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Absprung in eine OGS (#4654): das Backend prüft Recht und Träger der Schule
// und antwortet mit einem kurzlebigen Übergabe-Token für den Host der Schule.
export const POST = proxyPost("/traeger/school-sessions");
