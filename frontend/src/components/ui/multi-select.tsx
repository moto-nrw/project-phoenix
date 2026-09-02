"use client";

import { ChevronDown } from "lucide-react";

import { ListboxDropdown } from "./listbox-dropdown";

interface MultiSelectOption {
  readonly value: string;
  readonly label: string;
  readonly disabled?: boolean;
}

interface MultiSelectProps {
  /** Currently selected values. Empty = no restriction ("alle"). */
  readonly value: readonly string[];
  readonly options: readonly MultiSelectOption[];
  readonly onChange: (values: string[]) => void;
  readonly id?: string;
  readonly ariaLabel?: string;
  readonly ariaLabelledBy?: string;
  readonly disabled?: boolean;
  readonly className?: string;
  readonly menuClassName?: string;
  /** Replaces the default surface/size trigger classes. */
  readonly triggerClassName?: string;
  /**
   * Trigger text for an empty selection — say what "nothing selected" means in
   * this field's own words ("Alle Klassen"), never a bare "Bitte wählen".
   */
  readonly placeholder?: string;
  /**
   * Trigger text once more than `summaryThreshold` values are selected, as
   * `(count) => string` (e.g. `(n) => \`${n} Klassen\``). A single selection
   * always shows its own label; from two on, the default threshold of 1
   * switches to the summary because two full labels ("Klasse 3a, Klasse 4b")
   * truncate to uselessness in a filter-width trigger. Raise it for a wide
   * trigger that can carry the labels.
   */
  readonly summaryLabel?: (count: number) => string;
  readonly summaryThreshold?: number;
  readonly testId?: string;
}

const TRIGGER_BASE_CLASS =
  "flex items-center justify-between gap-2 rounded-lg border px-3 text-left text-sm shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500 disabled:opacity-80";

const DEFAULT_TRIGGER_CLASS =
  "moto-content-surface h-10 w-full hover:border-gray-300";

/**
 * MultiSelect: the kit's multi-value dropdown.
 *
 * Same trigger and menu as {@link CustomSelect} — it runs on the same
 * ListboxDropdown engine (portaled menu, keyboard navigation, typeahead) in
 * its multi mode — but each option is a checkable row and the menu stays open
 * while values are picked. Reach for it whenever a filter or field legitimately
 * takes several values at once; do NOT hand-roll a row of toggle chips, which
 * grows into an unreadable wall as soon as a school has more than a handful of
 * options.
 *
 * An empty selection means "no restriction". There is deliberately no neutral
 * "Alle …" option: it would be checkable alongside a real value and then have
 * to mean two contradictory things at once. Say it in `placeholder` instead.
 */
export function MultiSelect({
  value,
  options,
  onChange,
  id,
  ariaLabel,
  ariaLabelledBy,
  disabled = false,
  className = "",
  menuClassName = "",
  triggerClassName,
  placeholder = "Alle",
  summaryLabel,
  summaryThreshold = 1,
  testId,
}: MultiSelectProps) {
  const selected = [...new Set(value)];
  const labelFor = (optionValue: string) =>
    options.find((option) => option.value === optionValue)?.label ??
    optionValue;

  const triggerLabel = (() => {
    if (selected.length === 0) return placeholder;
    if (summaryLabel && selected.length > summaryThreshold) {
      return summaryLabel(selected.length);
    }
    return selected.map(labelFor).join(", ");
  })();

  const toggle = (optionValue: string) => {
    onChange(
      selected.includes(optionValue)
        ? selected.filter((entry) => entry !== optionValue)
        : [...selected, optionValue],
    );
  };

  return (
    <ListboxDropdown
      id={id}
      // Anchors keyboard focus on the first selected option when the menu
      // opens; the checked state itself comes from selectedValues.
      value={selected[0] ?? ""}
      selectedValues={selected}
      options={options}
      onChange={toggle}
      ariaLabel={ariaLabel}
      ariaLabelledBy={ariaLabelledBy}
      disabled={disabled}
      placeholder={placeholder}
      triggerRole="button"
      testId={testId}
      className={`${TRIGGER_BASE_CLASS} ${triggerClassName ?? DEFAULT_TRIGGER_CLASS} ${className}`}
      menuClassName={`scrollbar-thin overflow-y-auto moto-popover-surface rounded-xl border py-1 ${menuClassName}`}
      optionClassName="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50"
      activeOptionClassName="flex w-full items-center gap-2 bg-gray-50 px-4 py-2 text-left text-sm font-medium text-gray-900 transition-colors"
      disabledOptionClassName="flex w-full cursor-not-allowed items-center gap-2 px-4 py-2 text-left text-sm text-gray-400 transition-colors"
      renderTrigger={({ open }) => (
        <>
          <span
            className={`min-w-0 flex-1 truncate ${
              selected.length > 0 ? "text-gray-900" : "text-gray-500"
            }`}
          >
            {triggerLabel}
          </span>
          <ChevronDown
            aria-hidden="true"
            className={`h-4 w-4 flex-shrink-0 text-gray-400 transition-transform ${
              open ? "rotate-180" : ""
            }`}
          />
        </>
      )}
    />
  );
}
