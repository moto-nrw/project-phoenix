"use client";

import type { ReactNode } from "react";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type { PageHeaderWithSearchProps } from "~/components/ui/page-header/types";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import type {
  ActiveFilter,
  FilterConfig,
  OverflowMenuItem,
} from "~/components/ui/page-header/types";
import { cn } from "~/lib/utils";

/**
 * Das Seitengerüst des Tenant-Portals. JEDE Seite unter
 * `app/[tenant]/(protected)` rendert genau dieses Gerüst als Wurzel
 * (Ausnahmen: `/dashboard`, `/profile`, `/emergency`).
 *
 * Die Reihenfolge der Bausteine ist fest und liegt hier, nicht in der Seite:
 *
 *   1. Kopfkarte:  Titel  ...............  Aktionen
 *                  Statuszeile (echte Zahlen der Seite)
 *                  Suche + Filter
 *   2. Reiter:     horizontale Seitenreiter (immer dasselbe Bauteil)
 *   3. Inhalt:     Karten im 24-px-Rhythmus
 *
 * Damit kann eine Seite nicht mehr entscheiden, ob sie einen Kopf hat, wo ihre
 * Aktionen sitzen oder wie ihr Ladezustand aussieht. Sie liefert Daten, kein
 * Layout: kein eigenes `max-w`, kein `mx-auto`, kein eigenes Padding, kein
 * zweites `<main>`, keine eigene `<h1>`. Das Padding kommt aus der Shell.
 *
 * Es gibt bewusst KEINE Mini-Überschrift über dem Titel. Wo man ist, sagen die
 * Brotkrumen in der Kopfzeile und die Seitenleiste.
 */
export interface TenantPageTab {
  readonly value: string;
  readonly label: string;
  /** Zahl rechts am Reiter, zum Beispiel offene Anfragen. */
  readonly badge?: number;
  readonly disabled?: boolean;
}

export interface TenantPageProps {
  readonly title: string;
  /**
   * Statuszeile unter dem Titel: echte Zahlen der Seite, die sie ohnehin lädt
   * („116 Kinder · 107 zuhause · 9 krank"). Kein Erklärsatz. Während des
   * Ladens rendert das Gerüst an dieser Stelle ein Skelett.
   */
  readonly stats?: ReactNode;
  /** Zeigt statt der Statuszeile ein Skelett. */
  readonly statsLoading?: boolean;
  /** Aktionen rechts in der Titelzeile (Primäraktion, Export, Kebab). */
  readonly actions?: ReactNode;
  /** Element links vom Titel, zum Beispiel Avatar oder Konzept-Icon. */
  readonly leading?: ReactNode;
  /** Hebt den Titel eine Stufe an (Startseiten mit Begrüßung). */
  readonly prominent?: boolean;
  /** Blendet den mobilen Zurück-Knopf ein (Unterseiten). */
  readonly back?: boolean;
  /** Ziel des Zurück-Knopfs. Ohne Angabe die Datenverwaltung. */
  readonly backHref?: string;
  /** Vorlesbarer Text des Zurück-Knopfs, passend zum Ziel. */
  readonly backLabel?: string;

