// Aufschlüsselung eines Beitrags (#4664): je Kind eine Zeile je Position mit
// dem Betrag rechts und die Summe darunter, aus den Feldern des Kits. Die
// Eltern sehen sie in der Zusammenfassung der Ferienanmeldung, das
// Träger-Büro in der Anmeldung; die Texte bringt der Aufrufer in seiner
// Sprache mit.

import { DataField, DataGrid } from "~/components/ui/detail-modal-components";

export interface ContributionBreakdownChild {
  readonly key: string;
  readonly name: string;
  readonly lines: readonly {
    readonly label: string;
    readonly amount: string;
  }[];
  readonly total: string;
}

export function ContributionBreakdown({
  items,
  totalLabel,
  childTotalLabel,
  total,
}: Readonly<{
  items: readonly ContributionBreakdownChild[];
  /** Shown under all children when there is more than one. */
  totalLabel: string;
  childTotalLabel: string;
  total: string;
}>) {
  return (
    <div className="space-y-4">
      {items.map((child) => (
        <div key={child.key} className="space-y-2">
          <p className="text-sm font-semibold text-gray-900">{child.name}</p>
          <DataGrid columns={1}>
            {child.lines.map((line, index) => (
              <DataField
                key={`${child.key}-${index}`}
                label={line.label}
                inline
              >
                <span className="tabular-nums">{line.amount}</span>
              </DataField>
            ))}
            <DataField label={childTotalLabel} inline>
              <span className="font-semibold tabular-nums">{child.total}</span>
            </DataField>
          </DataGrid>
        </div>
      ))}
      {items.length > 1 ? (
        <DataGrid columns={1}>
          <DataField label={totalLabel} inline>
            <span className="font-semibold tabular-nums">{total}</span>
          </DataField>
        </DataGrid>
      ) : null}
    </div>
  );
}
