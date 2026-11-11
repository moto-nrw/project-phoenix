// Die Such- und Filterzeile von `TenantPage`: Suchfeld, Filter, Chips der
// aktiven Filter, Zähler-Plakette, Kebab und Hauptaktion der Liste. Sie steht
// nur in der Kopfkarte (`TenantPage`) und in `TenantPageListSearch`; eine
// Seite übergibt ihre Daten an `TenantPage` und rendert diese Zeile nie
// selbst. Seitentests mocken sie als die Fläche, die genau die Such-Props von
// `TenantPage` bekommt.
"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useViewportAtLeast } from "~/lib/hooks/use-viewport-at-least";
import { ActiveFilterChips } from "./page-header/ActiveFilterChips";
import { BadgeDisplay, BadgeDisplayCompact } from "./page-header/BadgeDisplay";
import { DesktopFilters } from "./page-header/DesktopFilters";
import { FilterButton } from "./page-header/FilterButton";
import { FilterPanel } from "./page-header/FilterPanel";
import { OverflowMenu } from "./page-header/OverflowMenu";
import {
  SearchBarDraftProvider,
  SharedSearchBar,
} from "./page-header/SearchBar";
import type {
  ActiveFilter,
  FilterConfig,
  FilterPanelAnchor,
  FilterSection,
  OverflowMenuItem,
  PageHeaderBadge,
  PageHeaderFilterPanel,
  PageHeaderSearch,
} from "./page-header/types";

/** Die Such- und Filter-Props von `TenantPage`. */
export interface TenantPageSearchRowProps {
  /** Suchfeld in der Kopfkarte. */
  readonly search?: PageHeaderSearch;
  readonly filters?: readonly FilterConfig[];
  readonly activeFilters?: readonly ActiveFilter[];
  readonly onClearAllFilters?: () => void;
  readonly overflowMenu?: readonly OverflowMenuItem[];
  /**
   * Zaehler-Plakette in der Such- und Filterzeile, z. B. die Zahl der
   * gefundenen Kinder. Nur fuer eine Zahl, die sich mit der Filterung aendert
   * — feste Kennzahlen gehoeren in `stats`.
   */
  readonly badge?: PageHeaderBadge;
  /**
   * Siehe {@link PageHeaderFilterPanel}: `filterPanel` allein für eine flache
   * Filterfläche, `filterPanel={{ sections }}` mit Überschriften. Ohne Angabe
   * stehen die Filter in der Zeile.
   */
  readonly filterPanel?: PageHeaderFilterPanel;
  /**
   * Hauptaktion IN der Such- und Filterzeile statt in der Titelzeile. Nur fuer
   * Aktionen, die zur Liste darunter gehoeren (An- und Abmelden). Auf dem
   * Desktop teilt sie die Zeile: oben Suche und Hauptaktion, darunter Filter
   * und Kebab.
   */
  readonly primaryAction?: React.ReactNode;
}

const EMPTY_FILTERS: readonly FilterConfig[] = [];
const EMPTY_ACTIVE_FILTERS: readonly ActiveFilter[] = [];
const NOOP_SEARCH_CHANGE = () => {};

