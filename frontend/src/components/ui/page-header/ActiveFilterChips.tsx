"use client";

import React from "react";
import type { ActiveFilterChipsProps } from "./types";

export function ActiveFilterChips({
  filters,
  onClearAll,
  className = "",
}: Readonly<ActiveFilterChipsProps>) {
  if (filters.length === 0) {
    return null;
  }

  return (
    <div className={`flex items-center justify-between ${className}`}>
      <div className="flex flex-wrap gap-2">
        {filters.map((filter) => (
          <span
            key={filter.id}
            className="bg-moto-blue-soft text-moto-blue-strong inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-medium"
          >
            {filter.label}
            <button
              type="button"
              aria-label={`Filter ${filter.label} entfernen`}
              onClick={filter.onRemove}
              className="hover:text-moto-blue-strong transition-colors"
            >
              <svg
                className="h-3 w-3"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </span>
        ))}
      </div>

      {onClearAll && (
        <button
          type="button"
          onClick={onClearAll}
          className="text-moto-blue-strong hover:text-moto-blue-hover text-xs font-medium transition-colors"
        >
          Alle löschen
        </button>
      )}
    </div>
  );
}
