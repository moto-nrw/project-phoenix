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
  mobileActionButton,
  className = "",
}: PageHeaderWithSearchProps) {
  const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);

  // Check if any filters are active (not in their default state)
  const hasActiveFilters = useMemo(() => {
    return filters.some((filter) => {
      const defaultValue = filter.options[0]?.value;
      return filter.value !== defaultValue;
    });
  }, [filters]);

  return (
    <div className={className}>
      {/* Title + Badge (only when title exists) */}
      {title && (
        <PageHeader
          title={title}
          badge={badge}
          statusIndicator={statusIndicator}
        />
      )}

      {/* Tabs + Badge/ActionButton inline (when no title - cleaner layout) */}
      {tabs && (
        <div className="mt-4 mb-4 md:mt-0">
          {/* Mobile & Desktop: Modern underline tabs with badge/button on the right */}
          <div className="flex items-end justify-between gap-2 md:gap-4">
            <NavigationTabs {...tabs} className="min-w-0 flex-1" />

            {/* Desktop: Action Button OR Badge/Status inline with tabs */}
            <div className="hidden flex-shrink-0 items-end gap-2 pb-3 md:flex md:gap-3">
              {!title && actionButton ? (
                actionButton
              ) : (
                <>
                  {!title && statusIndicator && (
                    <div
                      className={`h-2.5 w-2.5 flex-shrink-0 rounded-full ${
                        statusIndicator.color === "green"
                          ? "animate-pulse bg-green-500"
                          : statusIndicator.color === "yellow"
                            ? "bg-yellow-500"
                            : statusIndicator.color === "red"
                              ? "bg-red-500"
                              : "bg-gray-400"
                      }`}
                      title={statusIndicator.tooltip}
                    />
                  )}
                  {!title && badge && (
                    <div className="flex items-center gap-1.5 rounded-full border border-gray-100 bg-gray-50 px-2 py-1.5 md:gap-2 md:px-3">
                      {badge.icon && (
                        <span className="text-gray-500">{badge.icon}</span>
                      )}
                      <span className="text-sm font-semibold text-gray-900">
                        {badge.count}
                      </span>
                      {badge.label && (
                        <span className="hidden text-xs text-gray-500 md:inline">
                          {badge.label}
                        </span>
                      )}
                    </div>
                  )}
                </>
              )}
            </div>

            {/* Mobile: Compact Action Button OR Badge/Status */}
            <div className="mr-2 flex flex-shrink-0 items-center gap-2 pb-3 md:hidden md:gap-3">
              {!title && mobileActionButton ? (
                mobileActionButton
              ) : (
                <>
                  {!title && statusIndicator && (
                    <div
                      className={`h-2.5 w-2.5 flex-shrink-0 rounded-full ${
                        statusIndicator.color === "green"
                          ? "animate-pulse bg-green-500"
                          : statusIndicator.color === "yellow"
                            ? "bg-yellow-500"
                            : statusIndicator.color === "red"
                              ? "bg-red-500"
                              : "bg-gray-400"
                      }`}
                      title={statusIndicator.tooltip}
                    />
                  )}
                  {!title && badge && (
                    <div className="flex items-center gap-1.5 rounded-full border border-gray-100 bg-gray-50 px-2 py-1.5">
                      {badge.icon && (
                        <span className="text-gray-500">{badge.icon}</span>
                      )}
                      <span className="text-sm font-semibold text-gray-900">
                        {badge.count}
                      </span>
                    </div>
                  )}
                </>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Mobile Search & Filters */}
      <div className="md:hidden">
        {search && (
          <div className="mb-3 flex gap-2">
            <SearchBar {...search} className="flex-1" size="sm" />
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
      <div className="mb-6 hidden md:block">
        {(search !== undefined || filters.length > 0) && (
          <div className="mb-3 flex items-center gap-3">
            {search && (
              <SearchBar
                {...search}
                className={filters.length > 0 ? "w-64 lg:w-96" : "flex-1"}
                size="md"
              />
            )}
            {filters.length > 0 && <DesktopFilters filters={filters} />}
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