  /** Suchfeld in der Kopfkarte. */
  readonly search?: {
    readonly value: string;
    readonly onChange: (value: string) => void;
    readonly placeholder?: string;
  };
  readonly filters?: readonly FilterConfig[];
  readonly activeFilters?: readonly ActiveFilter[];
  readonly onClearAllFilters?: () => void;
  readonly overflowMenu?: readonly OverflowMenuItem[];
  /**
   * Zaehler-Plakette in der Such- und Filterzeile, z. B. die Zahl der
   * gefundenen Kinder. Nur fuer eine Zahl, die sich mit der Filterung aendert
   * — feste Kennzahlen gehoeren in `stats`.
   */
  readonly badge?: {
    readonly icon?: ReactNode;
    readonly count: number;
    readonly label?: string;
  };
  /**
   * Sammelt viele Filter hinter einer Schaltflaeche statt sie in die Zeile zu
   * legen. Ab etwa drei Filtern ist das die richtige Wahl — eine Zeile, die
   * umbricht, ist keine Zeile mehr. Mit `filterSections` bekommt die Flaeche
   * dahinter Ueberschriften.
   */
  readonly filterVariant?: "default" | "quiet";
  readonly filterSections?: PageHeaderWithSearchProps["filterSections"];
  /** Ab welcher Breite die Filter wieder in der Zeile stehen duerfen. */
  readonly desktopFiltersFrom?: PageHeaderWithSearchProps["desktopFiltersFrom"];
  /** Aktive Filter als Zahl auf der Schaltflaeche statt als Chip-Reihe. */
  readonly activeFilterDisplay?: PageHeaderWithSearchProps["activeFilterDisplay"];
  /**
   * Hauptaktion IN der Such- und Filterzeile statt in der Titelzeile. Nur fuer
   * Aktionen, die zur Liste darunter gehoeren (An- und Abmelden).
   */
  readonly primaryAction?: ReactNode;

  /**
   * Fertige Such- und Filterzeile fuer Layout-Adapter (DatabasePageLayout),
   * die ihre PageHeaderWithSearch selbst konfigurieren. Seiten nutzen
   * search/filters und NICHT diesen Slot.
   */
  readonly searchSlot?: ReactNode;

  /** Horizontale Seitenreiter unter der Kopfkarte. */
  readonly tabs?: {
    readonly value: string;
    readonly onChange: (value: string) => void;
    readonly items: readonly TenantPageTab[];
    /** Vorlesbarer Name der Reiterleiste. */
    readonly label?: string;
  };

  /**
   * Fehlerzustand: ersetzt den Inhalt, die Kopfkarte bleibt stehen. Die
   * Objektform nimmt eine Aktion auf, damit ein „Erneut versuchen" nicht
   * ersatzlos verschwindet.
   */
  readonly error?: string | { message: string; action?: ReactNode } | null;
  /** Ladezustand: ersetzt den Inhalt durch Skelette. */
  readonly loading?: boolean;
  /** Leerzustand: ersetzt den Inhalt, sobald nichts zu zeigen ist. */
  readonly empty?: {
    readonly title: string;
    readonly description?: string;
    readonly icon?: ReactNode;
    readonly action?: ReactNode;
  } | null;

  readonly children?: ReactNode;
  /** Nur für Testselektoren, nicht für Layout. */
  readonly testId?: string;
}

/** Ein Wert-Label-Paar der Statuszeile. Trennzeichen setzt `TenantPageStats`. */
export function TenantPageStats({
  items,
}: Readonly<{
  items: readonly { readonly value: ReactNode; readonly label: string }[];
}>) {
  return (
    <span className="inline-flex flex-wrap items-center gap-x-1.5 gap-y-1">
      {items.map((item, index) => (
        <span key={item.label} className="inline-flex items-center gap-1.5">
          {index > 0 && (
            <span aria-hidden="true" className="text-gray-300">
              ·
            </span>
          )}
          <span className="font-medium text-gray-900 tabular-nums">
            {item.value}
          </span>
          <span>{item.label}</span>
        </span>
      ))}
    </span>
  );
}

/**
 * Eine Bedienhöhe im Seitenkopf. Ohne diese Klammer stehen dort 32, 36 und
 * 40 px nebeneinander, je nachdem welches Kit-Bauteil eine Seite gerade
 * greift — gemessen auf /statistics: Zeitraum 32, Export 36, Filter 40.
 * Der Selektor greift auch verschachtelte Auslöser, deshalb Nachfahren und
 * nicht nur direkte Kinder.
 */
const CONTROL_HEIGHT = "[&_button]:h-9 [&_select]:h-9";

