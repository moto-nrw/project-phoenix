/**
 * Segmentierter Umschalter in der Optik der Filterleiste.
 *
 * Das Rezept (weiße Schiene, dunkles aktives Segment) steckte bisher
 * ausschließlich im Seitenkopf: `DesktopFilters` rendert es für Filter vom Typ
 * `buttons`, `FilterPanel` rendert die Mobilvariante. Beide sind fest an die
 * `FilterConfig`-Struktur von `PageHeaderWithSearch` gebunden und damit außerhalb
 * eines Seitenkopfs nicht verwendbar. Für eine Auswahl innerhalb eines Dialogs
 * gab es deshalb kein Kit-Element — genau die Lücke, die sonst wieder zu einem
 * handgebauten Stepper führt.
 *
 * Semantik ist eine Radiogruppe, nicht eine Reihe Buttons: es wird genau eine
 * Option gewählt, und Screenreader sollen "2 von 4" ansagen statt vier
 * unabhängige Schaltflächen.
 */
export interface SegmentedControlOption {
  readonly value: string;
  readonly label: string;
}

export function SegmentedControl({
  options,
  value,
  onChange,
  ariaLabel,
  ariaLabelledBy,
  className = "",
}: {
  readonly options: readonly SegmentedControlOption[];
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly ariaLabel?: string;
  readonly ariaLabelledBy?: string;
  readonly className?: string;
}) {
  return (
    <div
      role="radiogroup"
      aria-label={ariaLabel}
      aria-labelledby={ariaLabelledBy}
      className={`inline-flex rounded-xl bg-white p-1 shadow-sm ring-1 ring-gray-200 ring-inset ${className}`}
    >
      {options.map((option) => {
        const selected = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onChange(option.value)}
            className={`rounded-lg px-3 py-1.5 text-sm font-medium whitespace-nowrap transition-colors ${
              selected
                ? "bg-gray-900 text-white"
                : "text-gray-600 hover:text-gray-900"
            }`}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
