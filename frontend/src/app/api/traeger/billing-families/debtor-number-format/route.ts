import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Das Format der Debitorennummer des Trägers (#6675), damit die Felder der
// Familien es nennen und vorab prüfen.
export const GET = proxyGet("/traeger/billing-families/debtor-number-format");
