"use client";

import { useEffect } from "react";
import {
  normalizeFilterValues,
  type MobileFilterPanelProps,
  type FilterConfig,
} from "./types";

// Handler for single-select filter option
function handleSingleSelectClick(filter: FilterConfig, optionValue: string) {
  filter.onChange(optionValue);
}

// Handler for multi-select filter option - toggles selection
function handleMultiSelectClick(
  filter: FilterConfig,
  optionValue: string,
  selectedValues: string[],
) {
  const next = selectedValues.includes(optionValue)
    ? selectedValues.filter((v) => v !== optionValue)
    : [...selectedValues, optionValue];
  filter.onChange(next);
}

export function MobileFilterPanel({
  isOpen,
  onClose,
  filters,
  onApply,
  onReset,
}: Readonly<MobileFilterPanelProps>) {
  // Escape key closes the panel — only attached while open so we don't pay
  // the listener cost on every page that mounts a panel.
  useEffect(() => {
    if (!isOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [isOpen, onClose]);

  if (!isOpen) {
    return null;
  }

  const renderFilterOptions = (filter: FilterConfig) => {
    const isMulti = !!filter.multiSelect;
    const selectedValues = normalizeFilterValues(filter.value);
    switch (filter.type) {
      case "buttons":
        return (
          <div className="flex flex-wrap gap-1.5">
            {filter.options.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() =>
                  isMulti
                    ? handleMultiSelectClick(
                        filter,
                        option.value,
                        selectedValues,
                      )
                    : handleSingleSelectClick(filter, option.value)
                }
                className={`rounded-lg px-3 py-2 text-sm font-medium transition-all ${
                  selectedValues.includes(option.value)
                    ? "bg-gray-900 text-white"
                    : "bg-gray-50 text-gray-600 hover:bg-gray-100"
                } `}
              >
                {option.label}
              </button>
            ))}
          </div>
        );

      case "grid":
        return (
          <div className="grid grid-cols-2 gap-2">
            {filter.options.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() =>
                  isMulti
                    ? handleMultiSelectClick(
                        filter,
                        option.value,
                        selectedValues,
                      )
                    : handleSingleSelectClick(filter, option.value)
                }
                className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition-all ${
                  selectedValues.includes(option.value)
                    ? "bg-gray-900 text-white"
                    : "bg-gray-50 text-gray-600 hover:bg-gray-100"
                } `}
              >
                {option.icon && (
                  <svg
                    className="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d={option.icon}
                    />
                  </svg>
                )}
                {option.label}
              </button>
            ))}
          </div>
        );

      case "dropdown":
        return (
          <div>
            {isMulti ? (
              <div className="flex flex-wrap gap-1.5">
                {filter.options.map((option) => (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() =>
                      handleMultiSelectClick(
                        filter,
                        option.value,
                        selectedValues,
                      )
                    }
                    className={`rounded-lg px-3 py-2 text-sm font-medium transition-all ${
                      selectedValues.includes(option.value)
                        ? "bg-gray-900 text-white"
                        : "bg-gray-50 text-gray-600 hover:bg-gray-100"
                    } `}
                  >
                    {option.label}
                    {option.count !== undefined && (
                      <span
                        className={`ml-2 text-xs ${selectedValues.includes(option.value) ? "text-gray-300" : "text-gray-500"}`}
                      >
                        ({option.count})
                      </span>
                    )}
                  </button>
                ))}
              </div>
            ) : (
              <select
                id={`mobile-filter-${filter.id}`}
                data-testid={`filter-${filter.id}`}
                value={filter.value as string}
                onChange={(event) =>
                  handleSingleSelectClick(filter, event.target.value)
                }
                className="h-10 w-full rounded-lg border border-gray-200 bg-gray-50 px-3 text-sm font-medium text-gray-900"
              >
                {filter.options.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.count !== undefined
                      ? `${option.label} (${option.count})`
                      : option.label}
                  </option>
                ))}
              </select>
            )}
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <>
      {/* Click-outside backdrop. Transparent so the page content stays
          visible behind it — Stripe / Linear pattern. The panel itself
          dominates focus via shadow + border. */}
      <button
        type="button"
        onClick={onClose}
        aria-label="Filter schließen"
        className="fixed inset-0 z-40 cursor-default bg-transparent"
      />

      {/* Panel.
          - Mobile (<md): bottom-anchored sheet with side margins.
          - Tablet+ (md+): top-right anchored popover, ~400px wide. Sits
            below the header where the trigger lives, with a comfortable
            inner scroll region so long filter lists don't blow up the
            viewport. */}
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Filter"
        className="fixed inset-x-3 bottom-3 z-50 max-h-[85vh] overflow-y-auto rounded-2xl border border-gray-200 bg-white p-4 shadow-xl md:inset-x-auto md:top-[6rem] md:right-4 md:bottom-auto md:max-h-[80vh] md:w-96"
      >
        <div className="space-y-4">
          {filters.map((filter) => (
            <div key={filter.id}>
              <label
                htmlFor={`mobile-filter-${filter.id}`}
                className="mb-1.5 block text-sm font-semibold text-gray-800"
              >
                {filter.label}
              </label>
              {renderFilterOptions(filter)}
            </div>
          ))}
        </div>

        {(onApply ?? onReset) && (
          <div className="mt-4 flex gap-2 border-t border-gray-100 pt-3">
            {onReset && (
              <button
                type="button"
                onClick={onReset}
                className="flex-1 py-2 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
              >
                Zurücksetzen
              </button>
            )}
            {onApply && (
              <button
                type="button"
                onClick={() => {
                  onApply();
                  onClose();
                }}
                className="flex-1 rounded-lg bg-gray-900 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
              >
                Anwenden
              </button>
            )}
          </div>
        )}
      </div>
    </>
  );
}
