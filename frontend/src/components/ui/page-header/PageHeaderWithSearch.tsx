// PageHeaderWithSearch - refactored with extracted sub-components
"use client";

import React, { useState, useMemo } from "react";
import { useScrollY } from "~/lib/hooks/use-scroll-y";
import { PageHeader } from "./PageHeader";
import { SearchBar } from "./SearchBar";
import { DesktopFilters } from "./DesktopFilters";
import { MobileFilterButton } from "./MobileFilterButton";
import { MobileFilterPanel } from "./MobileFilterPanel";
import { ActiveFilterChips } from "./ActiveFilterChips";
import { NavigationTabs } from "./NavigationTabs";
import { OverflowMenu } from "./OverflowMenu";
import { DesktopTabsActionArea, MobileTabsActionArea } from "./TabsActionArea";
import {
  InlineStatusBadge,
  DesktopSearchAction,
  shouldShowInlineStatusBadge,
} from "./SearchRowHelpers";
import type { PageHeaderWithSearchProps } from "./types";

// Threshold (in px) past which compactOnScroll engages. Chosen empirically:
// just enough that a small touch-bounce or address-bar wobble doesn't trigger
// the compact state, but small enough that it kicks in immediately on real
// scroll.
const COMPACT_SCROLL_THRESHOLD = 40;

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
  overflowMenu,
  primaryAction,
  compactOnScroll = false,
  activeFilterDisplay = "chips",
  desktopFiltersFrom = "lg",
  className = "",
}: Readonly<PageHeaderWithSearchProps>) {
  const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);

  // Only subscribe to scroll updates when the consumer opted in. The hook
  // itself is rAF-throttled, but no point paying for it on the 28 pages that
  // don't use compactOnScroll.
  const scrollY = useScrollY();
  const isScrolled = compactOnScroll && scrollY > COMPACT_SCROLL_THRESHOLD;

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
  // Active-count is what we surface in count-mode; falls back to 0 cleanly
  // when activeFilters is empty.
  const activeFilterCount = activeFilters.length;
  const showFilterCountBadge =
    activeFilterDisplay === "count" && activeFilterCount > 0;
  const showChipsRow =
    activeFilterDisplay === "chips" && activeFilters.length > 0;

  return (
    <div className={className}>
      {/* Title + Badge + Mobile Action Button (only when title exists) */}
      {title && (
        <PageHeader
          title={title}
          badge={badge}
          statusIndicator={statusIndicator}
          actionButton={mobileActionButton ?? actionButton}
          overflowMenu={overflowMenu}
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
          overflowMenu={overflowMenu}
        />
      )}

      {/* Mobile/Tablet Search & Filters. Hidden at the breakpoint where
          the desktop inline-filter layout takes over. Tailwind needs both
          `lg:hidden` and `xl:hidden` literally present in the source for
          the JIT scanner to include them — hence the static map. */}
      <MobileSearchSection
        search={search}
        filters={filters}
        hasFilters={hasFilters}
        hasActiveFilters={hasActiveFilters}
        isMobileFiltersOpen={isMobileFiltersOpen}
        setIsMobileFiltersOpen={setIsMobileFiltersOpen}
        hasTabs={hasTabs}
        hasTitle={hasTitle}
        mobileActionButton={mobileActionButton ?? actionButton}
        statusIndicator={statusIndicator}
        badge={badge}
        activeFilters={activeFilters}
        onClearAllFilters={onClearAllFilters}
        isScrolled={isScrolled}
        compactOnScroll={compactOnScroll}
        activeFilterCountForBadge={
          showFilterCountBadge ? activeFilterCount : undefined
        }
        showChipsRow={showChipsRow}
        hideClass={desktopFiltersFrom === "xl" ? "xl:hidden" : "lg:hidden"}
      />

      {/* Desktop Search & Filters. Default kicks in at `lg` (1024px); pages
          with many filters can opt into `xl` (1280px) so iPad-class
          viewports use the sheet pattern instead of overflowing inline. */}
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
        // Render the kebab in the desktop search row only when there are no
        // tabs — when tabs exist the menu already lives in TabsSection.
        overflowMenu={hasTabs ? undefined : overflowMenu}
        primaryAction={primaryAction}
        isScrolled={isScrolled}
        compactOnScroll={compactOnScroll}
        activeFilterCountForBadge={
          showFilterCountBadge ? activeFilterCount : undefined
        }
        showChipsRow={showChipsRow}
        showClass={
          desktopFiltersFrom === "xl" ? "hidden xl:block" : "hidden lg:block"
        }
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
  readonly overflowMenu?: PageHeaderWithSearchProps["overflowMenu"];
}

