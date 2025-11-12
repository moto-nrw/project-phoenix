"use client";

import React from "react";
import type { ActiveFilterChipsProps } from "./types";

export function ActiveFilterChips({
  filters,
  onClearAll,
  className = "",
}: ActiveFilterChipsProps) {
  if (filters.length === 0) {
    return null;
  }

  return (
    <div className={`flex items-center justify-between ${className}`}>
      <div className="flex flex-wrap gap-2">
        {filters.map((filter) => (
          <span
            key={filter.id}
            className="inline-flex items-center gap-1 rounded-full bg-blue-100 px-3 py-1 text-xs font-medium text-blue-700"
          >
            {filter.label}
            <button
              type="button"
              onClick={filter.onRemove}
              className="transition-colors hover:text-blue-900"
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

      {onClearAll && filters.length > 1 && (
        <button
          type="button"
          onClick={onClearAll}
          className="text-xs font-medium text-blue-600 transition-colors hover:text-blue-700"
        >
          Alle löschen
        </button>
      )}
    </div>
  );
}
