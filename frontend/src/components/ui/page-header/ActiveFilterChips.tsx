"use client";

import React from "react";
import type { ActiveFilterChipsProps } from "./types";

export function ActiveFilterChips({ 
  filters, 
  onClearAll, 
  className = "" 
}: ActiveFilterChipsProps) {
  if (filters.length === 0) {
    return null;
  }

  return (
    <div className={`flex items-center justify-between ${className}`}>
      <div className="flex gap-2 flex-wrap">
        {filters.map((filter) => (
          <span
            key={filter.id}
            className="inline-flex items-center gap-1 px-3 py-1 bg-blue-100 text-blue-700 rounded-full text-xs font-medium"
          >
            {filter.label}
            <button
              type="button"
              onClick={filter.onRemove}
              className="hover:text-blue-900 transition-colors"
            >
              <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
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
          className="text-xs text-blue-600 hover:text-blue-700 font-medium transition-colors"
        >
          Alle löschen
        </button>
      )}
    </div>
  );
}