"use client";

// Die vier Summen der Abrechnung je Familie oder über alle Familien (#4691):
// ausgestellt, storniert, gutgeschrieben und die offene Forderung. Sie ist,
// was die Rechnungen nach Storno und Gutschrift noch verlangen, vor jedem
// Zahlungseingang: Zahlungen gleicht die Buchhaltung ab, nicht moto (#6348).

import { StatCard } from "~/components/ui/stat-card";
import type { BillingAmounts } from "~/lib/traeger-billing-families-api";
import { formatEuro } from "./contribution-format";

const OPEN_AMOUNT_HINT =
  "Stand vor Zahlungseingang. Zahlungen sieht moto nicht.";

export function BillingAmountCards({
  amounts,
  loading = false,
  scope,
}: Readonly<{
  amounts: BillingAmounts | null;
  loading?: boolean;
  /**
   * Whom the sums cover when a search or filter narrows the list
   * („Rechnungen der 2 gefundenen Familien.“); without it: all invoices.
   */
  scope?: string;
}>) {
  const value = (cents: number | undefined) => formatEuro(cents ?? 0);
  return (
    // Four in a row from xl on; a tablet keeps two per row, so every hint
    // stays short beside its figure.
    <div className="grid grid-cols-1 gap-3 min-[375px]:grid-cols-2 sm:gap-4 xl:grid-cols-4">
      <StatCard
        label="Ausgestellt"
        value={value(amounts?.issuedCents)}
        hint={scope ?? "Alle Rechnungen."}
        tone="neutral"
        compactValue
        loading={loading}
      />
      <StatCard
        label="Storniert"
        value={value(amounts?.cancelledCents)}
        hint="Durch Storno aufgehoben."
        tone="gray"
        compactValue
        loading={loading}
      />
      <StatCard
        label="Gutgeschrieben"
        value={value(amounts?.creditedCents)}
        hint="Durch Gutschrift erlassen."
        tone="blue"
        compactValue
        loading={loading}
      />
      <StatCard
        label="Offene Forderung"
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
