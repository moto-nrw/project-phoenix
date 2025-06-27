"use client";

import React from "react";
import type { MobileFilterPanelProps, FilterConfig } from "./types";

export function MobileFilterPanel({
  isOpen,
  onClose,
  filters,
  onApply,
  onReset
}: MobileFilterPanelProps) {
  if (!isOpen) {
    return null;
  }

  const renderFilterOptions = (filter: FilterConfig) => {
    switch (filter.type) {
      case 'buttons':
        return (
          <div className="grid grid-cols-5 gap-1.5">
            {filter.options.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => filter.onChange(option.value)}
                className={`
                  py-2 px-3 rounded-lg text-sm font-medium transition-all
                  ${filter.value === option.value 
                    ? 'bg-gray-900 text-white' 
                    : 'bg-gray-50 text-gray-600 hover:bg-gray-100'
                  }
                `}
              >
                {option.label}
              </button>
            ))}
          </div>
        );

      case 'grid':
        return (
          <div className="grid grid-cols-2 gap-2">
            {filter.options.map((option) => (
              <button
                key={option.value}
                type="button"
                onClick={() => filter.onChange(option.value)}
                className={`
                  flex items-center gap-2 py-2 px-3 rounded-lg text-sm font-medium transition-all
                  ${filter.value === option.value 
                    ? 'bg-gray-900 text-white' 
                    : 'bg-gray-50 text-gray-600 hover:bg-gray-100'
                  }
                `}
              >
                {option.icon && (
                  <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={option.icon} />
                  </svg>
                )}
                {option.label}
              </button>
            ))}
          </div>
        );

      default:
        return null;
    }
  };

  return (
    <div className="bg-white border border-gray-200 rounded-2xl p-4 shadow-sm mb-3">
      <div className="space-y-4">
        {filters.map((filter) => (
          <div key={filter.id}>
            <label className="text-xs font-medium text-gray-600 mb-1.5 block">
              {filter.label}
            </label>
            {renderFilterOptions(filter)}
          </div>
        ))}
      </div>

      {(onApply ?? onReset) && (
        <div className="flex gap-2 mt-4 pt-3 border-t border-gray-100">
          {onReset && (
            <button
              type="button"
              onClick={onReset}
              className="flex-1 py-2 text-sm font-medium text-gray-600 hover:text-gray-900 transition-colors"
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
              className="flex-1 py-2 bg-gray-900 text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors"
            >
              Anwenden
            </button>
          )}
        </div>
      )}
    </div>
  );
}