"use client";

// Die Lastschrift der Raten eines Jahreslaufs (#4697, ADR 0182): Die Raten
// eines Monats haben eine eigene Datei für die Bank. Die Kämmerei wählt
// den Monat; die Karte zeigt dessen Datei wie die eines Monatslaufs.

import { useState } from "react";
import { formatMonth } from "~/components/traeger/care-billing-format";
import { SepaDebitCard } from "~/components/traeger/sepa-debit-card";
import { CustomSelect } from "~/components/ui/custom-select";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";

/** The first days of the months from from to until, "YYYY-MM-01". */
function monthsBetween(from: string, until: string): string[] {
  const months: string[] = [];
  let [year, month] = from.split("-").map(Number) as [number, number];
  const [lastYear, lastMonth] = until.split("-").map(Number) as [
    number,
    number,
  ];
  while (year < lastYear || (year === lastYear && month <= lastMonth)) {
    months.push(`${year}-${String(month).padStart(2, "0")}-01`);
    month += 1;
    if (month > 12) {
      month = 1;
      year += 1;
    }
  }
  return months;
}

export function NoticeSepaDebits({
  periodFrom,
  periodUntil,
}: Readonly<{ periodFrom: string; periodUntil: string }>) {
  const today = useBerlinToday();
  const months = monthsBetween(periodFrom, periodUntil);
  const current = `${today.slice(0, 7)}-01`;
  const [month, setMonth] = useState(
    months.includes(current) ? current : (months[0] ?? current),
  );
  const runId = month.slice(0, 4) + month.slice(5, 7);
  return (
    <SepaDebitCard
      key={runId}
      kind="notice-months"
      runId={runId}
      title="Lastschrift der Raten"
      control={
        <div className="max-w-xs">
          <CustomSelect
            label="Raten fällig im"
            value={month}
            options={months.map((value) => ({
              value,
              label: formatMonth(value),
            }))}
            onChange={setMonth}
          />
        </div>
      }
    />
  );
}
