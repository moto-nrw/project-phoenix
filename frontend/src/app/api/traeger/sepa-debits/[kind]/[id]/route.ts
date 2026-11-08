import { sepaDebitRunPath } from "~/lib/traeger-sepa-debit-paths.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Die Lastschriftdatei eines ausgestellten Laufs (#4697): was in die Datei
// geht, was draußen bleibt und warum, ohne Bankdaten der Familien.
export const GET = proxyGet((params) => sepaDebitRunPath(params));
