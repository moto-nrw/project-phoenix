import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Telefon und E-Mail des Trägers für Widerrufe (#6200).
export const PUT = proxyPut("/traeger/master-data/withdrawal-contact");
