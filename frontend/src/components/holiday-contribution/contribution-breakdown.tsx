// Aufschlüsselung eines Beitrags (#4664): je Kind eine Zeile je Position mit
// dem Betrag rechts und die Summe darunter. Die Eltern sehen sie in der
// Zusammenfassung der Ferienanmeldung, das Träger-Büro in der Anmeldung; die
// Texte bringt der Aufrufer in seiner Sprache mit.

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
    <div className="space-y-4 text-sm">
      {items.map((child) => (
        <div key={child.key} className="space-y-1">
          <p className="font-semibold text-gray-900">{child.name}</p>
          <ul className="divide-y divide-gray-100">
            {child.lines.map((line, index) => (
              <li
                key={`${child.key}-${index}`}
                className="flex items-baseline justify-between gap-4 py-1.5"
              >
                <span className="min-w-0 text-gray-700">{line.label}</span>
                <span className="shrink-0 text-gray-900 tabular-nums">
                  {line.amount}
                </span>
              </li>
            ))}
            <li className="flex items-baseline justify-between gap-4 py-1.5">
              <span className="font-medium text-gray-900">
                {childTotalLabel}
              </span>
              <span className="shrink-0 font-semibold text-gray-900 tabular-nums">
                {child.total}
              </span>
            </li>
          </ul>
        </div>
      ))}
      {items.length > 1 ? (
        <div className="flex items-baseline justify-between gap-4 border-t border-gray-200 pt-3">
          <p className="font-semibold text-gray-900">{totalLabel}</p>
          <p className="shrink-0 font-semibold text-gray-900 tabular-nums">
            {total}
          </p>
        </div>
      ) : null}
    </div>
  );
}
