"use client";

// Die vier Summen der Abrechnung je Familie oder über alle Familien (#4691):
// ausgestellt, storniert, gutgeschrieben und offen. Offen ist, was die
// Rechnungen nach Storno und Gutschrift noch verlangen; Zahlungseingänge
// gleicht moto nicht ab.

import { StatCard } from "~/components/ui/stat-card";
import type { BillingAmounts } from "~/lib/traeger-billing-families-api";
import { formatEuro } from "./contribution-format";

export const OPEN_AMOUNT_HINT =
  "Ausgestellt, nicht storniert und nicht gutgeschrieben. Ob gezahlt wurde, prüft moto nicht.";

export function BillingAmountCards({
  amounts,
  loading = false,
}: Readonly<{ amounts: BillingAmounts | null; loading?: boolean }>) {
  const value = (cents: number | undefined) => formatEuro(cents ?? 0);
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <StatCard
        label="Ausgestellt"
        value={value(amounts?.issuedCents)}
        hint="Summe aller Rechnungen."
        tone="neutral"
        compactValue
        loading={loading}
      />
      <StatCard
        label="Storniert"
        value={value(amounts?.cancelledCents)}
        hint="Durch Storno ganz zurückgenommen."
        tone="gray"
        compactValue
        loading={loading}
      />
      <StatCard
        label="Gutgeschrieben"
        value={value(amounts?.creditedCents)}
        hint="Durch Gutschrift zurückgenommen."
        tone="blue"
        compactValue
        loading={loading}
      />
      <StatCard
        label="Offen"
        value={value(amounts?.openCents)}
        hint={OPEN_AMOUNT_HINT}
        tone={amounts && amounts.openCents > 0 ? "orange" : "neutral"}
        compactValue
        loading={loading}
      />
    </div>
  );
}

/** Adds the sums of several families. */
export function sumAmounts(
  families: readonly BillingAmounts[],
): BillingAmounts {
  return families.reduce<BillingAmounts>(
    (sum, family) => ({
      issuedCents: sum.issuedCents + family.issuedCents,
      cancelledCents: sum.cancelledCents + family.cancelledCents,
      creditedCents: sum.creditedCents + family.creditedCents,
      openCents: sum.openCents + family.openCents,
    }),
    { issuedCents: 0, cancelledCents: 0, creditedCents: 0, openCents: 0 },
  );
}
