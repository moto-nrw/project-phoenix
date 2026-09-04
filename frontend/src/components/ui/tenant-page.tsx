"use client";

import Link from "next/link";
import { ChevronDown } from "lucide-react";
import type {
  InputHTMLAttributes,
  KeyboardEvent,
  MouseEvent,
  ReactNode,
} from "react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type { PageHeaderWithSearchProps } from "~/components/ui/page-header/types";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { BELOW_SM, useMediaQuery } from "~/lib/hooks/use-media-query";
import type {
  ActiveFilter,
  FilterConfig,
  OverflowMenuItem,
} from "~/components/ui/page-header/types";
import { cn } from "~/lib/utils";

/**
 * Das Seitengerüst des Tenant-Portals. JEDE Seite unter
 * `app/[tenant]/(protected)` rendert genau dieses Gerüst als Wurzel
 * (Ausnahmen gibt es nicht.)
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
    /** Attribute des zugrunde liegenden Suchfelds, etwa `disabled`. */
    readonly inputProps?: InputHTMLAttributes<HTMLInputElement>;
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

/**
 * Ein Wert-Label-Paar der Statuszeile. Trennzeichen setzt `TenantPageStats`.
 *
 * Höchstens DREI Paare: die Statuszeile ist eine Orientierung, keine
 * Kennzahlenleiste. Sechs Paare (gemessen auf /staff) liest niemand mehr als
 * Satz — sie sind eine Tabelle ohne Kopf. Was über drei hinausgeht, trägt
 * die Fläche darunter (Kacheln, Karten, Tabelle); das Gerüst schneidet hier
 * ab, damit das Budget nicht pro Seite neu verhandelt wird.
 */
const MAX_STATS_ITEMS = 3;

