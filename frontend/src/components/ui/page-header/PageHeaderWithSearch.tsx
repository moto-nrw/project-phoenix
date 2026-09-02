// PageHeaderWithSearch - refactored with extracted sub-components
"use client";

import React, {
  useState,
  useMemo,
  useCallback,
  useEffect,
  useRef,
} from "react";
import { useScrollY } from "~/lib/hooks/use-scroll-y";
import { useViewportAtLeast } from "~/lib/hooks/use-viewport-at-least";
import { PageHeader } from "./PageHeader";
import { SearchBar } from "./SearchBar";
import { DesktopFilters } from "./DesktopFilters";
import { FilterButton } from "./FilterButton";
import { FilterPanel } from "./FilterPanel";
import { ActiveFilterChips } from "./ActiveFilterChips";
import { NavigationTabs } from "./NavigationTabs";
import { OverflowMenu } from "./OverflowMenu";
import { DesktopTabsActionArea, MobileTabsActionArea } from "./TabsActionArea";
import {
  InlineStatusBadge,
  DesktopSearchAction,
  shouldShowInlineStatusBadge,
} from "./SearchRowHelpers";
import type { FilterPanelAnchor, PageHeaderWithSearchProps } from "./types";

// Threshold (in px) past which compactOnScroll engages. Chosen empirically:
// just enough that a small touch-bounce or address-bar wobble doesn't trigger
// the compact state, but small enough that it kicks in immediately on real
// scroll.
const COMPACT_SCROLL_THRESHOLD = 40;
const EMPTY_FILTERS: NonNullable<PageHeaderWithSearchProps["filters"]> = [];
const EMPTY_ACTIVE_FILTERS: NonNullable<
  PageHeaderWithSearchProps["activeFilters"]
> = [];

