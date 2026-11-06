import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Belegung aller Schulen des Trägers je Gruppe und Betreuungsform (#4676).
export const GET = proxyGet("/traeger/schools/occupancy");
