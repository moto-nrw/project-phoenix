"use client";

import React, { useState, useMemo } from "react";
import { PageHeader } from "./PageHeader";
import { SearchBar } from "./SearchBar";
import { DesktopFilters } from "./DesktopFilters";
import { MobileFilterButton } from "./MobileFilterButton";
import { MobileFilterPanel } from "./MobileFilterPanel";
import { ActiveFilterChips } from "./ActiveFilterChips";
import { NavigationTabs } from "./NavigationTabs";
import type { PageHeaderWithSearchProps } from "./types";

export function PageHeaderWithSearch({
  title,
  badge,
  tabs,
  search,
  filters = [],
  activeFilters = [],
  onClearAllFilters,
  className = ""
}: PageHeaderWithSearchProps) {
  const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);

  // Check if any filters are active (not in their default state)
  const hasActiveFilters = useMemo(() => {
    return filters.some(filter => {
      const defaultValue = filter.options[0]?.value;
      return filter.value !== defaultValue;
    });
  }, [filters]);

  return (
    <div className={className}>
      {/* Page Header */}
      <PageHeader title={title} badge={badge} />

      {/* Navigation Tabs (if provided) */}
      {tabs && <NavigationTabs {...tabs} />}

      {/* Mobile Search & Filters */}
      <div className="md:hidden">
        {search && (
          <div className="flex gap-2 mb-3">
            <SearchBar 
              {...search} 
              className="flex-1" 
              size="sm"
            />
            {filters.length > 0 && (
              <MobileFilterButton
                isOpen={isMobileFiltersOpen}
                onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
                hasActiveFilters={hasActiveFilters}
              />
            )}
          </div>
        )}

        {/* Mobile Filter Panel */}
        {filters.length > 0 && (
          <MobileFilterPanel
            isOpen={isMobileFiltersOpen}
            onClose={() => setIsMobileFiltersOpen(false)}
            filters={filters}
            onApply={() => setIsMobileFiltersOpen(false)}
            onReset={onClearAllFilters}
          />
        )}

        {/* Active Filters (Mobile) */}
        {activeFilters.length > 0 && (
          <ActiveFilterChips
            filters={activeFilters}
            onClearAll={onClearAllFilters}
            className="mb-3"
          />
        )}
      </div>

      {/* Desktop Search & Filters */}
      <div className="hidden md:block mb-6">
        {(search !== undefined || filters.length > 0) && (
          <div className="flex items-center gap-3 mb-3">
            {search && (
              <SearchBar 
                {...search} 
                className="flex-1" 
                size="md"
              />
            )}
            {filters.length > 0 && (
              <DesktopFilters filters={filters} />
            )}
          </div>
        )}

        {/* Active Filters (Desktop) */}
        {activeFilters.length > 0 && (
          <ActiveFilterChips
            filters={activeFilters}
            onClearAll={onClearAllFilters}
          />
        )}
      </div>
    </div>
  );
}