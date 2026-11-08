import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import { sepaDebitRunPath } from "~/lib/traeger-sepa-debit-paths.server";
import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// Die SEPA-Lastschriftdatei (PAIN.008) eines ausgestellten Laufs (#4697).
// Sie trägt Bankdaten; niemand speichert sie zwischen.
export const GET = withPrivateDownloadNoStore(
  proxyFile((params) => `${sepaDebitRunPath(params)}/file`),
);