export function TenantPageStats({
  items,
}: Readonly<{
  items: readonly { readonly value: ReactNode; readonly label: string }[];
}>) {
  return (
    <span className="inline-flex flex-wrap items-center gap-x-1.5 gap-y-1">
      {items.slice(0, MAX_STATS_ITEMS).map((item, index) => (
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
// Eine Bedienhöhe in der Kopfkarte: 40 px für Schaltflächen, Auswahl- UND
// Eingabefelder, auf dem Telefon 44 px (die Untergrenze für eine
// Touch-Fläche, dieselbe Begründung wie `Button size="touch"` der
// Eltern-App). Das Eingabefeld fehlte hier anfangs, deshalb stand das
// Suchfeld mit 42 px neben Filterknöpfen mit 36 — auf acht Seiten derselbe
// Versatz. Der `SegmentedControl` behauptet seine Segmenthöhe mit `!` und
// kommt mitsamt Spur ebenfalls auf 40 px — siehe dort.
const CONTROL_HEIGHT =
  "[&_button]:h-10 [&_select]:h-10 [&_input]:h-10 max-sm:[&_button]:h-11 max-sm:[&_select]:h-11 max-sm:[&_input]:h-11";

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
    // `flex-1 flex-col`: die Seite wächst in der Inhaltshülle der Shell bis
    // zur Unterkante des Bildschirms, damit der Rumpf darunter die Höhe
    // füllen kann (siehe `.moto-tenant-body`).
    <div className="flex w-full flex-1 flex-col" data-testid={testId}>
      {/* Die Kopfkarte trägt Titel, Statuszeile, Aktionen und die Such- und
          Filterzeile. Sie schließt eng um ihren Inhalt: 20 px Rand, 4 px
          zwischen Titel und Statuszeile, 16 px vor der Suchzeile. Kein
          reservierter Leerraum darunter — genau der war der tote Streifen. */}
      {/* Eine Fläche von Titel bis Reiter. Die Reiter lagen vorher frei auf
          dem gemusterten Grund und sahen aus wie nachträglich dazwischen
          geschoben; sie sind jetzt die letzte Zeile des Kopfes. */}
      {/* Auf kleinen Bildschirmen liegt der Kopf flach auf dem gemusterten
          Grund. Die spezielle Klasse überschreibt die ungeschichtete
          Flächenklasse auch dann zuverlässig, wenn Tailwind-Utilities in
          einer CSS-Layer stehen. */}
      <header className="moto-content-surface rounded-2xl border p-5 shadow-sm max-sm:p-4">
        {back && (
          <MobileBackButton
            href={backHref}
            // Ohne eigenen Text würde der Standardtext „Zurück zur
            // Datenverwaltung" ein falsches Ziel ansagen.
            ariaLabel={backLabel ?? (backHref ? "Zurück" : undefined)}
          />
        )}
        <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
          {/* `flex-1 basis-0`: der Titelblock nimmt, was die Aktionen übrig
              lassen, statt mit seiner natürlichen Breite zu ringen. Vorher
              schob eine zweizeilige Statuszeile auf dem Telefon selbst das
              einzelne Kebab-Menü in eine eigene, linksbündige Zeile -- und
              die Kopfkarte wuchs um genau den toten Streifen, den sie auf
              dem Desktop nicht mehr hat. */}
          <div className="flex min-w-0 flex-1 basis-0 items-center gap-3 max-sm:min-w-[11rem]">
            {leading}
            <div className="min-w-0">
              <h1
                className={cn(
                  "font-semibold tracking-tight text-balance text-gray-900",
                  // Auch auf dem Telefon sichtbar: unter lg gibt es keine
                  // Shell-Kopfzeile mehr (wie in der Eltern-App), die
                  // Kopfkarte ist die einzige Trägerin des Seitennamens.
                  prominent
                    ? "text-2xl leading-tight sm:text-[28px]"
                    : "text-xl leading-tight sm:text-2xl",
                )}
              >
                {title}
              </h1>
              {statusLine != null &&
                (statsLoading ? (
                  <div className="mt-1 text-sm leading-5 text-gray-600">
                    {statusLine}
                  </div>
                ) : (
                  <p className="mt-1 text-sm leading-5 text-gray-600">
                    {statusLine}
                  </p>
                ))}
            </div>
          </div>
          {actions && (
            // Eine Bedienhöhe im Kopf. Ohne diese Klammer stehen dort 32,
            // 36 und 40 px nebeneinander, je nachdem welches Kit-Bauteil
            // eine Seite gerade greift.
            <div
              className={cn(
                // Die Aktionen stehen neben dem Titel, solange sie neben
                // seiner Mindestbreite (11rem unter sm) Platz haben; sonst
                // brechen sie als Gruppe in eine eigene Zeile um und stehen
                // dort rechts (`ml-auto`), wie auf dem Desktop. Linksbündig
                // stand der tote Raum rechts neben den Knöpfen.
                "ml-auto flex shrink-0 flex-wrap items-center justify-end gap-2",
                // Unter sm gilt: JEDE Zeile ist voll. Textknöpfe bekommen
                // eine eigene, volle Zeile (auch ein einzelner — rechtsbündig
                // neben Leerraum las er sich als toter Bereich); reine
                // Symbolknöpfe (das Kebab-Menü, `Button size="icon"`, per
                // `data-icon-only`) behalten ihr Maß und bleiben in der
                // Titel-/Statuszeile, die ihr Text füllt.
                "max-sm:contents",
                "max-sm:[&>*:not([data-icon-only])]:order-last max-sm:[&>*:not([data-icon-only])]:basis-full",
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
              "mt-4 max-sm:mt-3",
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

      {/* Der Rumpf füllt die Höhe des Bildschirms: seine letzte Fläche
          wächst bis zur Unterkante (`.moto-tenant-body` in globals.css).
          Vorher endete eine Seite mit einem Eintrag oder mit dem Leersatz
          nach 60 px Karte, und darunter stand der halbe Bildschirm nacktes
          Punktraster -- ein anderer Umriss auf jeder Seite. Jetzt haben die
          Seite mit einer Zeile, die mit dreißig und die leere denselben. */}
      <div className="moto-tenant-body mt-6 space-y-6 max-sm:mt-4 max-sm:space-y-3">
        <TenantPageBody
          loading={loading}
          loadingLabel={loadingLabel ?? `Der Bereich „${title}“ wird geladen…`}
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
  const tablistRef = useRef<HTMLDivElement>(null);
  const measureRef = useRef<HTMLDivElement>(null);
  // Vor der ersten Client-Auflösung gilt die Desktop-Variante. Das entspricht
  // dem Server-Render und verhindert eine Hydrations-Abweichung.
  const isPhone = useMediaQuery(BELOW_SM);
  const [visibleCount, setVisibleCount] = useState(items.length);
  // Liegen im scrollenden Band (unter sm) rechts noch Reiter? Dann zeigt ein
  // Verlaufs-Fade an der Kartenkante, dass man scrollen kann — eine hart
  // angeschnittene Pille liest sich sonst als Darstellungsfehler, und ein
  // ganz hinausgescrollter Reiter wäre unsichtbar, ohne dass etwas auf ihn
  // zeigt.
  const [canScrollRight, setCanScrollRight] = useState(false);

  const updateScrollHint = useCallback(() => {
    const tablist = tablistRef.current;
    if (!tablist) return;
    setCanScrollRight(
      tablist.scrollWidth - tablist.clientWidth - tablist.scrollLeft > 1,
    );
  }, []);

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
    updateScrollHint();
    const row = rowRef.current;
    const tablist = tablistRef.current;
    if (!row || !tablist) return;
    tablist.addEventListener("scroll", updateScrollHint, { passive: true });
    let observer: ResizeObserver | undefined;
    if (typeof ResizeObserver !== "undefined") {
      observer = new ResizeObserver(() => {
        measure();
        updateScrollHint();
      });
      observer.observe(row);
      observer.observe(tablist);
    }
    return () => {
      tablist.removeEventListener("scroll", updateScrollHint);
      observer?.disconnect();
    };
  }, [measure, updateScrollHint, items]);

  // Auf dem Telefon rueckt der aktive Reiter ins Sichtfenster des Bandes,
  // sonst steht die Seite auf einem Bereich, den man nicht sieht.
  useEffect(() => {
    const active = tablistRef.current?.querySelector<HTMLElement>(
      '[role="tab"][aria-selected="true"]',
    );
    if (typeof active?.scrollIntoView !== "function") return;
    active.scrollIntoView({ inline: "nearest", block: "nearest" });
  }, [value]);

  const visible = items.slice(0, visibleCount);
  const hidden = items.slice(visibleCount);
  const hiddenActive = hidden.find((item) => item.value === value);
  // Auf dem Desktop vertritt „Mehr“ den aktiven verborgenen Reiter. Auf dem
  // Telefon sind alle Reiter sichtbar, der aktive Reiter bleibt daher selbst
  // der eine Tabulator-Stopp.
  const rovingValue = hiddenActive && !isPhone ? "__mehr__" : value;

  const tabClass = (active: boolean, disabled?: boolean) =>
    cn(
      "flex shrink-0 items-center gap-1.5 border-b-[3px] pb-3 text-base whitespace-nowrap transition-colors",
      // Auf dem Telefon Pillen statt Grundlinie: eine graue Schrift mit
      // 3-px-Strich liest sich in einer 390-px-Spalte als Text, nicht als
      // Navigation. Die gefüllte Pille ist erkennbar ein Bedienelement.
      "max-sm:rounded-full max-sm:border-0 max-sm:px-3 max-sm:py-1.5 max-sm:pb-1.5 max-sm:text-sm",
      active
        ? "border-moto-green font-semibold text-gray-900 max-sm:bg-gray-900 max-sm:text-white"
        : "border-transparent font-medium text-gray-500 hover:border-gray-300 hover:text-gray-900 max-sm:bg-gray-100 max-sm:text-gray-700",
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

  const renderTab = (
    item: TenantPageTab,
    measuring = false,
    extraClassName?: string,
  ) => {
    const active = item.value === value;
    const className = cn(tabClass(active, item.disabled), extraClassName);
    if (item.href && !item.disabled) {
      return (
        <Link
          key={item.value}
          href={item.href}
          role={measuring ? undefined : "tab"}
          aria-selected={measuring ? undefined : active}
          tabIndex={measuring ? -1 : item.value === rovingValue ? 0 : -1}
          data-tab-value={item.value}
          className={className}
          onClick={(event: MouseEvent<HTMLAnchorElement>) => {
            // Mittelklick und Klick mit Zusatztaste öffnen ein zweites
            // Dokument. Der schlichte Linksklick meldet den Reiterwechsel
            // zusätzlich, lässt die Link-Navigation aber unangetastet.
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
        tabIndex={measuring ? -1 : item.value === rovingValue ? 0 : -1}
        data-tab-value={item.value}
        disabled={item.disabled}
        onClick={() => onChange(item.value)}
        className={className}
      >
        {renderInner(item)}
      </button>
    );
  };

  const moreTrigger = (measuring = false) => (
    <OverflowMenu
      key="__mehr__"
      ariaLabel={MORE_LABEL}
      triggerRole="tab"
      triggerAriaSelected={Boolean(hiddenActive)}
      triggerTabIndex={measuring ? -1 : rovingValue === "__mehr__" ? 0 : -1}
      triggerClassName={cn(tabClass(Boolean(hiddenActive)), "max-sm:hidden")}
      // leading-6 am Inhalt: ohne das drückt das Pfeil-Symbol die Zeilenhöhe
      // um ein Pixel und der Reiter steht einen Hauch tiefer als seine
      // Nachbarn.
      triggerContent={
        <span className="flex items-center gap-1 leading-6">
          {hiddenActive ? hiddenActive.label : MORE_LABEL}
          <ChevronDown className="size-3.5" aria-hidden />
        </span>
      }
      // Der Auslöser gehört zur Roving-Tabulator-Reihenfolge des Bandes.
      // `onKeyDown` sitzt am tablist-Container, damit alle Reiter dieselben
      // Pfeiltasten verwenden und der portalisierte Menüinhalt davon getrennt
      // bleibt.
      items={(measuring ? items : hidden).map((item) => ({
        label: item.label,
        href: item.href,
        disabled: item.disabled,
        selected: item.value === value,
        onClick: () => onChange(item.value),
      }))}
    />
  );

  const handleTabKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (
      event.key !== "ArrowLeft" &&
      event.key !== "ArrowRight" &&
      event.key !== "Home" &&
      event.key !== "End"
    ) {
      return;
    }

    const current = (event.target as HTMLElement).closest<HTMLElement>(
      '[role="tab"]',
    );
    if (!current || !tablistRef.current?.contains(current)) return;

    const tabs = Array.from(
      tablistRef.current.querySelectorAll<HTMLElement>('[role="tab"]'),
    ).filter((tab) => {
      if (tab.hasAttribute("disabled")) return false;
      // Desktop: die via `sm:hidden` gerenderten Überlauf-Reiter sind nicht
      // sichtbar. Telefon: „Mehr“ ist via `max-sm:hidden` ausgeblendet.
      return isPhone
        ? !tab.classList.contains("max-sm:hidden")
        : !tab.classList.contains("sm:hidden");
    });
    const index = tabs.indexOf(current);
    if (index === -1 || tabs.length === 0) return;

    event.preventDefault();
    const nextIndex =
      event.key === "Home"
        ? 0
        : event.key === "End"
          ? tabs.length - 1
          : (index + (event.key === "ArrowRight" ? 1 : -1) + tabs.length) %
            tabs.length;
    const next = tabs[nextIndex]!;
    next.focus();

    // Der Überlauf öffnet erst auf ausdrückliche Aktivierung. Jeder konkrete
    // Reiter wechselt beim Pfeil wie ein automatisch aktiviertes Tab-Panel.
    const nextValue = next.dataset.tabValue;
    if (nextValue) onChange(nextValue);
  };

  return (
    // Die Randaufhebung gehoert dem Reiterband: seine Grundlinie laeuft ueber
    // die volle Kartenbreite, auf jedem Geraet.
    <div className="-mx-5 mt-4 max-sm:-mx-4 max-sm:mt-3">
      {/* EIN Band auf allen Breiten. Unter sm scrollt es waagerecht und zeigt
          jeden Reiter; ab sm wird gemessen und der Ueberhang steht unter
          „Mehr". Vorher stand unter sm eine Auswahlliste -- sie zeigte nur den
          aktiven Wert („Status", „Betrieb") und las sich als Filter, nicht als
          Seitenbereich. Ein sichtbares Band mit derselben Grundlinie wie auf
          dem Desktop sagt auf den ersten Blick: hier wechselt man den
          Bereich, und so viele gibt es.

          Die Grundlinie gehört dem BAND, nicht dem einzelnen Reiter: eine
          Haarlinie über die volle Kartenbreite, der aktive Reiter färbt nur
          sein Stück davon ein. Dadurch sind alle Reiter gleich hohe Kästen
          und der Abstand hängt nicht mehr an einem Strich, den nur einer von
          ihnen trägt. Die Linie verbindet den Reiter zugleich sichtbar mit
          dem Inhalt darunter; eine einzeln getönte Pille sagt das nicht, sie
          liest sich als Filter. */}
      <div className="relative border-b border-gray-200 max-sm:border-0 sm:px-5">
        <div ref={rowRef} className="flex items-end gap-6 max-sm:gap-0">
          <div
            ref={tablistRef}
            role="tablist"
            aria-label={label}
            tabIndex={-1}
            onKeyDown={handleTabKeyDown}
            // Unter sm scrollt das Band bis an den Kartenrand; der Innenrand
            // der Karte sitzt als Polster im Scrollbereich, damit der erste
            // und der letzte Reiter nicht am Rahmen kleben.
            // Ab sm schneidet die Zeile ab (`clip`, kein Scrollkasten): die
            // Messung sorgt dafür, dass im Ruhezustand alles passt, aber
            // während die Seitenleiste auf- oder zuklappt, kann ein Bild lang
            // ein Reiter über den Kartenrand ragen, bis die Zeile neu gemessen
            // ist. Ohne Clip stand er in einem Screenshot halb außerhalb der
            // Karte.
            className="flex min-w-0 flex-1 items-end gap-6 max-sm:[scrollbar-width:none] max-sm:gap-2 max-sm:overflow-x-auto max-sm:px-4 sm:overflow-x-clip max-sm:[&::-webkit-scrollbar]:hidden"
          >
            {visible.map((item) => renderTab(item))}
            {/* Auf dem Telefon bleiben auch die gemessen verborgenen Reiter im
                Band stehen; der Sammelreiter verschwindet dort. */}
            {hidden.map((item) => renderTab(item, false, "sm:hidden"))}
            {/* Der Sammelreiter gehört semantisch zum Reiterband. Außerhalb
                davon würde er sich für Screenreader zwar als Reiter, aber
                nicht als Teil dieser Seitennavigation ankündigen. */}
            {hidden.length > 0 && moreTrigger()}
          </div>
        </div>
        {/* Scroll-Hinweis des mobilen Bandes: solange rechts Reiter liegen,
            blendet die Kante sie sichtbar aus — die Pille wirkt angeschnitten
            statt abgeschnitten, und beim Zu-Ende-Scrollen verschwindet der
            Verlauf. Ohne ihn wäre ein hinausgescrollter Reiter unsichtbar. */}
        {canScrollRight && (
          <div
            aria-hidden
            className="pointer-events-none absolute inset-y-0 right-0 w-10 bg-gradient-to-l from-white to-transparent sm:hidden"
          />
        )}

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
 * Titel und Beschreibung des Leerzustands als EIN Satzgefüge. Titel ohne
 * Schlusszeichen bekommen einen Punkt, damit „Keine Räume vorhanden Legen
 * Sie…" nicht entsteht.
 */
function joinEmptySentence(title: string, description?: string): string {
  const trimmed = title.trim();
  const punctuated = /[.!?…]$/.test(trimmed) ? trimmed : `${trimmed}.`;
  return description ? `${punctuated} ${description}` : punctuated;
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
    //
    // Zwei Formen: mit Aktion (nächster Schritt) oder Symbol (benannter
    // Zustand wie ForbiddenPage/FeatureOffPage) bekommt der Leerzustand die
    // volle Bühne (EmptyState). Eine schmucklose Feststellung ist EIN Satz
    // in der Karte, wie im Eltern-Portal — eine zentrierte Inszenierung
    // über einem Sachverhalt, an dem niemand etwas tun kann, ist eine
    // Aufgabe, die keine ist.
    return (
      <>
        {errorBanner}
        <SectionCard>
          {empty.action || empty.icon ? (
            <EmptyState
              title={empty.title}
              description={empty.description}
              icon={empty.icon}
              action={empty.action}
            />
          ) : (
            <p className="text-sm leading-6 text-gray-600">
              {joinEmptySentence(empty.title, empty.description)}
            </p>
          )}
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
