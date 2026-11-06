"use client";

import { PageHeaderSearchLayout } from "~/components/ui/page-header/PageHeaderSearchLayout";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { SectionCard } from "~/components/ui/section-card";
import type { TenantPageProps } from "./tenant-page-types";

/**
 * Eine Bedienhöhe im Seitenkopf. Ohne diese Klammer stehen dort 32, 36 und
 * 40 px nebeneinander, je nachdem welches Kit-Bauteil eine Seite gerade
 * greift — gemessen auf /statistics: Zeitraum 32, Export 36, Filter 40.
 * Der Selektor greift auch verschachtelte Auslöser, deshalb Nachfahren und
 * nicht nur direkte Kinder.
 */
// Eine Bedienhöhe in der Kopfkarte: 40 px für Schaltflächen, Links in
// Button-Optik (`ButtonLink`), Auswahl- UND Eingabefelder, auf dem Telefon
// 44 px (die Untergrenze für eine Touch-Fläche, dieselbe wie die
// Fingerregel des Kits). Das Eingabefeld fehlte hier anfangs, deshalb stand das
// Suchfeld mit 42 px neben Filterknöpfen mit 36 — auf acht Seiten derselbe
// Versatz; ebenso stand ein `ButtonLink` mit 32 oder 36 px neben 40-px-Knöpfen
// (#5531). Der `SegmentedControl` behauptet seine Segmenthöhe mit `!` und
// kommt mitsamt Spur ebenfalls auf 40 px — siehe dort.
const CONTROL_HEIGHT =
  "[&_button]:h-10 [&_a[data-button-link]]:h-10 [&_select]:h-10 [&_input]:h-10 max-sm:[&_button]:h-11 max-sm:[&_a[data-button-link]]:h-11 max-sm:[&_select]:h-11 max-sm:[&_input]:h-11";

/**
 * Die Such- und Filterzeile der Kopfkarte als eigene Fläche im Inhalt, direkt
 * über der Liste, die sie filtert. Nur für eine Seite, deren Liste weit unter
 * der Kopfkarte beginnt, weil andere Flächen davor stehen (Aktuelle Aufsicht,
 * #4353). Sonst gehören Suche und Filter in `search`/`filters` von
 * `TenantPage` (TENANT-PAGE-SPEC.md, Regel 5).
 */
export function TenantPageListSearch({
  label,
  search,
  filters,
  activeFilters,
  onClearAllFilters,
}: Readonly<
  Pick<
    TenantPageProps,
    "search" | "filters" | "activeFilters" | "onClearAllFilters"
  > & {
    /** Name der Suchfläche für Vorlesegeräte, z. B. „Kinder suchen“. */
    readonly label: string;
  }
>) {
  return (
    // Die Auswahl „Gruppe“ öffnet über den Kartenrand hinaus.
    <SectionCard overflow="visible">
      <div role="search" aria-label={label} className={CONTROL_HEIGHT}>
        <PageHeaderSearchLayout
          embedded
          title=""
          search={search}
          filters={filters as FilterConfig[] | undefined}
          activeFilters={activeFilters as ActiveFilter[] | undefined}
          onClearAllFilters={onClearAllFilters}
        />
      </div>
    </SectionCard>
  );
}