export function PageHeaderWithSearch({
  title,
  concept,
  badge,
  statusIndicator,
  tabs,
  search,
  filters = EMPTY_FILTERS,
  activeFilters = EMPTY_ACTIVE_FILTERS,
  onClearAllFilters,
  actionButton,
  mobileActionButton,
  tabsRowAction,
  overflowMenu,
  primaryAction,
  compactOnScroll = false,
  activeFilterDisplay = "chips",
  filterVariant = "default",
  filterSections,
  desktopFiltersFrom = "lg",
  className = "",
}: Readonly<PageHeaderWithSearchProps>) {
  const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);
  const [isDesktopFiltersOpen, setIsDesktopFiltersOpen] = useState(false);
  const isDesktopFilterLayout = useViewportAtLeast(
    desktopFiltersFrom === "xl" ? 1280 : 1024,
  );
  const previousDesktopFilterLayoutRef = useRef(isDesktopFilterLayout);

  // Only subscribe to scroll updates when the consumer opted in. The hook
  // itself is rAF-throttled, but no point paying for it on the 28 pages that
  // don't use compactOnScroll.
  const scrollY = useScrollY();
  const isScrolled = compactOnScroll && scrollY > COMPACT_SCROLL_THRESHOLD;

  // Check if any filters are active (not in their default state). Custom
  // filters are skipped: their `value`/`options` are unused stubs (see
  // FilterConfig.render), so the first-option comparison below would read an
  // empty string against `undefined` and mark them active in every default
  // state. A custom control owns its own rendering and reports its state
  // through `activeFilters` like any other filter.
  const hasActiveFilters = useMemo(() => {
    return filters.some((filter) => {
      if (filter.type === "custom") return false;
      // A multi-select reports its value as an array and treats the empty
      // selection as its default ("alle"), so the first-option comparison
      // below would mark it active in every default state — an array is never
      // equal to an option string.
      if (Array.isArray(filter.value)) return filter.value.length > 0;
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

  useEffect(() => {
    const wasDesktopFilterLayout = previousDesktopFilterLayoutRef.current;
    if (wasDesktopFilterLayout === isDesktopFilterLayout) return;
    previousDesktopFilterLayoutRef.current = isDesktopFilterLayout;

    if (isDesktopFilterLayout) {
      const shouldTransferToDesktopPopover =
        isMobileFiltersOpen && filterVariant === "quiet";
      setIsMobileFiltersOpen(false);
      setIsDesktopFiltersOpen(shouldTransferToDesktopPopover);
      return;
    }

    const shouldTransferToMobile = isDesktopFiltersOpen;
    setIsDesktopFiltersOpen(false);
    if (shouldTransferToMobile) setIsMobileFiltersOpen(true);
  }, [
    filterVariant,
    isDesktopFilterLayout,
    isDesktopFiltersOpen,
    isMobileFiltersOpen,
  ]);

  return (
    <div className={className}>
      {/* Title + Badge + Mobile Action Button (only when title exists) */}
      {title && (
        <PageHeader
          title={title}
          concept={concept}
          badge={badge}
          statusIndicator={statusIndicator}
          actionButton={mobileActionButton ?? actionButton}
          overflowMenu={overflowMenu}
        />
      )}

      {/* Tabs + Badge/ActionButton inline (when tabs exist) */}
      {(tabs ?? tabsRowAction) && (
        <TabsSection
          tabs={tabs}
          hasTitle={hasTitle}
          actionButton={actionButton}
          mobileActionButton={mobileActionButton}
          tabsRowAction={tabsRowAction}
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
        filterVariant={filterVariant}
        filterSections={filterSections}
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
        isDesktopFiltersOpen={isDesktopFiltersOpen}
        setIsDesktopFiltersOpen={setIsDesktopFiltersOpen}
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
        filterVariant={filterVariant}
        filterSections={filterSections}
        showClass={
          desktopFiltersFrom === "xl" ? "hidden xl:block" : "hidden lg:block"
        }
      />
    </div>
  );
}

// --- Sub-components for section organization ---

function useFilterPanelAnchor(
  isOpen: boolean,
  setIsOpen: (open: boolean) => void,
) {
  const anchorRef = useRef<HTMLDivElement>(null);
  const [anchor, setAnchor] = useState<FilterPanelAnchor | null>(null);

  // Snapshot the trigger position. This only seeds the panel's first paint;
  // once open, the panel tracks the live `anchorRef` element every frame and
  // re-positions itself imperatively (no React re-render), so we don't poll
  // here.
  const updateAnchor = useCallback(() => {
    const rect = anchorRef.current?.getBoundingClientRect();
    setAnchor(
      rect
        ? {
            left: Math.round(rect.left),
            bottom: Math.round(rect.bottom),
            right: Math.round(rect.right),
          }
        : null,
    );
  }, []);

  const toggle = useCallback(() => {
    if (!isOpen) updateAnchor();
    setIsOpen(!isOpen);
  }, [isOpen, setIsOpen, updateAnchor]);

  useEffect(() => {
    if (isOpen) updateAnchor();
  }, [isOpen, updateAnchor]);

  return { anchorRef, anchor, toggle };
}

interface TabsSectionProps {
  readonly tabs?: PageHeaderWithSearchProps["tabs"];
  readonly hasTitle: boolean;
  readonly actionButton?: React.ReactNode;
  readonly mobileActionButton?: React.ReactNode;
  readonly tabsRowAction?: React.ReactNode;
  readonly statusIndicator?: PageHeaderWithSearchProps["statusIndicator"];
  readonly badge?: PageHeaderWithSearchProps["badge"];
  readonly overflowMenu?: PageHeaderWithSearchProps["overflowMenu"];
}

function TabsSection({
  tabs,
  hasTitle,
  actionButton,
  mobileActionButton,
  tabsRowAction,
  statusIndicator,
  badge,
  overflowMenu,
}: TabsSectionProps) {
  const hasOverflowMenu = overflowMenu !== undefined && overflowMenu.length > 0;

  return (
    <div className="mt-4 mb-4 md:mt-0">
      <div className="flex flex-wrap items-center justify-between gap-2 md:flex-nowrap md:items-end md:gap-4">
        {tabs ? (
          <NavigationTabs {...tabs} className="min-w-0 flex-[1_1_10rem]" />
        ) : null}

        {tabsRowAction ? (
          <div className="flex flex-shrink-0 items-center pb-1 md:pb-3">
            {tabsRowAction}
          </div>
        ) : null}

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
  readonly filterVariant: NonNullable<
    PageHeaderWithSearchProps["filterVariant"]
  >;
  readonly filterSections?: PageHeaderWithSearchProps["filterSections"];
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
  filterVariant,
  filterSections,
  hideClass,
}: MobileSearchSectionProps) {
  const {
    anchorRef: filterButtonRef,
    anchor: filterPanelAnchor,
    toggle: toggleMobileFilters,
  } = useFilterPanelAnchor(isMobileFiltersOpen, setIsMobileFiltersOpen);

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
            <div ref={filterButtonRef} className="flex-shrink-0">
              <FilterButton
                isOpen={isMobileFiltersOpen}
                onClick={toggleMobileFilters}
                hasActiveFilters={hasActiveFilters}
                activeCount={activeFilterCountForBadge}
                testId="mobile-filter-button"
              />
            </div>
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
        <FilterPanel
          isOpen={isMobileFiltersOpen}
          onClose={() => setIsMobileFiltersOpen(false)}
          filters={filters}
          onApply={() => setIsMobileFiltersOpen(false)}
          onReset={onClearAllFilters}
          anchorRect={filterPanelAnchor}
          anchorRef={filterButtonRef}
          variant={filterVariant}
          sections={filterSections}
          testId="mobile-filter-panel"
        />
      )}

      {/* Active Filters (Mobile) — suppressed entirely when the consumer opted
          into count-display; the count badge on FilterButton replaces it. */}
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
  readonly isDesktopFiltersOpen: boolean;
  readonly setIsDesktopFiltersOpen: (open: boolean) => void;
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
  readonly filterVariant: NonNullable<
    PageHeaderWithSearchProps["filterVariant"]
  >;
  readonly filterSections?: PageHeaderWithSearchProps["filterSections"];
  /** Tailwind class that shows this section at the desktop breakpoint. */
  readonly showClass: "hidden lg:block" | "hidden xl:block";
}

function DesktopSearchSection({
  search,
  filters,
  hasFilters,
  hasTabs,
  hasTitle,
  isDesktopFiltersOpen,
  setIsDesktopFiltersOpen,
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
  filterVariant,
  filterSections,
  showClass,
}: DesktopSearchSectionProps) {
  const {
    anchorRef: desktopFilterButtonRef,
    anchor: desktopFilterPanelAnchor,
    toggle: toggleDesktopFilters,
  } = useFilterPanelAnchor(isDesktopFiltersOpen, setIsDesktopFiltersOpen);
  const hasOverflowMenu = overflowMenu !== undefined && overflowMenu.length > 0;
  const hasPrimaryAction = primaryAction != null;
  // The quiet variant is the single opt-in for the consolidated popover layout.
  const showFilterPopover = filterVariant === "quiet";

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

  // With a primaryAction the header splits into two rows. On a page whose
  // filters live in the popover (quiet variant) and that has no inline right
  // anchor, the second row would carry nothing but the kebab — a lone icon on
  // its own line reads as a layout accident. Keep it in row 1 next to the
  // primary action there, and drop the empty second row entirely.
  const hasInlineFilters = hasFilters && !showFilterPopover;
  const kebabJoinsPrimaryRow =
    hasPrimaryAction &&
    hasOverflowMenu &&
    !hasInlineRightAnchor &&
    !hasInlineFilters;

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

  const desktopFilterButton =
    hasFilters && showFilterPopover ? (
      <>
        <FilterButton
          isOpen={isDesktopFiltersOpen}
          onClick={toggleDesktopFilters}
          hasActiveFilters={activeFilters.length > 0}
          activeCount={activeFilterCountForBadge}
          testId="desktop-filter-button"
        />
        <FilterPanel
          isOpen={isDesktopFiltersOpen}
          onClose={() => setIsDesktopFiltersOpen(false)}
          filters={filters}
          onApply={() => setIsDesktopFiltersOpen(false)}
          onReset={onClearAllFilters}
          applyLabel="Schließen"
          placement="desktop"
          anchorRect={desktopFilterPanelAnchor}
          anchorRef={desktopFilterButtonRef}
          variant={filterVariant}
          sections={filterSections}
          testId="desktop-filter-panel"
        />
      </>
    ) : null;
  const inlineDesktopFilters =
    hasFilters && !showFilterPopover ? (
      <DesktopFilters filters={filters} />
    ) : null;

  // Popover layout shares the same search-bar + filter-button cluster in both
  // the primaryAction and single-row branches below. The two branches are
  // mutually exclusive returns, so reusing one element (incl. its ref) is safe.
  const popoverSearchCluster = (
    <div
      ref={desktopFilterButtonRef}
      className="flex min-w-0 flex-1 items-center gap-3"
    >
      {search && (
        <SearchBar {...search} className="min-w-48 flex-1" size="md" />
      )}
      {desktopFilterButton}
    </div>
  );

  // Two-row layout when a primaryAction is set: row 1 = search +
  // primaryAction (right), row 2 = filters + actionButton + kebab (right).
  // Single-row layout otherwise (rückwärtskompatibel for the 28 other
  // consumers).
  if (hasPrimaryAction) {
    return (
      <div className={`mb-6 ${showClass}`}>
        {/* Row 1: search + popover filter left, primary action right */}
        {(search !== undefined || desktopFilterButton || hasPrimaryAction) && (
          <div
            className={`mb-3 flex items-center gap-3 ${compactWrapper}`}
            style={{ transformOrigin: "top right" }}
          >
            {showFilterPopover
              ? popoverSearchCluster
              : search && (
                  <SearchBar
                    {...search}
                    className="min-w-48 flex-1"
                    size="md"
                  />
                )}
            <div className="ml-auto flex flex-shrink-0 items-center gap-2">
              {primaryAction}
              {kebabJoinsPrimaryRow ? (
                <OverflowMenu items={overflowMenu} />
              ) : null}
            </div>
          </div>
        )}

        {/* Row 2: filters left, action + kebab right */}
        {(hasInlineFilters || hasInlineRightAnchor) && (
          <div className="mb-3 flex items-center gap-3">
            {inlineDesktopFilters}

            {!showFilterPopover &&
            activeFilterCountForBadge !== undefined &&
            activeFilterCountForBadge > 0 ? (
              <span
                className="bg-moto-blue-soft text-moto-blue-strong ring-moto-blue/20 inline-flex h-6 items-center gap-1.5 rounded-full px-2.5 text-xs font-semibold ring-1 ring-inset"
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

            {hasOverflowMenu && !kebabJoinsPrimaryRow ? (
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
          className={`mb-3 flex flex-wrap items-center gap-3 ${compactWrapper}`}
          style={{ transformOrigin: "top right" }}
        >
          {showFilterPopover ? (
            popoverSearchCluster
          ) : (
            <>
              {search && (
                <SearchBar {...search} className={searchBarClass} size="md" />
              )}
              {inlineDesktopFilters}
            </>
          )}

          {!showFilterPopover &&
          activeFilterCountForBadge !== undefined &&
          activeFilterCountForBadge > 0 ? (
            <span
              className="bg-moto-blue-soft text-moto-blue-strong ring-moto-blue/20 inline-flex h-6 items-center gap-1.5 rounded-full px-2.5 text-xs font-semibold ring-1 ring-inset"
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
