"use client";

import { useState } from "react";

import { cn } from "~/lib/utils";

import { Button } from "./button";
import { normalizeSearch, SEARCHABLE_PORTAL_SCOPE } from "./custom-select";
import {
  ListboxDropdown,
  ListboxTriggerContent,
  type ListboxDropdownSize,
} from "./listbox-dropdown";

export interface MultiSelectOption {
  readonly value: string;
  readonly label: string;
  readonly disabled?: boolean;
  /** Short gray note under the label ("Zählt nicht als Betreuungstag"). */
  readonly badge?: string;
}

interface MultiSelectProps {
  /** Currently selected values. Empty = no restriction ("alle"). */
  readonly value: readonly string[];
  readonly options: readonly MultiSelectOption[];
  readonly onChange: (values: string[]) => void;
  readonly id?: string;
  readonly ariaLabel?: string;
  readonly ariaLabelledBy?: string;
  readonly ariaDescribedBy?: string;
  /** The field failed a check: red frame and `aria-invalid`. */
  readonly invalid?: boolean;
  readonly disabled?: boolean;
  /** Layout classes for the trigger (width, flex). The look is the kit's. */
  readonly className?: string;
  /** Layout classes for the menu, e.g. a minimum width. */
  readonly menuClassName?: string;
  /** Opens the menu from the trigger's right edge, e.g. near the window's. */
  readonly menuAlign?: "start" | "end";
  /** Field height: `default` 40 px, `compact` 36 px for dense rows. */
  readonly controlSize?: ListboxDropdownSize;
  /**
   * Trigger text for an empty selection — say what "nothing selected" means in
   * this field's own words ("Alle Klassen"), never a bare "Bitte wählen".
   */
  readonly placeholder?: string;
  /**
   * Trigger text while there is nothing to choose from (the field is then
   * disabled). Without it the placeholder stays.
   */
  readonly unavailableLabel?: string;
  /**
   * Trigger text once more than `summaryThreshold` values are selected, as
   * `(count, selected) => string` (e.g. `(n) => \`${n} Klassen\``). A single
   * selection always shows its own label; from two on, the default threshold
   * of 1 switches to the summary because two full labels ("Klasse 3a, Klasse
   * 4b") truncate to uselessness in a filter-width trigger. Raise it for a
   * wide trigger that can carry the labels, or set 0 to summarise every
   * selection (short marks such as allergen letters).
   */
  readonly summaryLabel?: (
    count: number,
    selected: readonly MultiSelectOption[],
  ) => string;
  readonly summaryThreshold?: number;
  /**
   * Filter field at the top of the menu for long lists (all children of a
   * school, all AGs). Typeahead switches off while it is there.
   */
  readonly searchable?: boolean;
  readonly searchPlaceholder?: string;
  /**
   * "Alle auswählen / Auswahl aufheben" above the list. With a search term
   * it acts on the matching options only.
   */
  readonly selectAll?: boolean;
  readonly testId?: string;
}

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
  ariaDescribedBy,
  invalid = false,
  disabled = false,
  className = "",
  menuClassName = "",
  menuAlign = "start",
  controlSize = "default",
  placeholder = "Alle",
  unavailableLabel,
  summaryLabel,
  summaryThreshold = 1,
  searchable = false,
  searchPlaceholder = "Suchen",
  selectAll = false,
  testId,
}: MultiSelectProps) {
  const [query, setQuery] = useState("");
  const selected = [...new Set(value)];
  const selectedSet = new Set(selected);
  const selectedOptions = options.filter((option) =>
    selectedSet.has(option.value),
  );
  const labelFor = (optionValue: string) =>
    options.find((option) => option.value === optionValue)?.label ??
    optionValue;
  // Nothing to choose from: an open, empty menu would only be a dead end.
  const unavailable = options.length === 0;

  const triggerLabel = (() => {
    if (unavailable && unavailableLabel) return unavailableLabel;
    if (selected.length === 0) return placeholder;
    if (summaryLabel && selected.length > summaryThreshold) {
      return summaryLabel(selected.length, selectedOptions);
    }
    return selected.map(labelFor).join(", ");
  })();

  const needle = normalizeSearch(query);
  const visibleOptions =
    searchable && needle
      ? options.filter((option) =>
          normalizeSearch(option.label).includes(needle),
        )
      : options;
  const selectableVisible = visibleOptions.filter((option) => !option.disabled);
  const allVisibleSelected =
    selectableVisible.length > 0 &&
    selectableVisible.every((option) => selectedSet.has(option.value));

  const toggle = (optionValue: string) => {
    onChange(
      selectedSet.has(optionValue)
        ? selected.filter((entry) => entry !== optionValue)
        : [...selected, optionValue],
    );
  };

  const toggleAllVisible = () => {
    const visible = new Set(selectableVisible.map((option) => option.value));
    onChange(
      allVisibleSelected
        ? selected.filter((entry) => !visible.has(entry))
        : [...new Set([...selected, ...visible])],
    );
  };

  const selectAllLabel = allVisibleSelected
    ? needle
      ? "Treffer abwählen"
      : "Auswahl aufheben"
    : needle
      ? "Alle Treffer auswählen"
      : "Alle auswählen";

  const menuHeader =
    selectAll && selectableVisible.length > 0 ? (
      <div className="border-b border-gray-100 px-4 pt-1 pb-2">
        <Button
          type="button"
          variant="link"
          size="md"
          onClick={toggleAllVisible}
        >
          {selectAllLabel}
        </Button>
      </div>
    ) : undefined;

  return (
    <ListboxDropdown
      id={id}
      // Anchors keyboard focus on the first selected option when the menu
      // opens; the checked state itself comes from selectedValues.
      value={
        selected.find((entry) =>
          visibleOptions.some((o) => o.value === entry),
        ) ?? ""
      }
      selectedValues={selected}
      options={visibleOptions}
      onChange={toggle}
      ariaLabel={ariaLabel}
      ariaLabelledBy={ariaLabelledBy}
      ariaDescribedBy={ariaDescribedBy}
      invalid={invalid}
      disabled={disabled || unavailable}
      placeholder={placeholder}
      triggerRole="button"
      testId={testId}
      size={controlSize}
      className={cn("w-full", className)}
      menuClassName={menuClassName}
      menuAlign={menuAlign}
      menuHeader={menuHeader}
      {...(searchable
        ? {
            searchValue: query,
            onSearchChange: setQuery,
            searchPlaceholder,
            onOpenChange: (open: boolean) => {
              if (!open) setQuery("");
            },
            portalScopeSelector: SEARCHABLE_PORTAL_SCOPE,
            emptyState: (
              <li role="presentation">
                <p className="px-4 py-2 text-sm text-gray-500">
                  Keine Treffer.
                </p>
              </li>
            ),
          }
        : {})}
      renderTrigger={({ open }) => (
        <ListboxTriggerContent
          label={triggerLabel}
          placeholderShown={selected.length === 0}
          open={open}
        />
      )}
    />
  );
}
