"use client";

import React from "react";
import { RemovableChip } from "~/components/ui/removable-chip";
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
          <RemovableChip
            key={filter.id}
            label={filter.label}
            onRemove={filter.onRemove}
            removeLabel={`Filter ${filter.label} entfernen`}
          />
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
