// PageHeaderWithSearch - refactored with extracted sub-components
"use client";

import React, { useState, useMemo } from "react";
import { PageHeader } from "./PageHeader";
import { SearchBar } from "./SearchBar";
import { DesktopFilters } from "./DesktopFilters";
import { MobileFilterButton } from "./MobileFilterButton";
import { MobileFilterPanel } from "./MobileFilterPanel";
import { ActiveFilterChips } from "./ActiveFilterChips";
import { NavigationTabs } from "./NavigationTabs";
import { DesktopTabsActionArea, MobileTabsActionArea } from "./TabsActionArea";
import {
  InlineStatusBadge,
  DesktopSearchAction,
  shouldShowInlineStatusBadge,
} from "./SearchRowHelpers";
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
}: Readonly<PageHeaderWithSearchProps>) {
  const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);

  // Check if any filters are active (not in their default state)
  const hasActiveFilters = useMemo(() => {
    return filters.some((filter) => {
      const defaultValue = filter.options[0]?.value;
      return filter.value !== defaultValue;
    });
  }, [filters]);

  const hasTitle = Boolean(title);
  const hasTabs = Boolean(tabs);
  const hasFilters = filters.length > 0;

  return (
    <div className={className}>
      {/* Title + Badge + Mobile Action Button (only when title exists) */}
      {title && (
        <PageHeader
          title={title}
          badge={badge}
          statusIndicator={statusIndicator}
          actionButton={mobileActionButton}
        />
      )}

      {/* Tabs + Badge/ActionButton inline (when tabs exist) */}
      {tabs && (
        <TabsSection
          tabs={tabs}
          hasTitle={hasTitle}
          actionButton={actionButton}
          mobileActionButton={mobileActionButton}
          statusIndicator={statusIndicator}
          badge={badge}
        />
      )}

      {/* Mobile & Tablet Search & Filters */}
      <MobileSearchSection
        search={search}
        filters={filters}
        hasFilters={hasFilters}
        hasActiveFilters={hasActiveFilters}
        isMobileFiltersOpen={isMobileFiltersOpen}
        setIsMobileFiltersOpen={setIsMobileFiltersOpen}
        hasTabs={hasTabs}
        hasTitle={hasTitle}
        mobileActionButton={mobileActionButton}
        statusIndicator={statusIndicator}
        badge={badge}
        activeFilters={activeFilters}
        onClearAllFilters={onClearAllFilters}
      />

      {/* Desktop Search & Filters */}
      <DesktopSearchSection
        search={search}
        filters={filters}
        hasFilters={hasFilters}
        hasTabs={hasTabs}
        hasTitle={hasTitle}
        actionButton={actionButton}
        statusIndicator={statusIndicator}
        badge={badge}
        activeFilters={activeFilters}
        onClearAllFilters={onClearAllFilters}
      />
    </div>
  );
}

// --- Sub-components for section organization ---

interface TabsSectionProps {
  readonly tabs: NonNullable<PageHeaderWithSearchProps["tabs"]>;
  readonly hasTitle: boolean;
  readonly actionButton?: React.ReactNode;
  readonly mobileActionButton?: React.ReactNode;
  readonly statusIndicator?: PageHeaderWithSearchProps["statusIndicator"];
  readonly badge?: PageHeaderWithSearchProps["badge"];
}

function TabsSection({
  tabs,
  hasTitle,
  actionButton,
  mobileActionButton,
  statusIndicator,
  badge,
}: TabsSectionProps) {
  return (
    <div className="mt-4 mb-4 md:mt-0">
      <div className="flex items-center justify-between gap-2 md:items-end md:gap-4">
        <NavigationTabs {...tabs} className="min-w-0 flex-1" />

        <DesktopTabsActionArea
          hasTitle={hasTitle}
          actionButton={actionButton}
          statusIndicator={statusIndicator}
          badge={badge}
        />

        <MobileTabsActionArea
          hasTitle={hasTitle}
          actionButton={mobileActionButton}
          statusIndicator={statusIndicator}
          badge={badge}
        />
      </div>
    </div>
  );
}