export function TenantPageSearchRow({
  search,
  filters = EMPTY_FILTERS,
  activeFilters = EMPTY_ACTIVE_FILTERS,
  onClearAllFilters,
  overflowMenu,
  badge,
  filterPanel,
  primaryAction,
}: Readonly<TenantPageSearchRowProps>) {
  // Die Filterfläche ist eine Option für sich: Schaltfläche mit Zahl statt
  // Chips, Desktop-Anordnung erst ab `xl`.
  const usesFilterPanel = filterPanel !== undefined;
  const filterSections =
    filterPanel === undefined || filterPanel === true
      ? undefined
      : filterPanel.sections;
  const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);
  const [isDesktopFiltersOpen, setIsDesktopFiltersOpen] = useState(false);
  const isDesktopFilterLayout = useViewportAtLeast(
    usesFilterPanel ? "xl" : "lg",
  );
  const previousDesktopFilterLayoutRef = useRef(isDesktopFilterLayout);

  // Check if any filters are active (not in their default state). Custom
  // filters are skipped: their `value`/`options` are unused stubs (see
  // FilterConfig.render), so the first-option comparison below would read an
  // empty string against `undefined` and mark them active in every default
  // state. A custom control owns its own rendering and reports its state
  // through `activeFilters` like any other filter.
  const hasActiveFilters = useMemo(() => {
    return (
      activeFilters.length > 0 ||
      filters.some((filter) => {
        if (filter.type === "custom") return false;
        // A multi-select reports its value as an array and treats the empty
        // selection as its default ("alle"), so the first-option comparison
        // below would mark it active in every default state — an array is never
        // equal to an option string.
        if (Array.isArray(filter.value)) return filter.value.length > 0;
        const defaultValue = filter.options[0]?.value;
        return filter.value !== defaultValue;
      })
    );
  }, [activeFilters, filters]);

  const activeFilterCount =
    usesFilterPanel && activeFilters.length > 0
      ? activeFilters.length
      : undefined;
  const showChipsRow = !usesFilterPanel && activeFilters.length > 0;

  useEffect(() => {
    const wasDesktopFilterLayout = previousDesktopFilterLayoutRef.current;
    if (wasDesktopFilterLayout === isDesktopFilterLayout) return;
    previousDesktopFilterLayoutRef.current = isDesktopFilterLayout;

    if (isDesktopFilterLayout) {
      const shouldTransferToDesktopPopover =
        isMobileFiltersOpen && usesFilterPanel;
      setIsMobileFiltersOpen(false);
      setIsDesktopFiltersOpen(shouldTransferToDesktopPopover);
      return;
    }

    const shouldTransferToMobile = isDesktopFiltersOpen;
    setIsDesktopFiltersOpen(false);
    if (shouldTransferToMobile) setIsMobileFiltersOpen(true);
  }, [
    usesFilterPanel,
    isDesktopFilterLayout,
    isDesktopFiltersOpen,
    isMobileFiltersOpen,
  ]);

  return (
    <SearchBarDraftProvider
      value={search?.value ?? ""}
      onChange={search?.onChange ?? NOOP_SEARCH_CHANGE}
      debounceMs={search?.debounceMs}
      resetKey={search?.resetKey}
    >
      <div>
        {/* Telefon und Tablet: Suche und Filter-Schaltfläche. Ab der Breite
          der Desktop-Anordnung ausgeblendet. Tailwind braucht `lg:hidden`
          und `xl:hidden` wörtlich im Quelltext. */}
        <MobileSearchSection
          search={search}
          filters={filters}
          hasActiveFilters={hasActiveFilters}
          isMobileFiltersOpen={isMobileFiltersOpen}
          setIsMobileFiltersOpen={setIsMobileFiltersOpen}
          badge={badge}
          activeFilters={activeFilters}
          onClearAllFilters={onClearAllFilters}
          activeFilterCount={activeFilterCount}
          showChipsRow={showChipsRow}
          usesFilterPanel={usesFilterPanel}
          filterSections={filterSections}
          hideClass={usesFilterPanel ? "xl:hidden" : "lg:hidden"}
          overflowMenu={overflowMenu}
        />

        {/* Desktop: Filter in der Zeile ab `lg` (1024 px), die Filterfläche
          erst ab `xl` (1280 px), damit ein iPad sie als Blatt bekommt. */}
        <DesktopSearchSection
          search={search}
          filters={filters}
          isDesktopFiltersOpen={isDesktopFiltersOpen}
          setIsDesktopFiltersOpen={setIsDesktopFiltersOpen}
          badge={badge}
          activeFilters={activeFilters}
          onClearAllFilters={onClearAllFilters}
          overflowMenu={overflowMenu}
          primaryAction={primaryAction}
          activeFilterCount={activeFilterCount}
          showChipsRow={showChipsRow}
          usesFilterPanel={usesFilterPanel}
          filterSections={filterSections}
          showClass={usesFilterPanel ? "hidden xl:block" : "hidden lg:block"}
        />
      </div>
    </SearchBarDraftProvider>
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

/** Zähler-Plakette der Zeile; auf dem Telefon ohne Label. */
function RowBadge({
  badge,
  variant,
}: Readonly<{ badge: PageHeaderBadge; variant: "mobile" | "desktop" }>) {
  if (variant === "mobile") {
    return (
      <div className="flex flex-shrink-0 items-center gap-2">
        <BadgeDisplayCompact count={badge.count} icon={badge.icon} />
      </div>
    );
  }
  return (
    <div className="ml-auto flex flex-shrink-0 items-center gap-3">
      <BadgeDisplay
        count={badge.count}
        label={badge.label}
        icon={badge.icon}
        showLabel={true}
      />
    </div>
  );
}

interface MobileSearchSectionProps {
  readonly search?: PageHeaderSearch;
  readonly filters: readonly FilterConfig[];
  readonly hasActiveFilters: boolean;
  readonly isMobileFiltersOpen: boolean;
  readonly setIsMobileFiltersOpen: (open: boolean) => void;
  readonly badge?: PageHeaderBadge;
  readonly activeFilters: readonly ActiveFilter[];
  readonly onClearAllFilters?: () => void;
  /** Zahl auf der Filter-Schaltfläche; nur mit Filterfläche gesetzt. */
  readonly activeFilterCount?: number;
  readonly showChipsRow: boolean;
  readonly usesFilterPanel: boolean;
  readonly filterSections?: readonly FilterSection[];
  /** Tailwind class that hides this section at the desktop breakpoint. */
  readonly hideClass: "lg:hidden" | "xl:hidden";
  /**
   * Neben der Suche: Die Kopfkarte hat auf dem Telefon keine andere Zeile,
   * die das Kebab trägt. Ohne es gäbe es das Menü erst ab der Desktop-Breite.
   */
  readonly overflowMenu?: readonly OverflowMenuItem[];
}

function MobileSearchSection({
  search,
  filters,
  hasActiveFilters,
  isMobileFiltersOpen,
  setIsMobileFiltersOpen,
  badge,
  activeFilters,
  onClearAllFilters,
  activeFilterCount,
  showChipsRow,
  usesFilterPanel,
  filterSections,
  hideClass,
  overflowMenu,
}: MobileSearchSectionProps) {
  const {
    anchorRef: filterButtonRef,
    anchor: filterPanelAnchor,
    toggle: toggleMobileFilters,
  } = useFilterPanelAnchor(isMobileFiltersOpen, setIsMobileFiltersOpen);
  const hasFilters = filters.length > 0;

  return (
    <div className={`${hideClass} [&>*:last-child]:mb-0`}>
      {/* Auch ohne Suchfeld: sonst gäbe es unter der Desktop-Breite keinen
          Weg zu den Filtern (Zugriffsprotokoll, Ferienanmeldungen). */}
      {(search ?? hasFilters) && (
        <div className="mb-3 flex items-center gap-2">
          {search && (
            <SharedSearchBar {...search} className="min-w-0 flex-1" size="sm" />
          )}

          {hasFilters && (
            <div ref={filterButtonRef} className="flex-shrink-0">
              <FilterButton
                isOpen={isMobileFiltersOpen}
                onClick={toggleMobileFilters}
                hasActiveFilters={hasActiveFilters}
                activeCount={activeFilterCount}
                testId="mobile-filter-button"
              />
            </div>
          )}

          {overflowMenu && overflowMenu.length > 0 && (
            <div className="flex-shrink-0">
              <OverflowMenu items={overflowMenu} />
            </div>
          )}

          {badge && <RowBadge badge={badge} variant="mobile" />}
        </div>
      )}

      {hasFilters && (
        <FilterPanel
          isOpen={isMobileFiltersOpen}
          onClose={() => setIsMobileFiltersOpen(false)}
          filters={filters}
          onApply={() => setIsMobileFiltersOpen(false)}
          onReset={onClearAllFilters}
          anchorRect={filterPanelAnchor}
          anchorRef={filterButtonRef}
          variant={usesFilterPanel ? "quiet" : "default"}
          sections={filterSections}
          testId="mobile-filter-panel"
        />
      )}

      {/* Mit Filterfläche ersetzt die Zahl auf der Schaltfläche die Chips. */}
      {showChipsRow && (
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
  readonly search?: PageHeaderSearch;
  readonly filters: readonly FilterConfig[];
  readonly isDesktopFiltersOpen: boolean;
  readonly setIsDesktopFiltersOpen: (open: boolean) => void;
  readonly badge?: PageHeaderBadge;
  readonly activeFilters: readonly ActiveFilter[];
  readonly onClearAllFilters?: () => void;
  readonly overflowMenu?: readonly OverflowMenuItem[];
  readonly primaryAction?: React.ReactNode;
  readonly activeFilterCount?: number;
  readonly showChipsRow: boolean;
  readonly usesFilterPanel: boolean;
  readonly filterSections?: readonly FilterSection[];
  /** Tailwind class that shows this section at the desktop breakpoint. */
  readonly showClass: "hidden lg:block" | "hidden xl:block";
}

function DesktopSearchSection({
  search,
  filters,
  isDesktopFiltersOpen,
  setIsDesktopFiltersOpen,
  badge,
  activeFilters,
  onClearAllFilters,
  overflowMenu,
  primaryAction,
  activeFilterCount,
  showChipsRow,
  usesFilterPanel,
  filterSections,
  showClass,
}: DesktopSearchSectionProps) {
  const {
    anchorRef: desktopFilterButtonRef,
    anchor: desktopFilterPanelAnchor,
    toggle: toggleDesktopFilters,
  } = useFilterPanelAnchor(isDesktopFiltersOpen, setIsDesktopFiltersOpen);
  const hasFilters = filters.length > 0;
  const hasOverflowMenu = overflowMenu !== undefined && overflowMenu.length > 0;
  const hasPrimaryAction = primaryAction != null;
  const hasBadge = badge !== undefined;
  const showFilterPopover = usesFilterPanel;
  const hasInlineFilters = hasFilters && !showFilterPopover;

  // With a primaryAction the header splits into two rows. On a page whose
  // filters live in the popover and that has no badge, the second row would
  // carry nothing but the kebab — a lone icon on its own line reads as a
  // layout accident. Keep it in row 1 next to the primary action there, and
  // drop the empty second row entirely.
  const kebabJoinsPrimaryRow =
    hasPrimaryAction && hasOverflowMenu && !hasBadge && !hasInlineFilters;

  const showSearchRow =
    search !== undefined ||
    hasFilters ||
    hasBadge ||
    hasOverflowMenu ||
    hasPrimaryAction;

  if (!showSearchRow && activeFilters.length === 0) {
    return null;
  }

  const desktopFilterButton =
    hasFilters && showFilterPopover ? (
      <>
        <FilterButton
          isOpen={isDesktopFiltersOpen}
          onClick={toggleDesktopFilters}
          hasActiveFilters={activeFilters.length > 0}
          activeCount={activeFilterCount}
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
          variant="quiet"
          sections={filterSections}
          testId="desktop-filter-panel"
        />
      </>
    ) : null;
  const inlineDesktopFilters = hasInlineFilters ? (
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
        <SharedSearchBar {...search} className="min-w-48 flex-1" size="md" />
      )}
      {desktopFilterButton}
    </div>
  );

  const kebab = (
    <div className={hasBadge ? "" : "ml-auto"}>
      <OverflowMenu items={overflowMenu ?? []} />
    </div>
  );

  // Two-row layout when a primaryAction is set: row 1 = search +
  // primaryAction (right), row 2 = filters + kebab (right).
  if (hasPrimaryAction) {
    return (
      <div className={`[&>*:last-child]:mb-0 ${showClass}`}>
        <div className="mb-3 flex items-center gap-3">
          {showFilterPopover
            ? popoverSearchCluster
            : search && (
                <SharedSearchBar
                  {...search}
                  className="min-w-48 flex-1"
                  size="md"
                />
              )}
          <div className="ml-auto flex flex-shrink-0 items-center gap-2">
            {badge ? <RowBadge badge={badge} variant="desktop" /> : null}
            {primaryAction}
            {kebabJoinsPrimaryRow ? (
              <OverflowMenu items={overflowMenu ?? []} />
            ) : null}
          </div>
        </div>

        {(hasInlineFilters || hasBadge) && (
          <div className="mb-3 flex items-center gap-3">
            {inlineDesktopFilters}
            {hasOverflowMenu && !kebabJoinsPrimaryRow ? kebab : null}
          </div>
        )}

        {showChipsRow && (
          <ActiveFilterChips
            filters={activeFilters}
            onClearAll={onClearAllFilters}
          />
        )}
      </div>
    );
  }

  const searchBarClass =
    hasFilters || hasBadge || hasOverflowMenu
      ? "min-w-48 max-w-96 flex-1"
      : "flex-1";

  return (
    <div className={`[&>*:last-child]:mb-0 ${showClass}`}>
      {showSearchRow && (
        <div className="mb-3 flex flex-wrap items-center gap-3">
          {showFilterPopover ? (
            popoverSearchCluster
          ) : (
            <>
              {search && (
                <SharedSearchBar
                  {...search}
                  className={searchBarClass}
                  size="md"
                />
              )}
              {inlineDesktopFilters}
            </>
          )}

          {badge ? <RowBadge badge={badge} variant="desktop" /> : null}

          {hasOverflowMenu ? kebab : null}
        </div>
      )}

      {showChipsRow && (
        <ActiveFilterChips
          filters={activeFilters}
          onClearAll={onClearAllFilters}
        />
      )}
    </div>
  );
}
