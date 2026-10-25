import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Eine Bankverbindung des Trägers (#4653): ändern, als Standard festlegen oder
// löschen.
const bankAccountPath = (params: Record<string, unknown>) =>
  `/traeger/master-data/bank-accounts/${requirePathSegmentParam(params)}`;

export const PUT = proxyPut(bankAccountPath);

export const DELETE = proxyDelete(bankAccountPath);
