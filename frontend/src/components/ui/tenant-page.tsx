"use client";

import Link from "next/link";
import { ChevronDown } from "lucide-react";
import type { ReactNode } from "react";
import { useCallback, useLayoutEffect, useRef, useState } from "react";
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

  /**
   * Hoehe der Elemente im searchSlot. "controls" (Standard) zieht Knoepfe,
   * Felder und Auswahllisten auf die einheitliche Bedienhoehe von 36 px.
   * "natural" laesst dem Slot seine eigene Hoehe -- fuer Inhalte, die keine
   * Bedienzeile sind, sondern die Hauptaktion der Seite (die grossen
   * Aktionskarten in der Kindakte).
   */
  readonly searchSlotHeight?: "controls" | "natural";

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
  searchSlotHeight = "controls",
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
          <div
            className={cn(
              "mt-4",
              searchSlotHeight === "controls" && CONTROL_HEIGHT,
            )}
          >
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
/** Abstand zwischen zwei Reitern (gap-6) -- die Messung braucht ihn als Zahl. */
const TAB_GAP = 24;

/** Auffangreiter, wenn die Breite nicht fuer alle Reiter reicht. */
const MORE_LABEL = "Mehr";

function TenantPageTabs({
  value,
  onChange,
  items,
  label = "Seitenbereiche",
}: NonNullable<TenantPageProps["tabs"]>) {
  // Reiter werden NICHT von Hand gebuendelt. Ein benannter Sammelreiter
  // („Verwaltung") ist geraten: er verraet nicht, was in ihm liegt, und er
  // buendelt auch dann, wenn der Platz laengst reicht. Stattdessen wird
  // gemessen -- sichtbar ist, was hineinpasst, der Rest steht unter „Mehr".
  const rowRef = useRef<HTMLDivElement>(null);
  const measureRef = useRef<HTMLDivElement>(null);
  const [visibleCount, setVisibleCount] = useState(items.length);

  const measure = useCallback(() => {
    const row = rowRef.current;
    const shadow = measureRef.current;
    if (!row || !shadow) return;

    const children = Array.from(shadow.children) as HTMLElement[];
    if (children.length < 2) return;
    // Das letzte Kind der Schattenzeile ist der „Mehr"-Auslöser.
    const moreWidth = children[children.length - 1]!.offsetWidth;
    const widths = children.slice(0, -1).map((child) => child.offsetWidth);
    const available = row.clientWidth;

    // Ohne Messwerte (Testumgebung, noch nicht gelayoutet, Zeile verborgen)
    // wird NICHT geraten: dann stehen alle Reiter da. Ein „Mehr" zu bauen,
    // weil die Breite unbekannt ist, versteckt Bereiche ohne Grund.
    if (available === 0 || widths.every((width) => width === 0)) {
      setVisibleCount(items.length);
      return;
    }

    let used = 0;
    let count = 0;
    for (const width of widths) {
      const next = used + (count > 0 ? TAB_GAP : 0) + width;
      if (next > available) break;
      used = next;
      count += 1;
    }

    // Passt nicht alles, braucht auch „Mehr" seinen Platz.
    if (count < widths.length) {
      while (count > 0 && used + TAB_GAP + moreWidth > available) {
        used -= widths[count - 1]! + (count > 1 ? TAB_GAP : 0);
        count -= 1;
      }
    }

    setVisibleCount(Math.max(count, 1));
  }, [items.length]);

  useLayoutEffect(() => {
    measure();
    const row = rowRef.current;
    if (!row || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(row);
    return () => observer.disconnect();
  }, [measure, items]);

  const visible = items.slice(0, visibleCount);
  const hidden = items.slice(visibleCount);

  const tabClass = (active: boolean, disabled?: boolean) =>
    cn(
      "flex shrink-0 items-center gap-1.5 border-b-[3px] pb-3 text-base whitespace-nowrap transition-colors",
      active
        ? "border-moto-green font-semibold text-gray-900"
        : "border-transparent font-medium text-gray-500 hover:border-gray-300 hover:text-gray-900",
      disabled && "cursor-not-allowed opacity-50",
    );

  const renderInner = (item: TenantPageTab) => (
    <>
      {item.label}
      {item.badge !== undefined && item.badge > 0 && (
        <span className="bg-moto-green/10 rounded-full px-1.5 py-0.5 text-xs font-semibold text-gray-900 tabular-nums">
          {item.badge}
        </span>
      )}
    </>
  );

  const renderTab = (item: TenantPageTab, measuring = false) => {
    const active = item.value === value;
    const className = tabClass(active, item.disabled);
    if (item.href && !item.disabled) {
      return (
        <Link
          key={item.value}
          href={item.href}
          role={measuring ? undefined : "tab"}
          aria-selected={measuring ? undefined : active}
          tabIndex={measuring ? -1 : undefined}
          className={className}
          onClick={(event) => {
            // Mittelklick und Klick mit Zusatztaste öffnen ein zweites
            // Dokument -- die aktuelle Seite bleibt stehen, dort gibt es
            // nichts zu bewachen. Der schlichte Linksklick navigiert dagegen
            // weg und läuft deshalb weiter über `onChange`, das den Wächter
            // für ungespeicherte Änderungen befragt.
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
          {renderInner(item)}
        </Link>
      );
    }
    return (
      <button
        key={item.value}
        type="button"
        role={measuring ? undefined : "tab"}
        aria-selected={measuring ? undefined : active}
        tabIndex={measuring ? -1 : undefined}
        disabled={item.disabled}
        onClick={() => onChange(item.value)}
        className={className}
      >
        {renderInner(item)}
      </button>
    );
  };

  const hiddenActive = hidden.find((item) => item.value === value);
  const moreTrigger = (measuring = false) => (
    <OverflowMenu
      key="__mehr__"
      ariaLabel={MORE_LABEL}
      triggerRole={measuring ? undefined : "tab"}
      triggerAriaSelected={measuring ? undefined : Boolean(hiddenActive)}
      triggerClassName={tabClass(Boolean(hiddenActive))}
      // leading-6 am Inhalt: ohne das drückt das Pfeil-Symbol die Zeilenhöhe
      // um ein Pixel und der Reiter steht einen Hauch tiefer als seine
      // Nachbarn.
      triggerContent={
        <span className="flex items-center gap-1 leading-6">
          {hiddenActive ? hiddenActive.label : MORE_LABEL}
          <ChevronDown className="size-3.5" aria-hidden />
        </span>
      }
      items={(measuring ? items : hidden).map((item) => ({
        label: item.label,
        onClick: () => onChange(item.value),
      }))}
    />
  );

  return (
    <div className="-mx-5 mt-4">
      {/* Unter sm eine Auswahlliste: sieben Reiter nebeneinander wären auf
          einem Telefon eine Scrollleiste, in der die Hälfte der Bereiche
          unsichtbar bleibt. Dieselbe Bauart, andere Form -- kein Sonderweg
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

      {/* Die Grundlinie gehört dem BAND, nicht dem einzelnen Reiter: eine
          Haarlinie über die volle Kartenbreite, der aktive Reiter färbt nur
          sein Stück davon ein. Dadurch sind alle Reiter gleich hohe Kästen
          und der Abstand hängt nicht mehr an einem Strich, den nur einer von
          ihnen trägt. Die Linie verbindet den Reiter zugleich sichtbar mit
          dem Inhalt darunter; eine einzeln getönte Pille sagt das nicht, sie
          liest sich als Filter. */}
      <div className="hidden border-b border-gray-200 px-5 sm:block">
        <div
          ref={rowRef}
          role="tablist"
          aria-label={label}
          className="flex items-end gap-6"
        >
          {visible.map((item) => renderTab(item))}
          {hidden.length > 0 && moreTrigger()}
        </div>

        {/* Schattenzeile für die Messung: sie steht in einem Kasten ohne Höhe
            und ist unsichtbar, behält aber die natürlichen Breiten aller
            Reiter. Ohne sie liesse sich nicht feststellen, wie viele Reiter
            in die Zeile passen -- nur, wie breit die bereits gekürzte Zeile
            ist. */}
        <div className="relative h-0 overflow-hidden" aria-hidden>
          <div
            ref={measureRef}
            className="pointer-events-none invisible absolute top-0 left-0 flex items-end gap-6"
          >
            {items.map((item) => renderTab(item, true))}
            {moreTrigger(true)}
          </div>
        </div>
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
