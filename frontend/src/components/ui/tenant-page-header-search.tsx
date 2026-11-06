"use client";

import { PageHeaderSearchLayout } from "~/components/ui/page-header/PageHeaderSearchLayout";
import type {
  ActiveFilter,
  FilterConfig,
  OverflowMenuItem,
} from "~/components/ui/page-header/types";
import { cn } from "~/lib/utils";
import type { TenantPageProps } from "./tenant-page-types";

// Eine Bedienhöhe in der Kopfkarte: 40 px für Schaltflächen, Links in
// Button-Optik (`ButtonLink`), Auswahl- UND Eingabefelder, auf dem Telefon
// 44 px (die Untergrenze für eine Touch-Fläche). Die Klassen stehen hier
// statisch, damit Tailwind sie in der nachgeladenen Zeile erkennt.
const CONTROL_HEIGHT =
  "[&_button]:h-10 [&_a[data-button-link]]:h-10 [&_select]:h-10 [&_input]:h-10 max-sm:[&_button]:h-11 max-sm:[&_a[data-button-link]]:h-11 max-sm:[&_select]:h-11 max-sm:[&_input]:h-11";

type TenantPageHeaderSearchProps = Pick<
  TenantPageProps,
  | "search"
  | "filters"
  | "activeFilters"
  | "onClearAllFilters"
  | "overflowMenu"
  | "badge"
  | "filterVariant"
  | "filterSections"
  | "desktopFiltersFrom"
  | "activeFilterDisplay"
  | "primaryAction"
  | "searchSlot"
  | "searchSlotHeight"
> & {
  readonly hasSearchRow: boolean;
};

export function TenantPageHeaderSearch({
  search,
  filters,
  activeFilters,
  onClearAllFilters,
  overflowMenu,
  badge,
  filterVariant,
  filterSections,
  desktopFiltersFrom,
  activeFilterDisplay,
  primaryAction,
  searchSlot,
  searchSlotHeight = "controls",
  hasSearchRow,
}: TenantPageHeaderSearchProps) {
  return (
    <div
      className={cn(
        "compact:mt-3 mt-4 max-sm:mt-3",
        searchSlotHeight === "controls" && CONTROL_HEIGHT,
      )}
    >
      {searchSlot}
      {!searchSlot && hasSearchRow && (
        <PageHeaderSearchLayout
          // Der Titel steht in der Kopfkarte darüber.
          embedded
          title=""
          search={search}
          filters={filters as FilterConfig[] | undefined}
          activeFilters={activeFilters as ActiveFilter[] | undefined}
          onClearAllFilters={onClearAllFilters}
          overflowMenu={overflowMenu as OverflowMenuItem[] | undefined}
          badge={badge}
          filterVariant={filterVariant}
          filterSections={filterSections}
          desktopFiltersFrom={desktopFiltersFrom}
          activeFilterDisplay={activeFilterDisplay}
          primaryAction={primaryAction}
        />
      )}
    </div>
  );
}
