import { proxyGet } from "~/lib/school/route-wrapper.server";

// Eigene Aufsichten der Lehrkraft für heute (#2527). Das Backend filtert auf
// die Einteilung im Betreuungsplan; ein Datum nimmt die Route bewusst nicht
// entgegen.
export const GET = proxyGet("/school/supervisions/");
