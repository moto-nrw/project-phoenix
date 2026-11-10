/**
 * What the page „Konten und Kostenstellen“ shows (#4821): which revenue
 * account a Leistung or an Angebot books on, which cost center holds on a
 * day, and the rules a value must meet. The rules are those of the backend
 * and of DATEV (backend/testdata/finance-booking-accounts-cases.json); the
 * backend checks again when it saves.
 */

import type {
  BookingAccounts,
  CostCenter,
  CostCenterScope,
  RevenueScope,
} from "~/lib/traeger-finance-exports-api";

/** The Leistungen with their own revenue account, in the order shown. */
export const REVENUE_SERVICES: readonly {
  readonly scope: Exclude<RevenueScope, "offering">;
  readonly label: string;
  readonly hint: string;
}[] = [
  {
    scope: "care",
    label: "Monatsbeiträge",
    hint: "Beiträge der Betreuung je Monat",
  },
  { scope: "meals", label: "Mensa", hint: "Essen je Monat" },
  {
    scope: "holiday",
    label: "Ferienbetreuung",
    hint: "Rechnungen der Ferienläufe",
  },
];

/** Where a revenue account comes from. */
type RevenueSource = "own" | "service" | "standard" | "missing";

export interface EffectiveRevenue {
  readonly account: string;
  readonly source: RevenueSource;
}

/** The own revenue account of a scope, "" without one. */
export function ownRevenueAccount(
  page: BookingAccounts,
  scope: RevenueScope,
  scopeId = "0",
): string {
  return (
    page.revenueAccounts.find(
      (account) => account.scope === scope && account.scopeId === scopeId,
    )?.account ?? ""
  );
}

/**
 * The account a scope books on: its own, for an Angebot that of the
 * monthly contributions, then the standard of the DATEV values.
 */
export function effectiveRevenue(
  page: BookingAccounts,
  scope: RevenueScope,
  scopeId = "0",
): EffectiveRevenue {
  const own = ownRevenueAccount(page, scope, scopeId);
  if (own) return { account: own, source: "own" };
  if (scope === "offering") {
    const care = ownRevenueAccount(page, "care");
    if (care) return { account: care, source: "service" };
  }
  if (page.standardAccount) {
    return { account: page.standardAccount, source: "standard" };
  }
  return { account: "", source: "missing" };
}

/** The cost centers of a scope, earliest first. */
export function costCenterTimeline(
  centers: readonly CostCenter[],
  scope: CostCenterScope,
  scopeId: string,
): CostCenter[] {
  return centers
    .filter((center) => center.scope === scope && center.scopeId === scopeId)
    .sort((a, b) => a.validFrom.localeCompare(b.validFrom));
}

/**
 * The cost center that holds on day ("YYYY-MM-DD"): the latest one from
 * that day or before. Null before the first; an entry with an empty code
 * ended the cost center.
 */
export function costCenterOn(
  timeline: readonly CostCenter[],
  day: string,
): CostCenter | null {
  let current: CostCenter | null = null;
  for (const center of timeline) {
    if (center.validFrom > day) break;
    current = center;
  }
  return current;
}

/** The next change after day, null without one. */
export function nextCostCenter(
  timeline: readonly CostCenter[],
  day: string,
): CostCenter | null {
  return timeline.find((center) => center.validFrom > day) ?? null;
}

const REVENUE_ACCOUNT = /^0*[1-9][0-9]*$/;
const COST_CENTER = /^[A-Za-z0-9_ ]*$/;

/** Why a revenue account cannot be saved, null when it can; "" removes. */
export function revenueAccountError(value: string): string | null {
  const account = value.trim();
  if (account === "") return null;
  if (!REVENUE_ACCOUNT.test(account)) {
    return "Nur Ziffern, nicht nur Nullen.";
  }
  if (account.length > 8) return "Höchstens 8 Ziffern.";
  return null;
}

/** Why a cost center cannot be saved, null when it can. */
export function costCenterError(value: string): string | null {
  const code = value.trim();
  if (code.length > 36) return "Höchstens 36 Zeichen.";
  if (!COST_CENTER.test(code)) {
    return "Nur Buchstaben ohne Umlaute, Ziffern, Leerzeichen und _.";
  }
  return null;
}
