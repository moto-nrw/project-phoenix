import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Debitorennummer einer Familie setzen (#6692), auch einer Familie ohne
// Betreuungsvertrag. Die Familie ist ihre E-Mail-Adresse.
export const PUT = proxyPut("/traeger/billing-families/debtor-number");
