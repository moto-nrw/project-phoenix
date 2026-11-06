import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Auffälligkeiten aller Schulen des Trägers gegen seine Grenzen (#4675).
export const GET = proxyGet("/traeger/schools/alerts");