function TabsSection({
  tabs,
  hasTitle,
  actionButton,
  mobileActionButton,
  statusIndicator,
  badge,
  overflowMenu,
}: TabsSectionProps) {
  const hasOverflowMenu = overflowMenu !== undefined && overflowMenu.length > 0;

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

        {hasOverflowMenu ? (
          <div className="flex flex-shrink-0 items-center pb-1 md:pb-3">
            <OverflowMenu items={overflowMenu} />
          </div>
        ) : null}
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
  readonly isScrolled: boolean;
  readonly compactOnScroll: boolean;
  /** When set, suppresses the chips row and renders this count on the filter pill. */
  readonly activeFilterCountForBadge?: number;
  readonly showChipsRow: boolean;
  /** Tailwind class that hides this section at the desktop breakpoint. */
  readonly hideClass: "lg:hidden" | "xl:hidden";
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
  isScrolled,
  compactOnScroll,
  activeFilterCountForBadge,
  showChipsRow,
  hideClass,
}: MobileSearchSectionProps) {
  const showInlineStatusBadge = shouldShowInlineStatusBadge(
    hasTabs,
    hasTitle,
    Boolean(mobileActionButton),
    statusIndicator,
    badge,
  );

  // Compact-on-scroll: shrink the search row's vertical footprint and add a
  // backdrop-blur background so it visually detaches from the page below.
  // We use transform instead of a height change so layout doesn't shift.
  const compactWrapper = compactOnScroll
    ? `transition-[transform,backdrop-filter,background-color] duration-150 ease-out motion-reduce:transition-none ${
        isScrolled
          ? "scale-[0.97] backdrop-blur-md bg-white/85 -mx-1 px-1 rounded-2xl"
          : ""
      }`
    : "";

  return (
    <div className={hideClass}>
      {search && (
        <div
          className={`mb-3 flex items-center gap-2 ${compactWrapper}`}
          style={{ transformOrigin: "top right" }}
        >
          <SearchBar {...search} className="min-w-0 flex-1" size="sm" />

          {hasFilters && (
            <MobileFilterButton
              isOpen={isMobileFiltersOpen}
              onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
              hasActiveFilters={hasActiveFilters}
              activeCount={activeFilterCountForBadge}
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

      {/* Active Filters (Mobile) — suppressed entirely when the consumer opted
          into count-display; the count badge on MobileFilterButton replaces it. */}
      {showChipsRow && activeFilters.length > 0 && (
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
  readonly overflowMenu?: PageHeaderWithSearchProps["overflowMenu"];
  readonly primaryAction?: React.ReactNode;
  readonly isScrolled: boolean;
  readonly compactOnScroll: boolean;
  readonly activeFilterCountForBadge?: number;
  readonly showChipsRow: boolean;
  /** Tailwind class that shows this section at the desktop breakpoint. */
  readonly showClass: "hidden lg:block" | "hidden xl:block";
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
  overflowMenu,
  primaryAction,
  isScrolled,
  compactOnScroll,
  activeFilterCountForBadge,
  showChipsRow,
  showClass,
}: DesktopSearchSectionProps) {
  const hasOverflowMenu = overflowMenu !== undefined && overflowMenu.length > 0;
  const hasPrimaryAction = primaryAction != null;

  // True when DesktopSearchAction will render something — the kebab
  // follows it as a tight cluster (no own `ml-auto` needed). False when
  // only the kebab needs anchoring at the far right.
  const hasInlineRightAnchor =
    Boolean(!hasTabs && actionButton) ||
    shouldShowInlineStatusBadge(
      hasTabs,
      hasTitle,
      Boolean(actionButton),
      statusIndicator,
      badge,
    );

  const hasActionContent = hasInlineRightAnchor || hasOverflowMenu;

  const showSearchRow =
    search !== undefined || hasFilters || hasActionContent || hasPrimaryAction;

  if (!showSearchRow && activeFilters.length === 0) {
    return null;
  }

  const compactWrapper = compactOnScroll
    ? `transition-[transform,backdrop-filter,background-color] duration-150 ease-out motion-reduce:transition-none ${
        isScrolled
          ? "scale-[0.98] backdrop-blur-md bg-white/85 -mx-2 px-2 rounded-2xl"
          : ""
      }`
    : "";

  // Two-row layout when a primaryAction is set: row 1 = search +
  // primaryAction (right), row 2 = filters + actionButton + kebab (right).
  // Single-row layout otherwise (rückwärtskompatibel for the 28 other
  // consumers).
  if (hasPrimaryAction) {
    return (
      <div className={`mb-6 ${showClass}`}>
        {/* Row 1: search left, primary action right */}
        {(search !== undefined || hasPrimaryAction) && (
          <div
            className={`mb-3 flex items-center gap-3 ${compactWrapper}`}
            style={{ transformOrigin: "top right" }}
          >
            {search && (
              <SearchBar {...search} className="min-w-48 flex-1" size="md" />
            )}
            <div className="ml-auto flex flex-shrink-0 items-center gap-2">
              {primaryAction}
            </div>
          </div>
        )}

        {/* Row 2: filters left, action + kebab right */}
        {(hasFilters || hasActionContent) && (
          <div className="mb-3 flex items-center gap-3">
            {hasFilters && <DesktopFilters filters={filters} />}

            {activeFilterCountForBadge !== undefined &&
            activeFilterCountForBadge > 0 ? (
              <span
                className="inline-flex h-6 items-center gap-1.5 rounded-full bg-blue-50 px-2.5 text-xs font-semibold text-blue-700 ring-1 ring-blue-200 ring-inset"
                aria-label={`${activeFilterCountForBadge} Filter aktiv`}
              >
                <span className="tabular-nums">
                  {activeFilterCountForBadge}
                </span>
                <span>aktiv</span>
              </span>
            ) : null}

            <DesktopSearchAction
              hasTabs={hasTabs}
              hasTitle={hasTitle}
              actionButton={actionButton}
              statusIndicator={statusIndicator}
              badge={badge}
            />

            {hasOverflowMenu ? (
              <div className={hasInlineRightAnchor ? "" : "ml-auto"}>
                <OverflowMenu items={overflowMenu} />
              </div>
            ) : null}
          </div>
        )}

        {showChipsRow && activeFilters.length > 0 && (
          <ActiveFilterChips
            filters={activeFilters}
            onClearAll={onClearAllFilters}
          />
        )}
      </div>
    );
  }

  // Single-row layout (default, unchanged for existing consumers).
  const searchBarClass =
    hasFilters || hasActionContent ? "min-w-48 max-w-96 flex-1" : "flex-1";

  return (
    <div className={`mb-6 ${showClass}`}>
      {showSearchRow && (
        <div
          className={`mb-3 flex items-center gap-3 ${compactWrapper}`}
          style={{ transformOrigin: "top right" }}
        >
          {search && (
            <SearchBar {...search} className={searchBarClass} size="md" />
          )}

          {hasFilters && <DesktopFilters filters={filters} />}

          {activeFilterCountForBadge !== undefined &&
          activeFilterCountForBadge > 0 ? (
            <span
              className="inline-flex h-6 items-center gap-1.5 rounded-full bg-blue-50 px-2.5 text-xs font-semibold text-blue-700 ring-1 ring-blue-200 ring-inset"
              aria-label={`${activeFilterCountForBadge} Filter aktiv`}
            >
              <span className="tabular-nums">{activeFilterCountForBadge}</span>
              <span>aktiv</span>
            </span>
          ) : null}

          <DesktopSearchAction
            hasTabs={hasTabs}
            hasTitle={hasTitle}
            actionButton={actionButton}
            statusIndicator={statusIndicator}
            badge={badge}
          />

          {hasOverflowMenu ? (
            <div className={hasInlineRightAnchor ? "" : "ml-auto"}>
              <OverflowMenu items={overflowMenu} />
            </div>
          ) : null}
        </div>
      )}

      {showChipsRow && activeFilters.length > 0 && (
        <ActiveFilterChips
          filters={activeFilters}
          onClearAll={onClearAllFilters}
        />
      )}
    </div>
  );
}
