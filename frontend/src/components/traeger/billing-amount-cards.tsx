"use client";

// Die vier Summen der Abrechnung je Familie oder über alle Familien (#4691):
// ausgestellt, storniert, gutgeschrieben und berechnet. Berechnet ist, was die
// Rechnungen nach Storno und Gutschrift verlangen. Zahlungseingänge gleicht
// moto nicht ab; deshalb heißt die Summe nicht „offen“: das läse sich wie
// „noch nicht gezahlt“.

import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import type { BillingAmounts } from "~/lib/traeger-billing-families-api";
import { formatEuro } from "./contribution-format";

export const OPEN_AMOUNT_HINT =
  "Ausgestellt minus Storno und Gutschrift. Ob gezahlt wurde, sieht moto nicht.";

export function BillingAmountCards({
  amounts,
  loading = false,
}: Readonly<{ amounts: BillingAmounts | null; loading?: boolean }>) {
  const value = (cents: number | undefined) => formatEuro(cents ?? 0);
  return (
    <div className="grid grid-cols-1 gap-3 min-[375px]:grid-cols-2 sm:gap-4 xl:grid-cols-4">
      <StatCard
        label="Ausgestellt"
        value={value(amounts?.issuedCents)}
        hint="Alle Rechnungen."
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
        tone="neutral"
        compactValue
        loading={loading}
      />
      <StatCard
        label="Berechnet"
        value={value(amounts?.openCents)}
        hint={OPEN_AMOUNT_HINT}
        tone="neutral"
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

/**
 * The four sums inside one card whose title says whom they cover (all or the
 * found families): the scope belongs on the surface with the figures.
 */
export function BillingAmountsSection({
  title,
  amounts,
}: Readonly<{ title: string; amounts: BillingAmounts }>) {
  const value = (cents: number) => formatEuro(cents);
  return (
    <SectionCard title={title} description={`Berechnet: ${OPEN_AMOUNT_HINT}`}>
      <div className="grid grid-cols-1 gap-3 min-[375px]:grid-cols-2 xl:grid-cols-4">
        <StatCard
          variant="inline"
          label="Ausgestellt"
          value={value(amounts.issuedCents)}
        />
        <StatCard
          variant="inline"
          label="Storniert"
          value={value(amounts.cancelledCents)}
        />
        <StatCard
          variant="inline"
          label="Gutgeschrieben"
          value={value(amounts.creditedCents)}
        />
        <StatCard
          variant="inline"
          label="Berechnet"
          value={value(amounts.openCents)}
        />
      </div>
    </SectionCard>
  );
}
