"use client";

import { useState } from "react";

interface Filter {
  id: string;
  label: string;
  options: { label: string; value: string | null }[];
}

interface DataListFiltersProps {
  filters: Filter[];
  onChange: (filterId: string, value: string | null) => void;
  className?: string;
}

export function DataListFilters({
  filters,
  onChange,
  className = "",
}: DataListFiltersProps) {
  const [activeFilters, setActiveFilters] = useState<
    Record<string, string | null>
  >({});

  const handleFilterChange = (filterId: string, value: string | null) => {
    const newActiveFilters = {
      ...activeFilters,
      [filterId]: value,
    };

    // If value is null or empty, remove this filter
    if (value === null || value === "") {
      delete newActiveFilters[filterId];
    }

    setActiveFilters(newActiveFilters);
    onChange(filterId, value);
  };

  return (
    <div className={`flex flex-wrap items-center gap-3 ${className}`}>
      {filters.map((filter) => (
        <div key={filter.id} className="flex-shrink-0">
          <select
            id={`filter-${filter.id}`}
            value={activeFilters[filter.id] ?? ""}
            onChange={(e) => {
              const value = e.target.value === "" ? null : e.target.value;
              handleFilterChange(filter.id, value);
            }}
            className="rounded-md border border-gray-300 bg-white px-3 py-2 text-sm focus:border-blue-500 focus:ring-2 focus:ring-blue-500 focus:outline-none"
          >
            {filter.options.map((option) => (
              <option key={option.value ?? "empty"} value={option.value ?? ""}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
      ))}
    </div>
  );
}
