import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Vorlagen des Trägers je Dokumentart (#4677): der Stand jeder Art.
export const GET = proxyGet("/traeger/master-data/document-templates");
