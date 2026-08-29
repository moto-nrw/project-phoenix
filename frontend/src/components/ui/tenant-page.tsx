"use client";

import Link from "next/link";
import { ChevronDown } from "lucide-react";
import type { ReactNode } from "react";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
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
  /**
   * Zielpfad, wenn der Reiter auf eine eigene Seite führt (die Register einer
   * Sammlung: „Kinder · Meine Gruppen · Stammdaten"). Dann rendert der Reiter
   * ein echtes `<a>`, damit Mittelklick, „in neuem Tab öffnen" und die
   * Tastatur funktionieren — ein Knopf mit `router.push` kann das nicht.
   * Ohne `href` schaltet der Reiter wie bisher nur den Inhalt um.
   */
  readonly href?: string;
  /**
   * Untereinträge. Ein Reiter mit Menü bündelt Flächen, die man selten
   * braucht (die Register einer Sammlung), damit sie nicht gleichrangig neben
   * der täglichen Ansicht stehen. Sechs Reiter nebeneinander sind eine
   * Werkzeugleiste, keine Orientierung.
   */
  readonly menu?: readonly TenantPageTab[];
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
   *
   * `keepContent` stellt die Meldung ÜBER den Inhalt, statt ihn zu ersetzen.
   * Das ist der Fall, wenn über denselben Kanal auch der Fehler einer
   * Einzelaktion gemeldet wird („Kind konnte nicht hinzugefügt werden"):
   * ersetzte die Meldung dort den Inhalt, wäre die Fläche nach dem ersten
   * Fehlschlag nicht mehr bedienbar und ein zweiter Versuch unmöglich.
   * Für einen fehlgeschlagenen Erstabruf bleibt die ersetzende Form richtig —
   * dahinter steht kein Inhalt, den man stehen lassen könnte.
   */
  readonly error?:
    | string
    | { message: string; action?: ReactNode; keepContent?: boolean }
    | null;
  /**
   * Ladezustand: ersetzt den Inhalt durch Skelette. `true` rendert das
   * generische Kartenskelett; ein Knoten rendert stattdessen das
   * Struktur-Skelett der Fläche (ein Plan-Raster lädt nicht wie eine
   * Kartenliste). Die vorlesbare Ankündigung und die Rolle bleiben in beiden
   * Fällen dieselben — deshalb kommt das Skelett hier herein und baut sich
   * nicht daneben eine zweite Ladefläche.
   */
  readonly loading?: boolean | ReactNode;
  /**
   * Vorlesbare Ankündigung des Ladezustands. Ohne Angabe wird sie aus dem
   * Titel gebildet („Kinder wird geladen…") — bei pluralen Titeln stimmt das
   * Verb dann nicht, deshalb kann die Seite den Satz selbst setzen.
   */
  readonly loadingLabel?: string;
  /**
   * Dialoge, Slide-overs und andere Overlays der Seite. Sie gehören NICHT zu
   * `children`: das Gerüst ersetzt den Inhalt in `loading`, `empty` und
   * `error`, und ein Dialog, der als Kind steht, wird dabei mit ausgehängt —
   * dann öffnet der Knopf im Leerzustand nichts. Was hier steht, bleibt in
   * jedem Zustand gemountet.
   */
  readonly overlays?: ReactNode;
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
// Eine Bedienhöhe in der Kopfkarte: 36 px für Schaltflächen, Auswahl- UND
// Eingabefelder. Das Eingabefeld fehlte hier, deshalb stand das Suchfeld mit
// 42 px neben Filterknöpfen mit 36 — auf acht Seiten derselbe Versatz.
const CONTROL_HEIGHT = "[&_button]:h-9 [&_select]:h-9 [&_input]:h-9";

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
  loadingLabel,
  overlays,
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
      {/* Eine Fläche von Titel bis Reiter. Die Reiter lagen vorher frei auf
          dem gemusterten Grund und sahen aus wie nachträglich dazwischen
          geschoben; sie sind jetzt die letzte Zeile des Kopfes. */}
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

        {/* Der Reiter bestimmt, WAS man ansieht; die Suche filtert DARIN.
            Deshalb steht er über der Suchzeile — darunter liest er sich als
            ein weiteres Filterelement. */}
        {tabs && <TenantPageTabs {...tabs} />}

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

      <div className={cn("space-y-6", tabs ? "mt-6" : "mt-6")}>
        <TenantPageBody
          loading={loading}
          loadingLabel={loadingLabel ?? `${title} wird geladen…`}
          error={error}
          empty={empty}
        >
          {children}
        </TenantPageBody>
      </div>
      {overlays}
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
    <div className="-mx-5 mt-4">
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
          {items
            .flatMap((item) => (item.menu ? [...item.menu] : [item]))
            .map((item) => (
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

      {/* Die Grundlinie gehört dem BAND, nicht dem einzelnen Reiter: eine
          Haarlinie über die volle Kartenbreite, der aktive Reiter färbt nur
          sein Stück davon ein. Dadurch sind alle Reiter gleich hohe Kästen
          und der Abstand hängt nicht mehr an einem Strich, den nur einer von
          ihnen trägt — der Fehler der ersten Fassung. Die Linie verbindet den
          Reiter zugleich sichtbar mit dem Inhalt darunter; eine einzeln
          getönte Pille sagt das nicht, sie liest sich als Filter. */}
      <div
        role="tablist"
        aria-label={label}
        className="hidden items-end gap-6 overflow-x-auto border-b border-gray-200 px-5 sm:flex"
      >
        {items.map((item) => {
          const active = item.menu
            ? item.menu.some((entry) => entry.value === value)
            : item.value === value;
          // Der aktive Reiter ist eine Fläche, kein Unterstrich. Ein Strich
          // an der Unterkante haben nur die aktiven Reiter, und die Zeile
          // richtet sich dann optisch an etwas aus, das den meisten Reitern
          // fehlt: unter dem Text der übrigen steht doppelt so viel Luft.
          // Als gleich große Kästen hängt der Abstand an nichts mehr — und es
          // ist dieselbe Sprache wie in der Seitenleiste: aktiv ist Fläche und
          // Schriftschnitt, nicht Farbe.
          const tabClass = cn(
            "flex shrink-0 items-center gap-1.5 border-b-[3px] pb-3 text-base whitespace-nowrap transition-colors",
            active
              ? "border-moto-green font-semibold text-gray-900"
              : "border-transparent font-medium text-gray-500 hover:border-gray-300 hover:text-gray-900",
            item.disabled && "cursor-not-allowed opacity-50",
          );
          const inner = (
            <>
              {item.label}
              {item.badge !== undefined && item.badge > 0 && (
                <span className="bg-moto-green/10 rounded-full px-1.5 py-0.5 text-xs font-semibold text-gray-900 tabular-nums">
                  {item.badge}
                </span>
              )}
            </>
          );
          if (item.menu && item.menu.length > 0) {
            const openEntry = item.menu.find((entry) => entry.value === value);
            return (
              <OverflowMenu
                key={item.value}
                ariaLabel={item.label}
                triggerRole="tab"
                triggerAriaSelected={active}
                triggerClassName={tabClass}
                // leading-6 am Inhalt: ohne das drückt das Pfeil-Symbol die
                // Zeilenhöhe um ein Pixel und der Reiter steht einen Hauch
                // tiefer als seine Nachbarn.
                triggerContent={
                  <span className="flex items-center gap-1 leading-6">
                    {openEntry ? openEntry.label : item.label}
                    <ChevronDown className="size-3.5" aria-hidden />
                  </span>
                }
                items={item.menu.map((entry) => ({
                  label: entry.label,
                  onClick: () => onChange(entry.value),
                }))}
              />
            );
          }
          if (item.href && !item.disabled) {
            return (
              <Link
                key={item.value}
                href={item.href}
                role="tab"
                aria-selected={active}
                className={tabClass}
                onClick={(event) => {
                  // Mittelklick und Klick mit Zusatztaste öffnen ein zweites
                  // Dokument — die aktuelle Seite bleibt stehen, dort gibt es
                  // nichts zu bewachen. Der schlichte Linksklick navigiert
                  // dagegen weg und läuft deshalb weiter über `onChange`, das
                  // den Wächter für ungespeicherte Änderungen befragt.
                  if (
                    event.defaultPrevented ||
                    event.button !== 0 ||
                    event.metaKey ||
                    event.ctrlKey ||
                    event.shiftKey ||
                    event.altKey
                  ) {
                    return;
                  }
                  event.preventDefault();
                  onChange(item.value);
                }}
              >
                {inner}
              </Link>
            );
          }
          return (
            <button
              key={item.value}
              type="button"
              role="tab"
              aria-selected={active}
              disabled={item.disabled}
              onClick={() => onChange(item.value)}
              className={tabClass}
            >
              {inner}
            </button>
          );
        })}
      </div>
    </div>
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
  loading: boolean | ReactNode;
  /** Vorlesbarer Text des Ladezustands, aus dem Seitentitel gebildet. */
  loadingLabel: string;
  error?: TenantPageProps["error"];
  empty?: TenantPageProps["empty"];
  children?: ReactNode;
}>) {
  const errorParts =
    typeof error === "string" ? { message: error } : (error ?? undefined);
  if (errorParts && !errorParts.keepContent) {
    return (
      <Alert
        type="error"
        message={errorParts.message}
        action={errorParts.action}
      />
    );
  }
  const errorBanner = errorParts ? (
    <Alert
      type="error"
      message={errorParts.message}
      action={errorParts.action}
    />
  ) : null;
  if (loading) {
    return (
      <>
        {errorBanner}
        <div
          className={loading === true ? "space-y-3" : undefined}
          role="status"
          aria-busy="true"
          aria-live="polite"
          aria-label={loadingLabel}
        >
          {loading === true ? (
            <>
              <Skeleton className="h-24 w-full rounded-2xl" />
              <Skeleton className="h-24 w-full rounded-2xl" />
              <Skeleton className="h-24 w-full rounded-2xl" />
            </>
          ) : (
            loading
          )}
        </div>
      </>
    );
  }
  if (empty) {
    // Auf derselben Fläche wie der Inhalt, den sie ersetzt: sonst steht der
    // Leerzustand als einziger Seitenzustand ohne Karte auf dem Hintergrund
    // und die Seite sieht leer statt aufgeräumt aus. Laden (Skelettkarten)
    // und Fehler (getönte Fläche) tragen ihre Fläche schon.
    return (
      <>
        {errorBanner}
        <SectionCard>
          <EmptyState
            title={empty.title}
            description={empty.description}
            icon={empty.icon}
            action={empty.action}
          />
        </SectionCard>
      </>
    );
  }
  return (
    <>
      {errorBanner}
      {children}
    </>
  );
}
