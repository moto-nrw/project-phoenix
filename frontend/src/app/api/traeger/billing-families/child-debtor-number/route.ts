import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Debitorennummer eines Kindes einer Familie setzen oder entfernen (#7007),
// wenn der Träger die Nummer je Kind führt.
export const PUT = proxyPut("/traeger/billing-families/child-debtor-number");
