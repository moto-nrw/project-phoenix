"use client";

import React from "react";
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
  if (!isOpen) {
    return null;
  }

  const renderFilterOptions = (filter: FilterConfig) => {
    const isMulti = !!filter.multiSelect;
    const selectedValues = normalizeFilterValues(filter.value);
    switch (filter.type) {
      case "buttons":
        return (
          <div className="grid grid-cols-5 gap-1.5">
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
          <div className="space-y-1">
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
                className={`w-full rounded-lg px-3 py-2 text-left text-sm font-medium transition-all ${
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
        );

      default:
        return null;
    }
  };

  return (
    <div className="mb-3 rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
      <div className="space-y-4">
        {filters.map((filter) => (
          <div key={filter.id}>
            <label className="mb-1.5 block text-xs font-medium text-gray-600">
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
  );
}
