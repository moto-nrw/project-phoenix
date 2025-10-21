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
        <div>
          {/* Mobile: Underline tabs inline with badge */}
          <div className="md:hidden flex items-center justify-between gap-3 mb-4 border-b border-gray-200">
            <div className="flex gap-1 -mb-px">
              <NavigationTabs {...tabs} className="mb-0" />
            </div>

            {/* Badge and Status inline with tabs */}
            {!title && (statusIndicator ?? badge) && (
              <div className="flex items-center gap-2 pb-3">
                {statusIndicator && (
                  <div
                    className={`h-2 w-2 rounded-full ${
                      statusIndicator.color === 'green' ? 'bg-green-500 animate-pulse' :
                      statusIndicator.color === 'yellow' ? 'bg-yellow-500' :
                      statusIndicator.color === 'red' ? 'bg-red-500' :
                      'bg-gray-400'
                    }`}
                    title={statusIndicator.tooltip}
                  />
                )}
                {badge && (
                  <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-100 rounded-full">
                    {badge.icon && <span className="text-gray-600">{badge.icon}</span>}
                    <span className="text-sm font-medium text-gray-700">{badge.count}</span>
                    {badge.label && <span className="text-sm text-gray-600">{badge.label}</span>}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Desktop: Settings-style tabs (separate from badge) */}
          <div className="hidden md:block mb-4">
            <NavigationTabs {...tabs} className="mb-0" />
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
            {/* Custom action button */}
            {actionButton}
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