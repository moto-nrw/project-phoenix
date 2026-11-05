"use client";

import { normalizeFilterValues, type FilterConfig } from "./types";
import {
  ListboxChevron,
  ListboxDropdown,
} from "~/components/ui/listbox-dropdown";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { ToggleChip } from "~/components/ui/toggle-chip";
import { cn } from "~/lib/utils";

interface DesktopFiltersProps {
  readonly filters: ReadonlyArray<FilterConfig>;
  readonly className?: string;
}

export function DesktopFilters({
  filters,
  className = "",
}: Readonly<DesktopFiltersProps>) {
  return (
    <div className={`flex flex-wrap gap-2 ${className}`}>
      {filters.map((filter) => (
        <FilterControl key={filter.id} filter={filter} />
      ))}
    </div>
  );
}

function FilterControl({ filter }: Readonly<{ filter: FilterConfig }>) {
  if (filter.type === "custom") {
    // A custom filter carries its own control; value/options are unused stubs.
    // Rendering it here (rather than skipping it) keeps a filter set from
    // silently losing a control when a consumer uses the inline desktop row
    // instead of the quiet popover panel.
    return filter.render ?? null;
  }

  if (filter.type === "buttons") {
    const selectedValues = normalizeFilterValues(filter.value);

    // Mehrfachauswahl ist eine Reihe einzeln schaltbarer Merkmale, keine
    // Umschaltleiste. Vorher trug sie die Optik des SegmentedControls und
    // stand damit als zweite, gleich aussehende Reiterleiste über den echten
    // Reitern der Seite (Anfragen: "Stammdaten | Betreuungszeiten | …" direkt
    // über "Eltern | Mitarbeitende"). Das Kit trennt beides ausdrücklich:
    // SegmentedControl wählt GENAU EINE Option, ToggleChip schaltet jede für
    // sich.
    if (filter.multiSelect) {
      return (
        <div
          className="flex flex-wrap gap-2"
          role="group"
          aria-label={filter.label}
        >
          {filter.options.map((option) => (
            <ToggleChip
              key={option.value}
              pressed={selectedValues.includes(option.value)}
              onPressedChange={(next) =>
                filter.onChange(
                  next
                    ? [...selectedValues, option.value]
                    : selectedValues.filter((v) => v !== option.value),
                )
              }
            >
              {option.label}
            </ToggleChip>
          ))}
        </div>
      );
    }

    return (
      <SegmentedControl
        ariaLabel={filter.label}
        value={selectedValues[0] ?? filter.options[0]?.value ?? ""}
        onChange={(value) => filter.onChange(value)}
        items={filter.options.map((option) => ({
          value: option.value,
          label: option.label,
        }))}
      />
    );
  }

  if (filter.type === "dropdown") {
    return <DropdownFilter filter={filter} />;
  }

  if (filter.type === "grid") {
    // For desktop, show grid filters as a dropdown with icons
    return <DropdownFilter filter={filter} showIcons />;
  }

  return null;
}

function FilterIcon({
  path,
  className,
}: Readonly<{ path: string; className: string }>) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d={path}
      />
    </svg>
  );
}

// The filter chip keeps its own look (white, no frame, blue ring while a
// restriction is active); menu, options, keyboard and portal come from the
// kit listbox (#5580).
function DropdownFilter({
  filter,
  showIcons = false,
}: Readonly<{
  filter: FilterConfig;
  showIcons?: boolean;
}>) {
  const isMulti = !!filter.multiSelect;
  const selectedValues = normalizeFilterValues(filter.value);

  const selectedOption = isMulti
    ? undefined
    : filter.options.find((opt) => opt.value === filter.value);

  // "Filtered" = a real restriction is active. For multi-select that means the
  // selection is neither empty (no restriction) nor the full visible option set.
  // Compare by MEMBERSHIP, not length: a saved selection can hold a hidden value
  // (e.g. a group/class the current user can't see) that pads the count to
  // options.length while still excluding a visible option — selected [A, hiddenC]
  // with options [A, B] is a real restriction, not "all selected". A selected
  // value outside the visible options is itself a restriction to surface (#1565
  // review pass 2). For single-select it means anything other than the
  // first/default option.
  const optionValueSet = new Set(filter.options.map((opt) => opt.value));
  const selectedValueSet = new Set(selectedValues);
  const allOptionsSelected = filter.options.every((opt) =>
    selectedValueSet.has(opt.value),
  );
  const hasHiddenSelection = selectedValues.some(
    (value) => !optionValueSet.has(value),
  );
  const isFiltered = isMulti
    ? selectedValues.length > 0 && (!allOptionsSelected || hasHiddenSelection)
    : filter.value !== filter.options[0]?.value;

  const options = filter.options.map((option) => ({
    value: option.value,
    label:
      option.count !== undefined
        ? `${option.label} (${option.count})`
        : option.label,
  }));

  return (
    <ListboxDropdown
      value={
        isMulti ? (selectedValues[0] ?? "") : (selectedOption?.value ?? "")
      }
      selectedValues={isMulti ? selectedValues : undefined}
      options={options}
      onChange={(next) => {
        if (isMulti) {
          filter.onChange(
            selectedValues.includes(next)
              ? selectedValues.filter((v) => v !== next)
              : [...selectedValues, next],
          );
        } else {
          filter.onChange(next);
        }
      }}
      // A single-select chip names itself by its current value, as before
      // ("Alle Status"); the multi chip shows the filter's label anyway.
      ariaLabel={isMulti ? filter.label : undefined}
      menuClassName="min-w-48"
      unstyledTrigger
      className={cn(
        "flex h-10 items-center gap-2 rounded-xl bg-white px-3 py-2 text-sm font-medium whitespace-nowrap shadow-sm transition-all hover:bg-gray-50 focus-visible:outline-2 focus-visible:outline-gray-400 aria-expanded:bg-gray-50",
        isFiltered && "ring-moto-blue ring-2 ring-offset-1",
      )}
      renderTrigger={({ open }) => (
        <>
          {showIcons && selectedOption?.icon ? (
            <FilterIcon
              path={selectedOption.icon}
              className="h-4 w-4 flex-shrink-0 text-gray-600"
            />
          ) : null}
          <span
            className={cn(
              "whitespace-nowrap",
              selectedOption && filter.value !== filter.options[0]?.value
                ? "text-gray-900"
                : "text-gray-600",
            )}
          >
            {isMulti ? filter.label : (selectedOption?.label ?? filter.label)}
          </span>
          <ListboxChevron open={open} />
        </>
      )}
      renderOption={(option) => {
        const source = filter.options.find(
          (entry) => entry.value === option.value,
        );
        return (
          <>
            {showIcons && source?.icon ? (
              <FilterIcon
                path={source.icon}
                className="h-4 w-4 flex-shrink-0"
              />
            ) : null}
            <span className="min-w-0 flex-1 truncate">
              {source?.label ?? option.label}
            </span>
            {source?.count !== undefined ? (
              <span className="ml-1 font-normal text-gray-500">
                ({source.count})
              </span>
            ) : null}
          </>
        );
      }}
    />
  );
}
