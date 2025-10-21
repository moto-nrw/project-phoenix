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
  statusIndicator,
  tabs,
  search,
  filters = [],
  activeFilters = [],
  onClearAllFilters,
  actionButton,
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
      {/* Title + Badge (only when title exists) */}
      {title && <PageHeader title={title} badge={badge} statusIndicator={statusIndicator} />}

      {/* Tabs + Badge inline (when no title - cleaner layout) */}
      {tabs && (
        <div className="mb-4">
          {/* Mobile & Desktop: Modern underline tabs with badge on the right */}
          <div className="flex items-end justify-between gap-2 md:gap-4">
            <NavigationTabs {...tabs} className="min-w-0 flex-1" />

            {/* Badge and Status inline with tabs - aligned and indented */}
            {!title && (statusIndicator ?? badge) && (
              <div className="flex items-center gap-2 md:gap-3 pb-3 mr-2 md:mr-4 flex-shrink-0">
                {statusIndicator && (
                  <div
                    className={`h-2.5 w-2.5 rounded-full flex-shrink-0 ${
                      statusIndicator.color === 'green' ? 'bg-green-500 animate-pulse' :
                      statusIndicator.color === 'yellow' ? 'bg-yellow-500' :
                      statusIndicator.color === 'red' ? 'bg-red-500' :
                      'bg-gray-400'
                    }`}
                    title={statusIndicator.tooltip}
                  />
                )}
                {badge && (
                  <div className="flex items-center gap-1.5 md:gap-2 px-2 md:px-3 py-1.5 bg-gray-50 rounded-full border border-gray-100">
                    {badge.icon && <span className="text-gray-500">{badge.icon}</span>}
                    <span className="text-sm font-semibold text-gray-900">{badge.count}</span>
                    {badge.label && <span className="text-xs text-gray-500 hidden md:inline">{badge.label}</span>}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}

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
        {(search !== undefined || filters.length > 0 || actionButton) && (
          <div className="flex items-center gap-3 mb-3">
            {search && (
              <SearchBar
                {...search}
                className={filters.length > 0 || actionButton ? "w-64 lg:w-96" : "flex-1"}
                size="md"
              />
            )}
            {filters.length > 0 && (
              <DesktopFilters filters={filters} />
            )}
            {/* Custom action button - pushed to right edge */}
            {actionButton && <div className="ml-auto">{actionButton}</div>}
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