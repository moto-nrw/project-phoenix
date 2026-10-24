import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Schulen des Trägers (#4649): das Backend liest sie aus der Projektion
// traeger-view, gefiltert auf den Träger der Sitzung.
export const GET = proxyGet("/traeger/schools");
