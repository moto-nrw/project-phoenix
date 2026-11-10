import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Exportprotokoll des Trägers, auf Wunsch eines Laufs (#4693).
export const GET = proxyGet("/traeger/finance-exports/files");
