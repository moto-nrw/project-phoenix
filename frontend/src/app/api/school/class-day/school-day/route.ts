import { proxyGet } from "~/lib/school/route-wrapper.server";

// Tagesübersicht der ganzen Schule im Schul-Portal (#4802): leitet die
// date-Query mit dem school-Token weiter; nur die Schulverwaltung
// (class_day:all_classes) bekommt eine Antwort.
export const GET = proxyGet("/school/class-day/school-day");