export function TenantPage({
  title,
  stats,
  statsLoading = false,
  actions,
  leading,
  prominent = false,
  back = false,
  backHref,
  backLabel,
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
  tabs,
  error,
  loading = false,
  empty,
  children,
  testId,
}: TenantPageProps) {
  const hasSearchRow =
    search !== undefined ||
    (filters?.length ?? 0) > 0 ||
    (overflowMenu?.length ?? 0) > 0 ||
    primaryAction != null ||
    badge !== undefined;

  const statusLine = statsLoading ? (
    <Skeleton className="h-4 w-56" />
  ) : (
    (stats ?? null)
  );

  return (
    <div className="w-full" data-testid={testId}>
      {back && (
        <MobileBackButton
          href={backHref}
          // Ohne eigenen Text würde der Standardtext „Zurück zur
          // Datenverwaltung" ein falsches Ziel ansagen.
          ariaLabel={backLabel ?? (backHref ? "Zurück" : undefined)}
        />
      )}

      {/* Die Kopfkarte trägt Titel, Statuszeile, Aktionen und die Such- und
          Filterzeile. Sie schließt eng um ihren Inhalt: 20 px Rand, 4 px
          zwischen Titel und Statuszeile, 16 px vor der Suchzeile. Kein
          reservierter Leerraum darunter — genau der war der tote Streifen. */}
      <header className="moto-content-surface rounded-2xl border p-5 shadow-sm">
        <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
          <div className="flex min-w-0 items-center gap-3">
            {leading}
            <div className="min-w-0">
              <h1
                className={cn(
                  "font-semibold tracking-tight text-balance text-gray-900",
                  prominent
                    ? "text-2xl leading-tight sm:text-[28px]"
                    : "text-xl leading-tight sm:text-2xl",
                )}
              >
                {title}
              </h1>
              {statusLine != null && (
                <p className="mt-1 text-sm leading-5 text-gray-600">
                  {statusLine}
                </p>
              )}
            </div>
          </div>
          {actions && (
            // Eine Bedienhöhe im Kopf. Ohne diese Klammer stehen dort 32,
            // 36 und 40 px nebeneinander, je nachdem welches Kit-Bauteil
            // eine Seite gerade greift.
            <div
              className={cn(
                // Unter sm eine eigene Zeile über die volle Breite -- aber
                // nur ab zwei Aktionen: sonst hängen sie untereinander am
                // rechten Rand und die Kopfkarte wächst um eine halbleere
                // Zeile. Eine einzelne Aktion (meist nur das Kebab-Menü)
                // bleibt neben dem Titel stehen, statt sich eine eigene,
                // fast leere Zeile darunter zu nehmen.
                "flex shrink-0 flex-wrap items-center gap-2",
                "max-sm:[&:has(>*:nth-child(2))]:w-full",
                CONTROL_HEIGHT,
              )}
            >
              {actions}
            </div>
          )}
        </div>

        {(searchSlot ?? hasSearchRow) && (
          <div className={cn("mt-4", CONTROL_HEIGHT)}>
            {searchSlot}
            {!searchSlot && hasSearchRow && (
              <PageHeaderWithSearch
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
        )}
      </header>

      {/* Die Reiter gehören zum Kopf, nicht zum Inhalt: eng darunter, mit
          echtem Abstand zum Inhalt. Vorher lagen 24 px darüber und 0 darunter,
          der Inhalt klebte an der Trennlinie. */}
      {tabs && (
        <div className="mt-4">
          <TenantPageTabs {...tabs} />
        </div>
      )}

      <div className="mt-6 space-y-6">
        <TenantPageBody
          loading={loading}
          loadingLabel={`${title} wird geladen…`}
          error={error}
          empty={empty}
        >
          {children}
        </TenantPageBody>
      </div>
    </div>
  );
}

/**
 * Die EINE Bauart für Seitenreiter im Tenant-Portal. Bewusst einfache
 * Schaltflächen statt Radix: Radix-Reiter schalten auf `mousedown`, was die
 * bestehenden `fireEvent.click`-Tests still ins Leere laufen lässt.
 * `ui/Tabs` bleibt für Reiter INNERHALB einer Karte, `SegmentedControl` für
 * eine Wertauswahl.
 */
function TenantPageTabs({
  value,
  onChange,
  items,
  label = "Seitenbereiche",
}: NonNullable<TenantPageProps["tabs"]>) {
  return (
    <>
      {/* Unter sm eine Auswahlliste: sieben Reiter nebeneinander wären auf
          einem Telefon eine Scrollleiste, in der die Hälfte der Bereiche
          unsichtbar bleibt. Dieselbe Bauart, andere Form — kein Sonderweg
          pro Seite. */}
      <div className="sm:hidden">
        <label className="sr-only" htmlFor="tenant-page-tabs">
          {label}
        </label>
        <select
          id="tenant-page-tabs"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className="focus:border-moto-green focus:ring-moto-green/30 w-full rounded-md border border-gray-200 bg-white px-4 py-2.5 text-sm font-medium text-gray-900 shadow-sm focus:ring-2 focus:outline-none"
        >
          {items.map((item) => (
            <option
              key={item.value}
              value={item.value}
              disabled={item.disabled}
            >
              {item.badge !== undefined && item.badge > 0
                ? `${item.label} (${item.badge})`
                : item.label}
            </option>
          ))}
        </select>
      </div>

      <div
        role="tablist"
        aria-label={label}
        className="-mb-px hidden gap-1 overflow-x-auto border-b border-gray-200 sm:flex"
      >
        {items.map((item) => {
          const active = item.value === value;
          return (
            <button
              key={item.value}
              type="button"
              role="tab"
              aria-selected={active}
              disabled={item.disabled}
              onClick={() => onChange(item.value)}
              className={cn(
                "flex shrink-0 items-center gap-2 border-b-2 px-3 py-2.5 text-sm font-medium whitespace-nowrap transition-colors",
                active
                  ? "border-moto-green text-gray-900"
                  : "border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-900",
                item.disabled && "cursor-not-allowed opacity-50",
              )}
            >
              {item.label}
              {item.badge !== undefined && item.badge > 0 && (
                <span className="bg-moto-green/10 rounded-full px-1.5 py-0.5 text-xs font-semibold text-gray-900 tabular-nums">
                  {item.badge}
                </span>
              )}
            </button>
          );
        })}
      </div>
    </>
  );
}

/**
 * Fehler, Laden und Leerzustand an EINER Stelle, damit sie auf keiner Seite
 * mehr als `return null`, freier Spinner oder „Wird geladen…"-Fließtext
 * auftauchen.
 */
function TenantPageBody({
  loading,
  loadingLabel,
  error,
  empty,
  children,
}: Readonly<{
  loading: boolean;
  /** Vorlesbarer Text des Ladezustands, aus dem Seitentitel gebildet. */
  loadingLabel: string;
  error?: TenantPageProps["error"];
  empty?: TenantPageProps["empty"];
  children?: ReactNode;
}>) {
  if (error) {
    const { message, action } =
      typeof error === "string" ? { message: error, action: undefined } : error;
    return <Alert type="error" message={message} action={action} />;
  }
  if (loading) {
    return (
      <div
        className="space-y-3"
        role="status"
        aria-busy="true"
        aria-live="polite"
        aria-label={loadingLabel}
      >
        <Skeleton className="h-24 w-full rounded-2xl" />
        <Skeleton className="h-24 w-full rounded-2xl" />
        <Skeleton className="h-24 w-full rounded-2xl" />
      </div>
    );
  }
  if (empty) {
    // Auf derselben Fläche wie der Inhalt, den sie ersetzt: sonst steht der
    // Leerzustand als einziger Seitenzustand ohne Karte auf dem Hintergrund
    // und die Seite sieht leer statt aufgeräumt aus. Laden (Skelettkarten)
    // und Fehler (getönte Fläche) tragen ihre Fläche schon.
    return (
      <SectionCard>
        <EmptyState
          title={empty.title}
          description={empty.description}
          icon={empty.icon}
          action={empty.action}
        />
      </SectionCard>
    );
  }
  return <>{children}</>;
}