interface MobileSearchSectionProps {
  readonly search?: PageHeaderWithSearchProps["search"];
  readonly filters: NonNullable<PageHeaderWithSearchProps["filters"]>;
  readonly hasFilters: boolean;
  readonly hasActiveFilters: boolean;
  readonly isMobileFiltersOpen: boolean;
  readonly setIsMobileFiltersOpen: (open: boolean) => void;
  readonly hasTabs: boolean;
  readonly hasTitle: boolean;
  readonly mobileActionButton?: React.ReactNode;
  readonly statusIndicator?: PageHeaderWithSearchProps["statusIndicator"];
  readonly badge?: PageHeaderWithSearchProps["badge"];
  readonly activeFilters: NonNullable<
    PageHeaderWithSearchProps["activeFilters"]
  >;
  readonly onClearAllFilters?: () => void;
}

function MobileSearchSection({
  search,
  filters,
  hasFilters,
  hasActiveFilters,
  isMobileFiltersOpen,
  setIsMobileFiltersOpen,
  hasTabs,
  hasTitle,
  mobileActionButton,
  statusIndicator,
  badge,
  activeFilters,
  onClearAllFilters,
}: MobileSearchSectionProps) {
  const showInlineStatusBadge = shouldShowInlineStatusBadge(
    hasTabs,
    hasTitle,
    Boolean(mobileActionButton),
    statusIndicator,
    badge,
  );

  return (
    <div className="lg:hidden">
      {search && (
        <div className="mb-3 flex items-center gap-2">
          <SearchBar {...search} className="min-w-0 flex-1" size="sm" />

          {hasFilters && (
            <MobileFilterButton
              isOpen={isMobileFiltersOpen}
              onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
              hasActiveFilters={hasActiveFilters}
            />
          )}

          {/* Mobile action button when no tabs and no title */}
          {!hasTabs && !hasTitle && mobileActionButton && (
            <div className="flex-shrink-0">{mobileActionButton}</div>
          )}

          {/* Badge/Status inline with search */}
          {showInlineStatusBadge && (
            <InlineStatusBadge
              statusIndicator={statusIndicator}
              badge={badge}
              variant="mobile"
            />
          )}
        </div>
      )}

      {/* Mobile Filter Panel */}
      {hasFilters && (
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
  );
}

interface DesktopSearchSectionProps {
  readonly search?: PageHeaderWithSearchProps["search"];
  readonly filters: NonNullable<PageHeaderWithSearchProps["filters"]>;
  readonly hasFilters: boolean;
  readonly hasTabs: boolean;
  readonly hasTitle: boolean;
  readonly actionButton?: React.ReactNode;
  readonly statusIndicator?: PageHeaderWithSearchProps["statusIndicator"];
  readonly badge?: PageHeaderWithSearchProps["badge"];
  readonly activeFilters: NonNullable<
    PageHeaderWithSearchProps["activeFilters"]
  >;
  readonly onClearAllFilters?: () => void;
}

function DesktopSearchSection({
  search,
  filters,
  hasFilters,
  hasTabs,
  hasTitle,
  actionButton,
  statusIndicator,
  badge,
  activeFilters,
  onClearAllFilters,
}: DesktopSearchSectionProps) {
  const hasActionContent =
    Boolean(!hasTabs && actionButton) ||
    shouldShowInlineStatusBadge(
      hasTabs,
      hasTitle,
      Boolean(actionButton),
      statusIndicator,
      badge,
    );

  const showSearchRow = search !== undefined || hasFilters || hasActionContent;

  if (!showSearchRow && activeFilters.length === 0) {
    return null;
  }

  const searchBarClass =
    hasFilters || hasActionContent ? "min-w-48 max-w-96 flex-1" : "flex-1";

  return (
    <div className="mb-6 hidden lg:block">
      {showSearchRow && (
        <div className="mb-3 flex items-center gap-3">
          {search && (
            <SearchBar {...search} className={searchBarClass} size="md" />
          )}

          {hasFilters && <DesktopFilters filters={filters} />}

          <DesktopSearchAction
            hasTabs={hasTabs}
            hasTitle={hasTitle}
            actionButton={actionButton}
            statusIndicator={statusIndicator}
            badge={badge}
          />
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
  );
}
