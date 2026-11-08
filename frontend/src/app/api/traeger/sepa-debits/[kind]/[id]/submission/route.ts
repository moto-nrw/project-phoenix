import { sepaDebitRunPath } from "~/lib/traeger-sepa-debit-paths.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Bestätigen, dass die Lastschriftdatei bei der Bank eingereicht ist (#4697).
export const POST = proxyPost(
  (params) => `${sepaDebitRunPath(params)}/submission`,
);
