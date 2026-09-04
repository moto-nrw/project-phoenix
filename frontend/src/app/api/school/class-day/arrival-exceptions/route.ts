import { proxyGet } from "~/lib/school/route-wrapper.server";

// Tagesausnahmen einer zugewiesenen Klasse ab heute (#2970): leitet die
// class-Query mit dem school-Token weiter; Zuweisung und Freigabe prüft das
// Backend.
export const GET = proxyGet("/school/class-day/arrival-exceptions");
