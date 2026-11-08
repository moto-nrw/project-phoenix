import { bffError } from "~/lib/api-response-error.server";
import { requireNumericParam } from "~/lib/route-wrapper-utils.server";

// The backend path of the debit file of a run (#4697): only the two kinds of
// run that issue invoices, only a numeric run ID.
const SEPA_DEBIT_RUN_KINDS = new Set(["care-runs", "holiday-runs"]);

export function sepaDebitRunPath(params: Record<string, unknown>): string {
  const kind = params.kind;
  if (typeof kind !== "string" || !SEPA_DEBIT_RUN_KINDS.has(kind)) {
    throw bffError(404, "unknown kind of run");
  }
  return `/traeger/sepa-debits/${kind}/${requireNumericParam(params)}`;
}
